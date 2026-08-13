package rekvizitupaieska

import (
	"errors"
	"testing"
)

func TestBuildCompanyIDLookupURL(t *testing.T) {
	cases := []struct {
		name      string
		companyID string
		want      string
	}{
		{
			name:      "nine digit code",
			companyID: "302764291",
			want:      "https://rekvizitai.vz.lt/imones/1/?company_code=302764291&order=1&redirected=1&resetFilter=0",
		},
		{
			name:      "leading zero preserved",
			companyID: "012345678",
			want:      "https://rekvizitai.vz.lt/imones/1/?company_code=012345678&order=1&redirected=1&resetFilter=0",
		},
		{
			name:      "whitespace trimmed",
			companyID: "  302764291  ",
			want:      "https://rekvizitai.vz.lt/imones/1/?company_code=302764291&order=1&redirected=1&resetFilter=0",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := buildCompanyIDLookupURL(registryBase, testCase.companyID)
			if err != nil {
				t.Fatalf("buildCompanyIDLookupURL(%q) returned error %v", testCase.companyID, err)
			}
			if got != testCase.want {
				t.Errorf("buildCompanyIDLookupURL(%q) = %q, want %q", testCase.companyID, got, testCase.want)
			}
		})
	}
}

func TestBuildCompanyNameLookupURL(t *testing.T) {
	cases := []struct {
		name        string
		companyName string
		want        string
	}{
		{
			name:        "single term",
			companyName: "orl",
			want:        "https://rekvizitai.vz.lt/imones/1/?name=orl&order=1&redirected=1&resetFilter=0",
		},
		{
			name:        "space becomes a plus",
			companyName: "orl apt",
			want:        "https://rekvizitai.vz.lt/imones/1/?name=orl+apt&order=1&redirected=1&resetFilter=0",
		},
		{
			name:        "a run of separators becomes one plus",
			companyName: "orl.-.-.-.-.apt",
			want:        "https://rekvizitai.vz.lt/imones/1/?name=orl+apt&order=1&redirected=1&resetFilter=0",
		},
		{
			name:        "lithuanian letter percent encoded",
			companyName: "orlaivi" + uOgonek,
			want:        "https://rekvizitai.vz.lt/imones/1/?name=orlaivi%C5%B3&order=1&redirected=1&resetFilter=0",
		},
		{
			name:        "lithuanian letters across two terms",
			companyName: "orlaivi" + uOgonek + " aptarnavimo",
			want:        "https://rekvizitai.vz.lt/imones/1/?name=orlaivi%C5%B3+aptarnavimo&order=1&redirected=1&resetFilter=0",
		},
		{
			name:        "quotes dropped and term lowercased by normalization",
			companyName: `UAB "ORLEN"`,
			want:        "https://rekvizitai.vz.lt/imones/1/?name=uab+orlen&order=1&redirected=1&resetFilter=0",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := buildCompanyNameLookupURL(registryBase, testCase.companyName)
			if err != nil {
				t.Fatalf("buildCompanyNameLookupURL(%q) returned error %v", testCase.companyName, err)
			}
			if got != testCase.want {
				t.Errorf("buildCompanyNameLookupURL(%q) = %q, want %q", testCase.companyName, got, testCase.want)
			}
		})
	}
}

// TestBuildCompanyNameLookupURLComposesDecomposedTerms is the end-to-end form of the NFC
// pass, pinned to the byte the registry receives. The input is the decomposed, capitalised
// spelling of "azuolas" with an ogonek on the A and a caron on the Z; the value on the wire
// must be the composed, lowercased one — %C4%85 is U+0105 a-ogonek and %C5%BE is U+017E
// z-caron.
//
// Without composing, the marks are not letters, the run ends at each base letter, and the
// URL comes out as name=A+z+uolas: three groups, one of them a single letter, which is the
// near-everything search the letter-run guard exists to prevent.
func TestBuildCompanyNameLookupURLComposesDecomposedTerms(t *testing.T) {
	companyName := aOgonekUpperNFD + zCaronUpperNFD + "uolas"
	want := "https://rekvizitai.vz.lt/imones/1/?name=%C4%85%C5%BEuolas&order=1&redirected=1&resetFilter=0"

	got, err := buildCompanyNameLookupURL(registryBase, companyName)
	if err != nil {
		t.Fatalf("buildCompanyNameLookupURL(%q) returned error %v", companyName, err)
	}
	if got != want {
		t.Errorf("buildCompanyNameLookupURL(%q) = %q, want %q", companyName, got, want)
	}
}

