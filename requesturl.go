package rekvizitupaieska

import (
	"net/url"
)

const registryLookupPath = "/imones/1/"

const (
	companyIDParameter   = "company_code"
	companyNameParameter = "name"
)

// The three parameters every lookup carries besides the code or the search term. Their values
// are the ones the registry's own search form sends: it POSTs to /imones/1/ and the redirect it
// answers with appends resetFilter=0, order=1 and redirected=1.
const (
	resetFilterParameter = "resetFilter"
	resetFilterKeepAll   = "0"
	orderParameter       = "order"
	orderDefault         = "1"
	redirectedParameter  = "redirected"
	redirectedFromForm   = "1"
)

type lookupQueryParameter struct {
	Name  string
	Value string
}

func buildCompanyIDLookupURL(baseURL *url.URL, companyID string) (string, error) {
	validatedCompanyID, err := validateCompanyID(companyID)
	if err != nil {
		return "", err
	}

	lookupParameter := lookupQueryParameter{
		Name:  companyIDParameter,
		Value: validatedCompanyID,
	}

	return buildLookupURL(baseURL, lookupParameter), nil
}

func buildCompanyNameLookupURL(baseURL *url.URL, companyName string) (string, error) {
	normalizedCompanyName, err := normalizeCompanyName(companyName)
	if err != nil {
		return "", err
	}

	lookupParameter := lookupQueryParameter{
		Name:  companyNameParameter,
		Value: normalizedCompanyName,
	}

	return buildLookupURL(baseURL, lookupParameter), nil
}

func buildLookupURL(baseURL *url.URL, queryParameter lookupQueryParameter) string {
	queryParameters := buildFreshRegistryCallParams()
	queryParameters.Set(queryParameter.Name, queryParameter.Value)

	lookupURL := baseURL.JoinPath(registryLookupPath)
	lookupURL.RawQuery = queryParameters.Encode()

	return lookupURL.String()
}

// buildFreshRegistryCallParams returns the parameters that make /imones/1/ run a new search
// rather than re-render the one it already holds.
func buildFreshRegistryCallParams() url.Values {
	return url.Values{
		resetFilterParameter: {resetFilterKeepAll},
		orderParameter:       {orderDefault},
		redirectedParameter:  {redirectedFromForm},
	}
}
