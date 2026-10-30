package rekvizitupaieska

import (
	"errors"
	"os"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/PuerkitoBio/goquery"
)

// searchResultsFixturePath is a real results page saved from the registry. It is the search
// for "citma" and not the "orl apt" search the written spec uses in its URL examples — the
// companies below are the ones this page actually lists.
const searchResultsFixturePath = "testdata/company_lookup_response.html"

var goldenSearchResultURLs = []string{
	"https://rekvizitai.vz.lt/imone/citma/",
	"https://rekvizitai.vz.lt/imone/panevezio_citma/",
	"https://rekvizitai.vz.lt/imone/vilniaus_citma/",
	"https://rekvizitai.vz.lt/imone/citma_ventus/",
}

const goldenSearchResultCount = 4

// highlightedSearchResultsFixturePath is the registry's results page for "diginet". Diginet LTU
// is a promoted listing, so its block carries "company highlighted" — a genuine result that a
// filter on the block class would drop.
const highlightedSearchResultsFixturePath = "testdata/company_lookup_highlighted_response.html"

var goldenHighlightedSearchResultURLs = []string{
	"https://rekvizitai.vz.lt/imone/plius/",
	"https://rekvizitai.vz.lt/imone/diginetas/",
}

// similarCompanyURLs are the companies the *detail* page lists in its "panašios įmonės"
// sidebar. That sidebar is built from the same markup as a search result, so these are what
// a results parser returns when it is pointed at a detail page and its selector is not
// scoped tightly enough.
var similarCompanyURLs = []string{
	"https://rekvizitai.vz.lt/imone/argus_saugos_tarnyba/",
	"https://rekvizitai.vz.lt/imone/es_security/",
	"https://rekvizitai.vz.lt/imone/ekskomisaru_biuras/",
	"https://rekvizitai.vz.lt/imone/baltijos_saugos_grupe/",
	"https://rekvizitai.vz.lt/imone/rimano_biuras/",
}

func TestParseSearchResultURLsReadsTheSavedResultsPage(t *testing.T) {
	got, err := parseSearchResultURLs(registryBase, openFixture(t, searchResultsFixturePath))
	if err != nil {
		t.Fatalf("parseSearchResultURLs(%s) returned error %v", searchResultsFixturePath, err)
	}

	assertURLsEqual(t, got, goldenSearchResultURLs)
}

func TestParseSearchResultURLsKeepsHighlightedResults(t *testing.T) {
	got, err := parseSearchResultURLs(registryBase, openFixture(t, highlightedSearchResultsFixturePath))
	if err != nil {
		t.Fatalf("parseSearchResultURLs(%s) returned error %v", highlightedSearchResultsFixturePath, err)
	}

	assertURLsEqual(t, got, goldenHighlightedSearchResultURLs)
}

// TestParseSearchResultURLsIgnoresSimilarCompanies feeds the parser a company *detail* page.
// The detail page's "similar companies" sidebar uses the same div.block-list > div.list-item
// > div.company > div.company-info > a.company-title markup as a search result and would
// otherwise come back as five perfectly plausible, entirely wrong companies. The block class
// cannot tell them apart — "highlighted" marks a promoted listing on both pages — so the
// detail page is recognised as a whole, by the registration code only it carries.
func TestParseSearchResultURLsIgnoresSimilarCompanies(t *testing.T) {
	got, err := parseSearchResultURLs(registryBase, openFixture(t, companyInfoFixturePath))
	if err != nil {
		t.Fatalf("parseSearchResultURLs(%s) returned error %v", companyInfoFixturePath, err)
	}

	for _, gotURL := range got {
		for _, similarCompanyURL := range similarCompanyURLs {
			if gotURL == similarCompanyURL {
				t.Errorf("parseSearchResultURLs returned similar-company URL %q from a detail page", gotURL)
			}
		}
	}
	if len(got) != 0 {
		t.Fatalf("parseSearchResultURLs on a detail page returned %d URLs, want none: %q", len(got), got)
	}
}

func TestParseSearchResultURLs(t *testing.T) {
	cases := []struct {
		name string
		page string
		want []string
	}{
		{
			name: "empty page",
			page: "",
			want: []string{},
		},
		{
			name: "page without result blocks",
			page: `<html><body><div class="block-list"><div class="list-item"></div></div></body></html>`,
			want: []string{},
		},
		{
			name: "absolute href passes through unchanged",
			page: searchResultsPageWith(`<a class="company-title" href="https://rekvizitai.vz.lt/imone/citma/">Citma</a>`),
			want: []string{"https://rekvizitai.vz.lt/imone/citma/"},
		},
		{
			name: "root relative href resolved against the registry",
			page: searchResultsPageWith(`<a class="company-title" href="/imone/citma/">Citma</a>`),
			want: []string{"https://rekvizitai.vz.lt/imone/citma/"},
		},
		{
			name: "anchor without an href skipped",
			page: searchResultsPageWith(`<a class="company-title">Citma</a>`),
			want: []string{},
		},
		{
			name: "anchor with an empty href skipped",
			page: searchResultsPageWith(`<a class="company-title" href="">Citma</a>`),
			want: []string{},
		},
		{
			name: "unparseable href skipped",
			page: searchResultsPageWith(`<a class="company-title" href="https://rekvizitai.vz.lt:port/imone/citma/">Citma</a>`),
			want: []string{},
		},
		{
			name: "highlighted result block kept",
			page: `<html><body><div class="block-list p-0"><div class="list-item">` +
				`<div class="company highlighted"><div class="company-info">` +
				`<a class="company-title" href="/imone/plius/">Diginet LTU</a>` +
				`</div></div></div></div></body></html>`,
			want: []string{"https://rekvizitai.vz.lt/imone/plius/"},
		},
		{
			name: "document order preserved",
			page: searchResultsPageWith(
				`<a class="company-title" href="/imone/citma/">Citma</a>`,
				`<a class="company-title" href="/imone/vilniaus_citma/">Vilniaus citma</a>`,
			),
			want: []string{
				"https://rekvizitai.vz.lt/imone/citma/",
				"https://rekvizitai.vz.lt/imone/vilniaus_citma/",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := parseSearchResultURLs(registryBase, strings.NewReader(testCase.page))
			if err != nil {
				t.Fatalf("parseSearchResultURLs returned error %v", err)
			}

			assertURLsEqual(t, got, testCase.want)
		})
	}
}

