package rekvizitupaieska

import (
	"io"
	"net/url"
	"strconv"

	"github.com/PuerkitoBio/goquery"
)

// searchResultBlockSelector matches one block per company on a lookup results page, which is
// what both a search by name and a search by code come back as.
const searchResultBlockSelector = "div.block-list div.list-item div.company"

// a result block's class is exactly "company", a similar-company block's is "company
// highlighted". Rejecting the class is what stops a detail page from parsing as a page of results.
const similarCompanyClass = ".highlighted"

const searchResultLinkSelector = "a.company-title"

const searchResultCountSelector = "div.breadcrumb strong"

func parseSearchResultURLs(baseURL *url.URL, page io.Reader) ([]string, error) {
	document, err := goquery.NewDocumentFromReader(page)
	if err != nil {
		return nil, err
	}

	return extractSearchResultURLs(baseURL, document), nil
}

func extractSearchResultURLs(baseURL *url.URL, document *goquery.Document) []string {
	resultBlocks := document.Find(searchResultBlockSelector).Not(similarCompanyClass)

	searchResultURLs := []string{}
	resultBlocks.Find(searchResultLinkSelector).Each(func(_ int, resultLink *goquery.Selection) {
		href, exists := resultLink.Attr("href")
		if !exists {
			return
		}
		if resolvedURL := resolveAgainstBaseURL(baseURL, href); resolvedURL != nil {
			searchResultURLs = append(searchResultURLs, resolvedURL.String())
		}
	})

	return searchResultURLs
}

func parseSearchResultCount(document *goquery.Document) (int, bool) {
	countText := normalizeScrapedText(document.Find(searchResultCountSelector).First().Text())
	if countText == "" {
		return 0, false
	}

	statedCount, err := strconv.Atoi(countText)
	if err != nil {
		return 0, false
	}

	return statedCount, true
}
