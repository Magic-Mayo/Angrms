package slices

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
)

func stringContains(letters string, slices [][]string) []string {
	var found []string

	for _, wordArray := range slices {
		for _, word := range wordArray {
			everyLetter := true
			for _, letter := range letters {
				if strings.HasPrefix(string(word), string(letter)) {
					continue
				}

				if !strings.Contains(string(word), string(letter)) {
					everyLetter = false
					break
				}
			}

			if everyLetter {
				rgx, err := regexp.Compile("^[" + letters + "]+$")

				if err != nil {

				}

				if rgx.MatchString(string(word)) {
					found = append(found, string(word))
				}
			}
		}
	}

	return found
}

func FindWordsWithLetters(letters string) []string {
	content, err := ioutil.ReadFile("words.json")

	if err != nil {
		log.Fatal("Cannot open file", err)
	}

	var payload map[string][]string
	err = json.Unmarshal(content, &payload)

	if err != nil {
		log.Fatal("Cannot unmarshal JSON", err)
	}

	var words [][]string
	for _, letter := range letters {
		words = append(words, payload[string(letter)])
	}

	return stringContains(letters, words)
}