// TestBuildCompanyNameLookupURLIsIdenticalForBothSpellings checks the property behind the
// case above: how a caller happened to spell a term must not change the request. The two
// URLs have to be equal byte for byte, not merely both valid.
func TestBuildCompanyNameLookupURLIsIdenticalForBothSpellings(t *testing.T) {
	cases := []struct {
		name       string
		composed   string
		decomposed string
	}{
		{
			name:       "lowercase",
			composed:   aOgonek + zCaron + "uolas",
			decomposed: aOgonekNFD + zCaronNFD + "uolas",
		},
		{
			name:       "uppercase",
			composed:   aOgonekUpper + zCaronUpper + "uolas",
			decomposed: aOgonekUpperNFD + zCaronUpperNFD + "uolas",
		},
		{
			name:       "three letters",
			composed:   aOgonek + cCaron + eOgonek,
			decomposed: aOgonekNFD + cCaronNFD + eOgonekNFD,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fromComposed, err := buildCompanyNameLookupURL(registryBase, testCase.composed)
			if err != nil {
				t.Fatalf("buildCompanyNameLookupURL(%q) returned error %v", testCase.composed, err)
			}

			fromDecomposed, err := buildCompanyNameLookupURL(registryBase, testCase.decomposed)
			if err != nil {
				t.Fatalf("buildCompanyNameLookupURL(%q) returned error %v", testCase.decomposed, err)
			}

			if fromComposed != fromDecomposed {
				t.Errorf("composed gave %q, decomposed gave %q, want the same URL", fromComposed, fromDecomposed)
			}
		})
	}
}

// TestBuildLookupURLEscapesParameterInjection pins the escaping rather than the fact that
// nothing currently reaches it holding an "&" or an "=". Normalization strips both today, so
// this calls buildLookupURL directly: if a later change relaxes normalization, or a new caller
// passes a value straight through, an unescaped separator would end the parameter early and
// let the rest of the value be read as another parameter. The escaping is [net/url.Values.Encode]'s
// today, so the test also fails if the query string is ever hand-assembled again without it.
func TestBuildLookupURLEscapesParameterInjection(t *testing.T) {
	cases := []struct {
		name           string
		queryParameter lookupQueryParameter
		want           string
	}{
		{
			name:           "ampersand and equals in the value",
			queryParameter: lookupQueryParameter{Name: companyNameParameter, Value: "orl&admin=1"},
			want:           "https://rekvizitai.vz.lt/imones/1/?name=orl%26admin%3D1&order=1&redirected=1&resetFilter=0",
		},
		{
			name:           "ampersand and equals in the name",
			queryParameter: lookupQueryParameter{Name: "name&admin=1", Value: "orl"},
			want:           "https://rekvizitai.vz.lt/imones/1/?name%26admin%3D1=orl&order=1&redirected=1&resetFilter=0",
		},
		{
			name:           "question mark and hash in the value",
			queryParameter: lookupQueryParameter{Name: companyNameParameter, Value: "orl?apt#top"},
			want:           "https://rekvizitai.vz.lt/imones/1/?name=orl%3Fapt%23top&order=1&redirected=1&resetFilter=0",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := buildLookupURL(registryBase, testCase.queryParameter); got != testCase.want {
				t.Errorf("buildLookupURL(%#v) = %q, want %q", testCase.queryParameter, got, testCase.want)
			}
		})
	}
}

func TestBuildCompanyIDLookupURLRejectsInvalidCode(t *testing.T) {
	for _, companyID := range []string{"", "30276429", "3027642911", "30276429a"} {
		got, err := buildCompanyIDLookupURL(registryBase, companyID)
		if got != "" {
			t.Errorf("buildCompanyIDLookupURL(%q) = %q, want empty string", companyID, got)
		}

		var invalidCompanyID ErrInvalidCompanyID
		if !errors.As(err, &invalidCompanyID) {
			t.Fatalf("buildCompanyIDLookupURL(%q) error = %v, want ErrInvalidCompanyID", companyID, err)
		}
		if invalidCompanyID.CompanyID != companyID {
			t.Errorf("ErrInvalidCompanyID.CompanyID = %q, want %q", invalidCompanyID.CompanyID, companyID)
		}
	}
}

func TestBuildCompanyNameLookupURLRejectsInvalidName(t *testing.T) {
	for _, companyName := range []string{"", "or la", "o r l", aOgonek + " " + cCaron} {
		got, err := buildCompanyNameLookupURL(registryBase, companyName)
		if got != "" {
			t.Errorf("buildCompanyNameLookupURL(%q) = %q, want empty string", companyName, got)
		}

		var invalidCompanyName ErrInvalidCompanyName
		if !errors.As(err, &invalidCompanyName) {
			t.Fatalf("buildCompanyNameLookupURL(%q) error = %v, want ErrInvalidCompanyName", companyName, err)
		}
		if invalidCompanyName.CompanyName != companyName {
			t.Errorf("ErrInvalidCompanyName.CompanyName = %q, want %q", invalidCompanyName.CompanyName, companyName)
		}
	}
}
