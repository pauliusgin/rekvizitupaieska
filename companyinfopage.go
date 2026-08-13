package rekvizitupaieska

import (
	"io"
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

const (
	companyNameSelector    = "div.main-info h1.title"
	companyLogoSelector    = `img[itemprop="logo"]`
	companyAddressSelector = "#copy_address"
	companyIDSelector      = "#ccode"
	companyVatIDSelector   = "#pcode"
)

func parseCompanyInfoPage(baseURL *url.URL, page io.Reader) (CompanyInfo, bool, error) {
	document, err := goquery.NewDocumentFromReader(page)
	if err != nil {
		return CompanyInfo{}, false, err
	}

	companyInfo, found := extractCompanyInfo(baseURL, document)

	return companyInfo, found, nil
}

func extractCompanyInfo(baseURL *url.URL, document *goquery.Document) (CompanyInfo, bool) {
	companyInfo := CompanyInfo{
		CompanyName:    scrapeFieldText(document, companyNameSelector),
		CompanyLogo:    scrapeCompanyLogoURL(baseURL, document),
		CompanyAddress: scrapeFieldText(document, companyAddressSelector),
		CompanyID:      scrapeFieldText(document, companyIDSelector),
		CompanyVatID:   scrapeFieldText(document, companyVatIDSelector),
	}

	if companyInfo.CompanyName == "" || companyInfo.CompanyID == "" {
		return CompanyInfo{}, false
	}

	return companyInfo, true
}

func scrapeFieldText(document *goquery.Document, selector string) string {
	return normalizeScrapedText(document.Find(selector).First().Text())
}

func scrapeCompanyLogoURL(baseURL *url.URL, document *goquery.Document) string {
	logoSource, exists := document.Find(companyLogoSelector).First().Attr("src")
	if !exists {
		return ""
	}

	resolvedLogoURL := resolveAgainstBaseURL(baseURL, logoSource)
	if resolvedLogoURL == nil {
		return ""
	}

	return resolvedLogoURL.String()
}
