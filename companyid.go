package rekvizitupaieska

import "strings"

const requiredCompanyIDLength = 9

func validateCompanyID(companyID string) (string, error) {
	trimmedCompanyID := strings.TrimSpace(companyID)

	validLength := len(trimmedCompanyID) == requiredCompanyIDLength
	onlyDigits := containsOnlyASCIIDigits(trimmedCompanyID)

	if !validLength || !onlyDigits {
		return "", ErrInvalidCompanyID{CompanyID: companyID}
	}

	return trimmedCompanyID, nil
}

func containsOnlyASCIIDigits(text string) bool {
	for index := range len(text) {
		if text[index] < '0' || text[index] > '9' {
			return false
		}
	}
	return true
}
