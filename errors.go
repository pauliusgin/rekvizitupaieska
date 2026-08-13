package rekvizitupaieska

import "fmt"

const companyIDExpectationText = "expected exactly 9 digits"
const companyNameExpectationText = "expected at least 3 consecutive letters"

type ErrInvalidCompanyID struct {
	CompanyID string
}

func (e ErrInvalidCompanyID) Error() string {
	return fmt.Sprintf("invalid company ID %q: %s", e.CompanyID, companyIDExpectationText)
}

type ErrInvalidCompanyName struct {
	CompanyName string
}

func (e ErrInvalidCompanyName) Error() string {
	return fmt.Sprintf("invalid company name %q: %s", e.CompanyName, companyNameExpectationText)
}

type ErrTooManyMatches struct {
	CompanyName string
	MatchCount  int
	MatchURLs   []string
}

func (e ErrTooManyMatches) Error() string {
	return fmt.Sprintf("company name %q matched %d companies: narrow the search term",
		e.CompanyName, e.MatchCount)
}

type ErrCompanyNotFound struct {
	Query string
}

func (e ErrCompanyNotFound) Error() string {
	return fmt.Sprintf("no company found for %q", e.Query)
}

type ErrRegistryRequestFailed struct {
	URL        string
	StatusCode int
}

func (e ErrRegistryRequestFailed) Error() string {
	return fmt.Sprintf("registry request for %q returned status %d", e.URL, e.StatusCode)
}
