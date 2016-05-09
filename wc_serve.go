package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Used to hold discovered valid language dirs
type Folder struct {
	basename string
	fullpath string
}

// Used to hold language information needed for tfidf
type LangData struct {
	docfreqs  map[string]float64
	stopwords map[string]bool
	numdocs   int
}

// search for one-deep sub directories (that start with a capital letter) of the provided search directory
func find_valid_directories(searchdir string) []Folder {
	var valid_dirs []Folder

	// Search for directories starting with a capital letter
	re := regexp.MustCompile("^[[:upper:]][a-z]+$")

	abs_path, _ := filepath.Abs(searchdir)

	files, _ := ioutil.ReadDir(abs_path + "/")
	for _, f := range files {
		if f.IsDir() && re.MatchString(f.Name()) {
			valid_dirs = append(valid_dirs, Folder{f.Name(), abs_path + "/" + f.Name()})
		}
	}
	return valid_dirs
}

// for a specific language directory, read pre-determined convention data
func read_lang_data(langdir string, numstopwords int) LangData {
	docfreqsF := "docfreqs.txt"   // file contains "word float"
	stopwordsF := "stopwords.txt" // file contains "word"
	numwordsF := "numdocs.txt"    // file contains "int"

	docfreqs := make(map[string]float64)
	if content, err := ioutil.ReadFile(langdir + "/" + docfreqsF); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			elements := strings.Fields(line)
			if len(elements) == 2 {
				word := strings.ToLower(elements[0])
				freq, _ := strconv.ParseFloat(elements[1], 64)
				docfreqs[word] = docfreqs[word] + float64(freq)
			}
		}
	}

	stopwords := make(map[string]bool)
	if content, err := ioutil.ReadFile(langdir + "/" + stopwordsF); err == nil {
		lines := strings.Split(string(content), "\n")
		count := 0
		for _, line := range lines {
			elements := strings.Fields(line)
			if len(elements) == 1 {
				word := strings.ToLower(elements[0])
				stopwords[word] = true

				// read only the first N stopwords
				if count >= numstopwords {
					break
				}
				count++
			}
		}
	}

	numdocs := 1
	if content, err := ioutil.ReadFile(langdir + "/" + numwordsF); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			elements := strings.Fields(line)
			if len(elements) == 1 {
				numdocs, _ = strconv.Atoi(elements[0])
				break // all useful information should be only on the first line
			}
		}
	}

	return LangData{docfreqs, stopwords, numdocs}
}

// core wordcloud computations; for each word: filter out stopwords, normalize tfs and dfs, compute tfidf
func compute_wordcloud(raw_tfs map[string]int, langdata LangData) map[string]map[string]float64 {
	wordcloud := make(map[string]map[string]float64)

	// compute set difference of input raw tf words and the stopwords
	stopwords := langdata.stopwords
	tfs := map[string]int{}
	for word, _ := range raw_tfs {
		tfs[strings.ToLower(word)] = raw_tfs[word]
	}
	for stopword, _ := range stopwords {
		delete(tfs, stopword)
	}

	// for each word, if its tf is at least 1, compute tfidf
	numdocs := langdata.numdocs
	docfreqs := langdata.docfreqs
	for word, _ := range tfs {
		tf := float64(tfs[word])
		if tf >= 1 {
			norm_tf := 1 + math.Log(float64(tf))
			df := docfreqs[word]
			norm_df := math.Log(float64(numdocs) / (1 + df))
			tfidf := norm_tf * norm_df

			wordcloud[word] = map[string]float64{"tf": tf, "df": df, "tfidf": tfidf}
		}
	}
	return wordcloud
}

// curried web handler to provide a list of available languages
func RootHandler(lang_lookup map[string]LangData) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		langs := make([]string, 0, len(lang_lookup))
		for k := range lang_lookup {
			langs = append(langs, k)
		}

		// Marshal provided interface into JSON structure
		langsj, _ := json.Marshal(langs)

		// Write content-type, statuscode, payload
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, "%s", langsj)
	}
}

