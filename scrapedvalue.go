package rekvizitupaieska

import (
	"net/url"
	"strings"
)

func normalizeScrapedText(rawText string) string {
	return strings.Join(strings.Fields(rawText), " ")
}

func resolveAgainstBaseURL(baseURL *url.URL, rawURL string) *url.URL {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return nil
	}

	referenceURL, err := url.Parse(trimmedURL)
	if err != nil {
		return nil
	}

	return baseURL.ResolveReference(referenceURL)
}
