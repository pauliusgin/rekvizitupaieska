package rekvizitupaieska

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/iotest"
)

// companyInfoFixturePath is a real company detail page saved from the registry, for
// UAB "ORLEN Apsauga ".
const companyInfoFixturePath = "testdata/company_info_response.html"

// goldenCompanyName carries a trailing space before its closing quote. That space is real —
// it is in the registry's own record, and it appears in two independently generated places
// on the saved page. Do not "fix" it: the test will fail against the fixture.
const goldenCompanyName = `UAB "ORLEN Apsauga "`

var goldenCompanyInfo = CompanyInfo{
	CompanyName:    goldenCompanyName,
	CompanyLogo:    "https://rekvizitai.vz.lt/logos/orlen_apsauga_142511_021.jpg",
	CompanyAddress: "Mažeikių g. 75, Juodeikių k., LT-89467 Mažeikių r.",
	CompanyID:      "302764291",
	CompanyVatID:   "LT100006806712",
}

func TestParseCompanyInfoPageReadsTheSavedDetailPage(t *testing.T) {
	got, found, err := parseCompanyInfoPage(registryBase, openFixture(t, companyInfoFixturePath))
	if err != nil {
		t.Fatalf("parseCompanyInfoPage(%s) returned error %v", companyInfoFixturePath, err)
	}
	if !found {
		t.Fatalf("parseCompanyInfoPage(%s) found = false, want true", companyInfoFixturePath)
	}

	assertCompanyInfoEqual(t, got, goldenCompanyInfo)
}

func TestParseCompanyInfoPage(t *testing.T) {
	cases := []struct {
		name string
		page string
		want CompanyInfo
	}{
		{
			name: "every field present",
			page: companyInfoPageWith(companyInfoPageFields{
				Name:    "UAB &quot;ORLEN Apsauga &quot;",
				LogoSrc: "https://rekvizitai.vz.lt/logos/orlen_apsauga_142511_021.jpg",
				Address: "Mažeikių g. 75, Juodeikių k., LT-89467 Mažeikių r.",
				ID:      "302764291",
				VatID:   "LT100006806712",
			}),
			want: goldenCompanyInfo,
		},
		{
			name: "wrapped address collapsed onto one line",
			page: companyInfoPageWith(companyInfoPageFields{
				Name:    "UAB &quot;ORLEN Apsauga &quot;",
				Address: "Mažeikių g. 75, Juodeikių k., LT-89467 Mažeikių\n                                r.",
				ID:      "302764291",
			}),
			want: CompanyInfo{
				CompanyName:    goldenCompanyName,
				CompanyAddress: "Mažeikių g. 75, Juodeikių k., LT-89467 Mažeikių r.",
				CompanyID:      "302764291",
			},
		},
		{
			name: "company without a VAT code",
			page: companyInfoPageWith(companyInfoPageFields{
				Name:    "MB Bekodė",
				LogoSrc: "/logos/bekode-14581-140.jpg",
				Address: "Kirtimų g. 51, LT-02244 Vilnius",
				ID:      "302764291",
			}),
			want: CompanyInfo{
				CompanyName:    "MB Bekodė",
				CompanyLogo:    "https://rekvizitai.vz.lt/logos/bekode-14581-140.jpg",
				CompanyAddress: "Kirtimų g. 51, LT-02244 Vilnius",
				CompanyID:      "302764291",
			},
		},
		{
			name: "company without a logo",
			page: companyInfoPageWith(companyInfoPageFields{
				Name:    "UAB CITMA ventus",
				Address: "Metalo g. 2A, LT-02190 Vilnius",
				ID:      "302764291",
				VatID:   "LT100006806712",
			}),
			want: CompanyInfo{
				CompanyName:    "UAB CITMA ventus",
				CompanyAddress: "Metalo g. 2A, LT-02190 Vilnius",
				CompanyID:      "302764291",
				CompanyVatID:   "LT100006806712",
			},
		},
		{
			name: "unparseable logo src reported as no logo",
			page: companyInfoPageWith(companyInfoPageFields{
				Name:    "UAB Citma",
				LogoSrc: "https://rekvizitai.vz.lt:port/logos/citma-14580-139.jpg",
				ID:      "302764291",
			}),
			want: CompanyInfo{
				CompanyName: "UAB Citma",
				CompanyID:   "302764291",
			},
		},
		{
			name: "logo src resolved against the registry",
			page: companyInfoPageWith(companyInfoPageFields{
				Name:    "UAB Citma",
				LogoSrc: "/logos/citma-14580-139.jpg",
				ID:      "302764291",
			}),
			want: CompanyInfo{
				CompanyName: "UAB Citma",
				CompanyLogo: "https://rekvizitai.vz.lt/logos/citma-14580-139.jpg",
				CompanyID:   "302764291",
			},
		},
		{
			name: "company without an address",
			page: companyInfoPageWith(companyInfoPageFields{
				Name:    "UAB Citma",
				LogoSrc: "/logos/citma-14580-139.jpg",
				ID:      "302764291",
				VatID:   "LT100006806712",
			}),
			want: CompanyInfo{
				CompanyName:  "UAB Citma",
				CompanyLogo:  "https://rekvizitai.vz.lt/logos/citma-14580-139.jpg",
				CompanyID:    "302764291",
				CompanyVatID: "LT100006806712",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, found, err := parseCompanyInfoPage(registryBase, strings.NewReader(testCase.page))
			if err != nil {
				t.Fatalf("parseCompanyInfoPage returned error %v", err)
			}
			if !found {
				t.Fatalf("parseCompanyInfoPage found = false, want true")
			}

			assertCompanyInfoEqual(t, got, testCase.want)
		})
	}
}