// curried web handler to provide individual wordcloud given a provided language and set of words with frequencies
func WordcloudHandler(lang_lookup map[string]LangData) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// 75MB max accepted payload
		max_bytes := 75000000
		r.Body = http.MaxBytesReader(w, r.Body, int64(max_bytes))
		err := r.ParseForm()
		if err != nil {
			w.WriteHeader(413)
			fmt.Fprintf(w, "Request entity too large: max accepted bytes is set to %d.\n", max_bytes)
			return
		}

		tfs := make(map[string]int)

		language := r.FormValue("language")
		json.NewDecoder(strings.NewReader(r.FormValue("tfs"))).Decode(&tfs)

		var wordcloud map[string]map[string]float64
		if lang_data, ok := lang_lookup[language]; ok {
			wordcloud = compute_wordcloud(tfs, lang_data)
		} else {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Language %s is not available for processing.\n", language)
			return
		}

		// Marshal provided interface into JSON structure
		wcj, _ := json.Marshal(wordcloud)

		// Write content-type, statuscode, payload
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, "%s", wcj)
	}
}

// curried bulk web handler to provide a list of wordclouds given a provided language and a list of sets of words with frequencies
func WordcloudBulkHandler(lang_lookup map[string]LangData) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// 75MB max accepted payload
		max_bytes := 75000000
		r.Body = http.MaxBytesReader(w, r.Body, int64(max_bytes))
		err := r.ParseForm()
		if err != nil {
			w.WriteHeader(413)
			fmt.Fprintf(w, "Request entity too large: max accepted bytes is set to %d.\n", max_bytes)
			return
		}
		
		manytfs := make([]map[string]int, 0, 500)

		language := r.FormValue("language")
		json.NewDecoder(strings.NewReader(r.FormValue("tfs"))).Decode(&manytfs)

		wordclouds := make([]map[string]map[string]float64, 0, len(manytfs))
		if lang_data, ok := lang_lookup[language]; ok {
			for _, tfs := range manytfs {
				wordcloud := compute_wordcloud(tfs, lang_data)
				wordclouds = append(wordclouds, wordcloud)
			}
		} else {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Language %s is not available for processing.\n", language)
			return
		}

		// Marshal provided interface into JSON structure
		wcj, _ := json.Marshal(wordclouds)

		// Write content-type, statuscode, payload
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, "%s", wcj)
	}
}

func main() {
	// Itemize command line args and output runtime to stdout
	datadirPtr := flag.String("datadir", "", "Directory of language data")
	numstopwordsPtr := flag.Int("numstopwords", 300, "The number of stopwords")
	portPtr := flag.Int("port", 8080, "Port to host http server")
	secportPtr := flag.Int("secport", 8081, "Secure port to host https server")
	certPtr := flag.String("cert", "", "Path to cert for http2 & ssl")
	keyPtr := flag.String("key", "", "Path to key for http2 & ssl")
	flag.Parse()

	fmt.Println("datadir:", *datadirPtr)
	fmt.Println("numstopwords:", *numstopwordsPtr)
	fmt.Println("port:", *portPtr)
	fmt.Println("secport:", *secportPtr)
	fmt.Println("cert:", *certPtr)
	fmt.Println("key:", *keyPtr)
	fmt.Println("trailing args:", flag.Args())

	// Allocate principle in-memory data structure
	lang_lookup := make(map[string]LangData)

	// Find valid directories and load in-memory data structure
	valid_dirs := find_valid_directories(*datadirPtr)
	for _, vdir := range valid_dirs {
		fmt.Println("Language: ", vdir.basename)
		fmt.Println("Directory: ", vdir.fullpath)
		temp_data := read_lang_data(vdir.fullpath, *numstopwordsPtr)
		lang_lookup[vdir.basename] = temp_data
	}

	// Define router endpoints
	router := httprouter.New()
	router.GET("/", RootHandler(lang_lookup))
	router.POST("/wordcloud", WordcloudHandler(lang_lookup))
	router.POST("/wordcloud/bulk", WordcloudBulkHandler(lang_lookup))

	// Check to see if HTTPS is possible to run
	if (*certPtr != "") && (*keyPtr != "") && (*portPtr != *secportPtr) {
		srvsec := &http.Server{
			Addr:    ":" + strconv.Itoa(*secportPtr), // Traditionally ":443"
			Handler: router,                          // Handler
		}
		fmt.Println("Starting HTTPS server...")
		go srvsec.ListenAndServeTLS(*certPtr, *keyPtr)
	} else {
		fmt.Println("Not starting HTTPS server. Lacking either cert, key or unique port separate from http.\n")
	}

	// Always run HTTP server
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(*portPtr), // Traditionally ":80"
		Handler: router,                       // Handler
	}
	fmt.Println("Starting HTTP server...")
	log.Fatal(srv.ListenAndServe())
}
