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
	searchResultURLs := []string{}
	if isCompanyInfoPage(document) {
		return searchResultURLs
	}

	resultBlocks := document.Find(searchResultBlockSelector)
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

// isCompanyInfoPage tells a detail page apart from a results page. The detail page's
// "Taip pat rekomenduojame" sidebar reuses the result block markup, and the block class cannot
// tell them apart: "highlighted" marks a promoted listing, which shows up both in that sidebar
// and as a genuine search result.
func isCompanyInfoPage(document *goquery.Document) bool {
	return document.Find(companyIDSelector).Length() > 0
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