// TestParseCompanyInfoPageReportsAPageThatIsNotACompany pins the line between "this company
// has no VAT code" and "this is not a company page".
//
// The logo, the address and the VAT code are genuinely optional and come back empty — that is
// the table above. The name and the registration code are different: every real company page
// carries both, so a page missing either did not resolve to a company, and reporting it as
// found with empty fields would make a 404 body or a registry redesign look like a successful
// lookup. The caller turns found=false into an [ErrCompanyNotFound].
func TestParseCompanyInfoPageReportsAPageThatIsNotACompany(t *testing.T) {
	cases := []struct {
		name string
		page string
	}{
		{
			name: "empty page",
			page: "",
		},
		{
			name: "not a company page at all",
			page: `<html><body><p>not a company page</p></body></html>`,
		},
		{
			name: "no company name",
			page: companyInfoPageWith(companyInfoPageFields{
				ID: "302764291",
			}),
		},
		{
			name: "no registration code",
			page: companyInfoPageWith(companyInfoPageFields{
				Name:  "UAB Citma",
				VatID: "LT100006806712",
			}),
		},
		{
			name: "registration code node present but empty",
			page: companyInfoPageWith(companyInfoPageFields{
				Name: "UAB Citma",
				ID:   "   ",
			}),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, found, err := parseCompanyInfoPage(registryBase, strings.NewReader(testCase.page))
			if err != nil {
				t.Fatalf("parseCompanyInfoPage error = %v, want nil", err)
			}
			if found {
				t.Errorf("parseCompanyInfoPage found = true, want false")
			}
			if got != (CompanyInfo{}) {
				t.Errorf("parseCompanyInfoPage = %+v, want the zero CompanyInfo", got)
			}
		})
	}
}

// TestParseCompanyInfoPageReportsAFailedRead covers the response body that dies halfway
// through — a dropped connection. It is the only thing this parser still calls an error:
// nothing is wrong with the page, it never arrived.
func TestParseCompanyInfoPageReportsAFailedRead(t *testing.T) {
	readError := errors.New("connection reset")

	got, found, err := parseCompanyInfoPage(registryBase, iotest.ErrReader(readError))
	if got != (CompanyInfo{}) {
		t.Errorf("parseCompanyInfoPage = %+v, want the zero CompanyInfo", got)
	}
	if found {
		t.Errorf("parseCompanyInfoPage found = true, want false")
	}
	if !errors.Is(err, readError) {
		t.Fatalf("parseCompanyInfoPage error = %v, want %v", err, readError)
	}
}

// companyInfoPageFields are the five values a synthetic detail page carries. An empty one
// leaves the node out of the page altogether, which is how the registry renders a company
// that has no logo image, no address or no VAT code.
type companyInfoPageFields struct {
	Name    string
	LogoSrc string
	Address string
	ID      string
	VatID   string
}

// companyInfoPageWith builds a detail page holding only the nodes the five selectors read,
// nested the way the real page nests them. Values are written into the page as raw HTML, so
// a test can pass an entity such as &quot; and check that it comes back decoded.
func companyInfoPageWith(fields companyInfoPageFields) string {
	var page strings.Builder

	page.WriteString(`<html><body><div class="main-info">`)
	if fields.Name != "" {
		fmt.Fprintf(&page, `<div class="name d-md-block"><h1 class="title">%s</h1></div>`, fields.Name)
	}
	if fields.LogoSrc != "" {
		fmt.Fprintf(&page,
			`<div class="logo"><a href="#"><img class="img-fluid" src="%s" /></a></div>`,
			fields.LogoSrc)
	}
	page.WriteString(`</div><div class="company mid-info"><table><tbody>`)
	if fields.Address != "" {
		fmt.Fprintf(&page,
			`<tr itemprop="address"><td class="value"><a class="href"><span id="copy_address">%s</span></a></td></tr>`,
			fields.Address)
	}
	if fields.ID != "" {
		fmt.Fprintf(&page, `<tr><td class="value"><span id="ccode">%s</span></td></tr>`, fields.ID)
	}
	if fields.VatID != "" {
		fmt.Fprintf(&page, `<tr><td class="value value--pvm"><span id="pcode">%s</span></td></tr>`, fields.VatID)
	}
	page.WriteString(`</tbody></table></div></body></html>`)

	return page.String()
}

func assertCompanyInfoEqual(t *testing.T, got, want CompanyInfo) {
	t.Helper()

	if got.CompanyName != want.CompanyName {
		t.Errorf("CompanyName = %q, want %q", got.CompanyName, want.CompanyName)
	}
	if got.CompanyLogo != want.CompanyLogo {
		t.Errorf("CompanyLogo = %q, want %q", got.CompanyLogo, want.CompanyLogo)
	}
	if got.CompanyAddress != want.CompanyAddress {
		t.Errorf("CompanyAddress = %q, want %q", got.CompanyAddress, want.CompanyAddress)
	}
	if got.CompanyID != want.CompanyID {
		t.Errorf("CompanyID = %q, want %q", got.CompanyID, want.CompanyID)
	}
	if got.CompanyVatID != want.CompanyVatID {
		t.Errorf("CompanyVatID = %q, want %q", got.CompanyVatID, want.CompanyVatID)
	}
}
