import sys
import codecs
import random
import json

def main():
	random.seed(1337)

	stopword_file = sys.argv[1]
	outfile = sys.argv[2]

	words = set()
	with codecs.open(stopword_file, 'r', 'utf8') as fh:
		for line in fh:
			words.add(line.strip())

	all_sets = list()
	for i in xrange(0, 500):
		rand_words = random.sample(words, 350)
		rand_ints = [random.randint(0,50) for j in rand_words]

		rand_set = dict(zip(rand_words, rand_ints))
		all_sets.append(rand_set)

	with codecs.open(outfile, 'w', 'utf8') as fh:
		fh.write("language=Spanish&tfs=")
		fh.write(json.dumps(all_sets, ensure_ascii=False))
		fh.write("\n")



if __name__ == "__main__":
	main()