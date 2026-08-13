package rekvizitupaieska

import (
	"context"
	"fmt"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

const userAgentHeader = "User-Agent"

const (
	lowestSuccessStatus  = 200
	highestSuccessStatus = 299
)

func (c *Client) fetchRegistryDocument(ctx context.Context, pageURL string) (*goquery.Document, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build registry request for %q: %w", pageURL, err)
	}
	request.Header.Set(userAgentHeader, c.userAgent)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("registry request for %q: %w", pageURL, err)
	}
	defer response.Body.Close()

	if !isSuccessStatus(response.StatusCode) {
		return nil, ErrRegistryRequestFailed{URL: pageURL, StatusCode: response.StatusCode}
	}

	return goquery.NewDocumentFromReader(response.Body)
}

func isSuccessStatus(statusCode int) bool {
	return statusCode >= lowestSuccessStatus && statusCode <= highestSuccessStatus
}
