package rekvizitupaieska

import (
	"context"
	"sync"
)

type searchResults struct {
	CompanyURLs []string
	MatchCount  int
}

func (c *Client) GetCompanyInfoByID(ctx context.Context, companyID string) (CompanyInfo, error) {
	lookupURL, err := buildCompanyIDLookupURL(c.baseURL, companyID)
	if err != nil {
		return CompanyInfo{}, err
	}

	results, err := c.fetchSearchResults(ctx, lookupURL)
	if err != nil {
		return CompanyInfo{}, err
	}
	if len(results.CompanyURLs) == 0 {
		return CompanyInfo{}, ErrCompanyNotFound{Query: companyID}
	}

	return c.fetchCompanyInfo(ctx, companyInfoRequest{
		Query:      companyID,
		CompanyURL: results.CompanyURLs[0],
	})
}

func (c *Client) GetCompanyInfoByName(ctx context.Context, companyName string) ([]CompanyInfo, error) {
	lookupURL, err := buildCompanyNameLookupURL(c.baseURL, companyName)
	if err != nil {
		return nil, err
	}

	results, err := c.fetchSearchResults(ctx, lookupURL)
	if err != nil {
		return nil, err
	}
	if len(results.CompanyURLs) == 0 {
		return nil, ErrCompanyNotFound{Query: companyName}
	}
	if results.MatchCount > c.maxMatches {
		return nil, ErrTooManyMatches{
			CompanyName: companyName,
			MatchCount:  results.MatchCount,
			MatchURLs:   results.CompanyURLs,
		}
	}

	return c.fetchEveryCompanyInfo(ctx, companyName, results.CompanyURLs)
}

// fetchSearchResults GETs a lookup page and reads both the result URLs and the match count off
// the one parsed document.
func (c *Client) fetchSearchResults(ctx context.Context, lookupURL string) (searchResults, error) {
	document, err := c.fetchRegistryDocument(ctx, lookupURL)
	if err != nil {
		return searchResults{}, err
	}

	companyURLs := extractSearchResultURLs(c.baseURL, document)

	matchCount, stated := parseSearchResultCount(document)
	if !stated {
		matchCount = len(companyURLs)
	}

	return searchResults{CompanyURLs: companyURLs, MatchCount: matchCount}, nil
}

type companyInfoRequest struct {
	Query      string
	CompanyURL string
}

func (c *Client) fetchCompanyInfo(ctx context.Context, request companyInfoRequest) (CompanyInfo, error) {
	document, err := c.fetchRegistryDocument(ctx, request.CompanyURL)
	if err != nil {
		return CompanyInfo{}, err
	}

	companyInfo, found := extractCompanyInfo(c.baseURL, document)
	if !found {
		return CompanyInfo{}, ErrCompanyNotFound{Query: request.Query}
	}

	return companyInfo, nil
}

func (c *Client) fetchEveryCompanyInfo(ctx context.Context, query string, companyURLs []string) ([]CompanyInfo, error) {
	companyInfos := make([]CompanyInfo, len(companyURLs))
	fetchErrors := make([]error, len(companyURLs))

	var fetchGroup sync.WaitGroup
	for index, companyURL := range companyURLs {
		fetchGroup.Go(func() {
			companyInfos[index], fetchErrors[index] = c.fetchCompanyInfo(ctx, companyInfoRequest{
				Query:      query,
				CompanyURL: companyURL,
			})
		})
	}
	fetchGroup.Wait()

	for _, fetchError := range fetchErrors {
		if fetchError != nil {
			return nil, fetchError
		}
	}

	return companyInfos, nil
}
