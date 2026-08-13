package rekvizitupaieska

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const minimumLetterRunLength = 3

func normalizeCompanyName(companyName string) (string, error) {
	composedCompanyName := norm.NFC.String(companyName)
	lowercasedCompanyName := strings.ToLower(composedCompanyName)

	letterRuns := extractLetterRuns(lowercasedCompanyName)
	if longestLetterRunLength(letterRuns) < minimumLetterRunLength {
		return "", ErrInvalidCompanyName{CompanyName: companyName}
	}

	return strings.Join(letterRuns, " "), nil
}

var casedLetterCategories = []*unicode.RangeTable{unicode.Lu, unicode.Ll}

func extractLetterRuns(text string) []string {
	var letterRuns []string
	var currentRun strings.Builder

	for _, character := range text {
		if unicode.In(character, casedLetterCategories...) {
			currentRun.WriteRune(character)
			continue
		}
		if currentRun.Len() > 0 {
			letterRuns = append(letterRuns, currentRun.String())
			currentRun.Reset()
		}
	}
	if currentRun.Len() > 0 {
		letterRuns = append(letterRuns, currentRun.String())
	}

	return letterRuns
}

// longestLetterRunLength returns the length in letters (runes) of the longest run, or zero when
// there are none.
func longestLetterRunLength(letterRuns []string) int {
	longestLength := 0

	for _, letterRun := range letterRuns {
		if letterCount := utf8.RuneCountInString(letterRun); letterCount > longestLength {
			longestLength = letterCount
		}
	}

	return longestLength
}