// TestParseSearchResultURLsReportsAFailedRead covers the response body that dies halfway
// through — a dropped connection.
func TestParseSearchResultURLsReportsAFailedRead(t *testing.T) {
	readError := errors.New("connection reset")

	got, err := parseSearchResultURLs(registryBase, iotest.ErrReader(readError))
	if got != nil {
		t.Errorf("parseSearchResultURLs = %q, want nil", got)
	}
	if !errors.Is(err, readError) {
		t.Fatalf("parseSearchResultURLs error = %v, want %v", err, readError)
	}
}

func TestParseSearchResultCountReadsTheSavedResultsPage(t *testing.T) {
	document := parseFixtureDocument(t, searchResultsFixturePath)

	got, stated := parseSearchResultCount(document)
	if !stated {
		t.Fatalf("parseSearchResultCount(%s) reported no stated count", searchResultsFixturePath)
	}
	if got != goldenSearchResultCount {
		t.Errorf("parseSearchResultCount(%s) = %d, want %d", searchResultsFixturePath, got, goldenSearchResultCount)
	}
}

func TestParseSearchResultCount(t *testing.T) {
	cases := []struct {
		name       string
		page       string
		want       int
		wantStated bool
	}{
		{
			name:       "count stated",
			page:       `<div class="breadcrumb"><div>Rasta įmonių: <strong>4</strong></div></div>`,
			want:       4,
			wantStated: true,
		},
		{
			name:       "count wrapped in whitespace",
			page:       "<div class=\"breadcrumb\"><div>Rasta įmonių: <strong>\n    12\n  </strong></div></div>",
			want:       12,
			wantStated: true,
		},
		{
			name:       "no breadcrumb at all",
			page:       `<html><body><p>Nieko nerasta</p></body></html>`,
			want:       0,
			wantStated: false,
		},
		{
			name:       "breadcrumb without a number",
			page:       `<div class="breadcrumb"><div>Rasta įmonių: <strong>daug</strong></div></div>`,
			want:       0,
			wantStated: false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			document, err := goquery.NewDocumentFromReader(strings.NewReader(testCase.page))
			if err != nil {
				t.Fatalf("goquery.NewDocumentFromReader returned error %v", err)
			}

			got, stated := parseSearchResultCount(document)
			if stated != testCase.wantStated {
				t.Fatalf("parseSearchResultCount stated = %t, want %t", stated, testCase.wantStated)
			}
			if got != testCase.want {
				t.Errorf("parseSearchResultCount = %d, want %d", got, testCase.want)
			}
		})
	}
}

// searchResultsPageWith wraps anchors in the nesting a real results page puts them in, so a
// synthetic page exercises the same selector the fixture does.
func searchResultsPageWith(companyTitleLinks ...string) string {
	var page strings.Builder

	page.WriteString(`<html><body><div class="block-list p-0"><div class="list-item">`)
	for _, companyTitleLink := range companyTitleLinks {
		page.WriteString(`<div class="company"><div class="company-info">`)
		page.WriteString(companyTitleLink)
		page.WriteString(`</div></div>`)
	}
	page.WriteString(`</div></div></body></html>`)

	return page.String()
}

// openFixture opens one of the saved registry pages under testdata/. Tests run in the
// package directory, so the relative path resolves; the fixtures are read where they are and
// never copied, they are 300 KB each.
func openFixture(t *testing.T, fixturePath string) *os.File {
	t.Helper()

	fixture, err := os.Open(fixturePath)
	if err != nil {
		t.Fatalf("os.Open(%q) returned error %v", fixturePath, err)
	}
	t.Cleanup(func() {
		fixture.Close()
	})

	return fixture
}

func parseFixtureDocument(t *testing.T, fixturePath string) *goquery.Document {
	t.Helper()

	document, err := goquery.NewDocumentFromReader(openFixture(t, fixturePath))
	if err != nil {
		t.Fatalf("goquery.NewDocumentFromReader(%q) returned error %v", fixturePath, err)
	}

	return document
}

func assertURLsEqual(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d URLs %q, want %d %q", len(got), got, len(want), want)
	}
	for index, wantURL := range want {
		if got[index] != wantURL {
			t.Errorf("URL %d = %q, want %q", index, got[index], wantURL)
		}
	}
}
