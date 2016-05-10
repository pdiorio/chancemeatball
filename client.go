package main

import (
	"net/http"
	"io/ioutil"
	"fmt"
	"os"
	"crypto/tls"
	"net/url"
	//"time"
	//"net"
	"golang.org/x/net/http2"
)

func main() {

    tr := &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
        //Proxy: http.ProxyFromEnvironment,
        //Dial: (&net.Dialer{
        //        Timeout:   30 * time.Second,
        //        KeepAlive: 30 * time.Second,
        //}).Dial,
        //TLSHandshakeTimeout:   10 * time.Second,
        //ExpectContinueTimeout: 1 * time.Second,
    }
    //tr.ExpectContinueTimeout = 0
    http2.ConfigureTransport(tr)
    client := &http.Client{Transport: tr}
	resp, err := client.Get("https://google.com/")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	//body, _ := ioutil.ReadAll(resp.Body)
	//fmt.Println(string(body))
	fmt.Println(string(resp.Proto))
	//fmt.Println(tr)

	resp2, err2 := client.PostForm("https://127.0.0.1:8081/wordcloud", url.Values{"language": {"Spanish"}, "tfs": {"{\"biblioteca\": 1}"}})
	if err2 != nil {
		fmt.Println(err2)
		os.Exit(1)
	}
	defer resp2.Body.Close()
	body2, _ := ioutil.ReadAll(resp2.Body)
	fmt.Println(string(body2))
	fmt.Println(string(resp2.Proto))
	//fmt.Println(tr)
}