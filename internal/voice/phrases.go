package voice

import (
	"regexp"
	"strings"
)

var nonLetters = regexp.MustCompile(`[^a-z0-9 ]+`)

func normalize(text string) string {
	text = nonLetters.ReplaceAllString(strings.ToLower(text), " ")
	return strings.Join(strings.Fields(text), " ")
}

func MatchPhrase(text string, phrases []string) (bool, string) {
	heard := normalize(text)
	for _, p := range phrases {
		phrase := normalize(p)
		if phrase == "" {
			continue
		}
		if i := strings.Index(heard, phrase); i >= 0 {
			return true, strings.TrimSpace(heard[i+len(phrase):])
		}
	}
	return false, ""
}
