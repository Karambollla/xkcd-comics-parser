package words

import (
	"strings"
	"unicode"

	"github.com/kljensen/snowball/english"
)

func Norm(phrase string) []string {
	if len(phrase) == 0 {
		return []string{}
	}
	var words []string
	fieldFunc := func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}
	for _, field := range strings.FieldsFunc(phrase, fieldFunc) {
		word := strings.ToLower(field)
		words = append(words, word)
	}
	res := make([]string, 0)
	dupMap := make(map[string]struct{})
	for _, word := range words {
		if english.IsStopWord(word) {
			continue
		}
		wrd := english.Stem(word, true)
		if _, exists := dupMap[wrd]; !exists {
			dupMap[wrd] = struct{}{}
			res = append(res, wrd)
		}
	}
	return res
}
