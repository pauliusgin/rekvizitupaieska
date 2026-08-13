//go:build live

// This file is built only under the "live" tag, so the default `go test ./...` never reaches
// the network:
//
//	go test -tags=live -run TestLiveRegistry ./...
//
// What it guards is drift between the saved pages under testdata/ and the registry as it is
// served today. The fixtures pin selectors, and a redesign that renames a class or drops an id
// turns every parser in this package into one that quietly returns empty fields — the offline
// suite would stay green because it is reading the old page. So the assertions here are about
// the shape of a live page, not its contents: which selectors still resolve, and whether a
// lookup still comes back as a search rather than as the registry's promo catalogue. Company
// data itself churns, and pinning it would make this test fail for the wrong reason.

package rekvizitupaieska

import (
	"context"
	"strings"
	"testing"
	"time"
)

// liveLookupTimeout covers the whole test rather than one request: the ID lookup alone makes
// two, and a registry slow enough to need longer than this is a result worth seeing.
const liveLookupTimeout = 30 * time.Second

// liveSearchTerm is the term testdata/company_lookup_response.html was captured for, which is
// what makes the saved page and the live one comparable.
const liveSearchTerm = "citma"

// liveImplausibleMatchCount is far above what any one term matches and far below the registry's
// company total, so a count past it means the page counted the whole registry rather than a
// search.
const liveImplausibleMatchCount = 1000

func TestLiveRegistryStillMatchesFixtures(t *testing.T) {
	client := NewClient(ClientParams{})

	t.Run("search results page", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), liveLookupTimeout)
		defer cancel()

		lookupURL, err := buildCompanyNameLookupURL(client.baseURL, liveSearchTerm)
		if err != nil {
			t.Fatalf("buildCompanyNameLookupURL(%q) returned error %v", liveSearchTerm, err)
		}

		document, err := client.fetchRegistryDocument(ctx, lookupURL)
		if err != nil {
			t.Fatalf("fetchRegistryDocument(%q) returned error %v", lookupURL, err)
		}

		// The stated count is checked through parseSearchResultCount rather than through
		// fetchSearchResults, because fetchSearchResults falls back to the number of links it
		// found. That fallback would hide the loss of div.breadcrumb strong, which is exactly
		// what this test exists to catch.
		statedCount, stated := parseSearchResultCount(document)
		if !stated {
			t.Errorf("live %q page states no match count, want the count from %q", liveSearchTerm, searchResultCountSelector)
		}
		// A count in the hundreds of thousands is the registry's whole company total, which is
		// what the catalogue page states when a lookup did not search at all.
		if stated && (statedCount == 0 || statedCount > liveImplausibleMatchCount) {
			t.Errorf("live %q page states %d matches, want a plausible search result count", liveSearchTerm, statedCount)
		}

		liveURLs := extractSearchResultURLs(client.baseURL, document)
		if len(liveURLs) == 0 {
			t.Fatalf("live %q page yielded no result URLs, want at least one", liveSearchTerm)
		}
		for _, liveURL := range liveURLs {
			if !strings.HasPrefix(liveURL, registryBaseURL+companyDetailPathPrefix) {
				t.Errorf("live result URL %q is not a company detail page", liveURL)
			}
		}

		// A lookup that failed to trigger a search comes back as the registry's promo
		// catalogue, and that page is not obviously broken: it carries a breadcrumb count of
		// its own — the registry's entire company total, 235265 the day this was written — and
		// a full page of links. What it does not carry is any of the companies the search was
		// for, so overlap with the fixture is what tells the two pages apart. Overlap and not
		// equality, because companies are added to and removed from the registry.
		if countOverlappingURLs(liveURLs, goldenSearchResultURLs) == 0 {
			t.Errorf("live %q results %q share no company with the fixture's %q", liveSearchTerm, liveURLs, goldenSearchResultURLs)
		}
	})

	t.Run("company detail page", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), liveLookupTimeout)
		defer cancel()

		liveCompanyInfo, err := client.GetCompanyInfoByID(ctx, goldenCompanyInfo.CompanyID)
		if err != nil {
			t.Fatalf("GetCompanyInfoByID(%q) returned error %v", goldenCompanyInfo.CompanyID, err)
		}

		// The name and the code are the pair extractCompanyInfo treats as proof that a page
		// describes a company, and a registration code identifies one company forever, so
		// these two are pinned to the fixture. The rest are only checked for being there: an
		// address changes when a company moves, and a logo when it rebrands, but an empty one
		// means the selector stopped resolving.
		if liveCompanyInfo.CompanyID != goldenCompanyInfo.CompanyID {
			t.Errorf("live CompanyID = %q, want %q", liveCompanyInfo.CompanyID, goldenCompanyInfo.CompanyID)
		}

		// The name is compared on its letters alone. The registry edits the punctuation around
		// a name without the company changing at all — the fixture was saved when this one was
		// recorded as `UAB "ORLEN Apsauga "`, trailing space and all, and it since lost that
		// space. Pinning the raw string would report a tidied record as a parser failure.
		liveName := collapseCompanyNameFormatting(liveCompanyInfo.CompanyName)
		fixtureName := collapseCompanyNameFormatting(goldenCompanyInfo.CompanyName)
		if liveName != fixtureName {
			t.Errorf("live CompanyName = %q, want %q (compared as %q vs %q)",
				liveCompanyInfo.CompanyName, goldenCompanyInfo.CompanyName, liveName, fixtureName)
		}

		optionalFields := map[string]string{
			"CompanyAddress": liveCompanyInfo.CompanyAddress,
			"CompanyVatID":   liveCompanyInfo.CompanyVatID,
			"CompanyLogo":    liveCompanyInfo.CompanyLogo,
		}
		for fieldName, fieldValue := range optionalFields {
			if fieldValue == "" {
				t.Errorf("live %s is empty, want a value — the fixture company has one", fieldName)
			}
		}
	})
}

// companyDetailPathPrefix is the path every company detail page lives under. It is spelled out
// here rather than in the package because nothing but this test needs to recognize one.
const companyDetailPathPrefix = "/imone/"

// collapseCompanyNameFormatting reduces a name to the letters that identify the company:
// quotes dropped and every run of whitespace collapsed to one space. What is left still tells
// two companies apart, while surviving the registry's own reformatting of one.
func collapseCompanyNameFormatting(companyName string) string {
	withoutQuotes := strings.ReplaceAll(companyName, `"`, "")

	return strings.Join(strings.Fields(withoutQuotes), " ")
}

func countOverlappingURLs(liveURLs, fixtureURLs []string) int {
	fixtureURLSet := make(map[string]struct{}, len(fixtureURLs))
	for _, fixtureURL := range fixtureURLs {
		fixtureURLSet[fixtureURL] = struct{}{}
	}

	overlapping := 0
	for _, liveURL := range liveURLs {
		if _, found := fixtureURLSet[liveURL]; found {
			overlapping++
		}
	}

	return overlapping
}
