package rekvizitupaieska

import (
	"errors"
	"reflect"
	"testing"
)

// The Lithuanian letters below are built from their code points rather than pasted in, and
// the source of this file holds no character outside ASCII.
//
// Several cases turn on whether a term is composed or decomposed, and the two spellings are
// indistinguishable on screen: a-ogonek written as one character and a-ogonek written as an
// "a" followed by a combining mark both show as the same glyph. A pasted literal would leave
// the next reader no way to tell which one a case is testing, and an editor, a paste, or a
// tool that normalizes text on save can silently rewrite one into the other — which is the
// very bug these cases exist to catch.
const (
	aOgonek = string(rune(0x0105)) // a-ogonek, composed
	cCaron  = string(rune(0x010D)) // c-caron, composed
	eOgonek = string(rune(0x0119)) // e-ogonek, composed
	uOgonek = string(rune(0x0173)) // u-ogonek, composed
	zCaron  = string(rune(0x017E)) // z-caron, composed

	aOgonekUpper = string(rune(0x0104)) // A-ogonek, composed
	uOgonekUpper = string(rune(0x0172)) // U-ogonek, composed
	zCaronUpper  = string(rune(0x017D)) // Z-caron, composed

	combiningOgonek = string(rune(0x0328)) // U+0328 COMBINING OGONEK, category Mn
	combiningCaron  = string(rune(0x030C)) // U+030C COMBINING CARON, category Mn

	aOgonekNFD      = "a" + combiningOgonek // a-ogonek, decomposed
	cCaronNFD       = "c" + combiningCaron  // c-caron, decomposed
	eOgonekNFD      = "e" + combiningOgonek // e-ogonek, decomposed
	zCaronNFD       = "z" + combiningCaron  // z-caron, decomposed
	aOgonekUpperNFD = "A" + combiningOgonek // A-ogonek, decomposed
	zCaronUpperNFD  = "Z" + combiningCaron  // Z-caron, decomposed

	microSign     = string(rune(0x00B5)) // U+00B5 MICRO SIGN, category Ll
	ordinalFemale = string(rune(0x00AA)) // U+00AA FEMININE ORDINAL INDICATOR, category Lo
	modifierMark  = string(rune(0x02BC)) // U+02BC MODIFIER LETTER APOSTROPHE, category Lm
)

func TestNormalizeCompanyNameAcceptsTermsWithALetterRun(t *testing.T) {
	cases := []struct {
		name        string
		companyName string
		want        string
	}{
		{name: "three letters", companyName: "orl", want: "orl"},
		{name: "run followed by a single letter", companyName: "orl a", want: "orl a"},
		{name: "single letter followed by a run", companyName: "a orl", want: "a orl"},
		{name: "two runs", companyName: "orl apt", want: "orl apt"},
		{name: "reversed runs keep their order", companyName: "apt orl", want: "apt orl"},
		{name: "hyphen separated", companyName: "orl-apt", want: "orl apt"},
		{name: "run of separators collapses to one space", companyName: "orl.-.-.-.-.apt", want: "orl apt"},
		{name: "repeated spaces collapsed", companyName: "  orl   apt  ", want: "orl apt"},
		{name: "leading and trailing junk stripped", companyName: "...orl apt!!!", want: "orl apt"},
		{name: "digits dropped", companyName: "orl1apt", want: "orl apt"},
		{
			name:        "lithuanian letters",
			companyName: aOgonek + cCaron + eOgonek,
			want:        aOgonek + cCaron + eOgonek,
		},
		{
			name:        "lithuanian letters in a longer term",
			companyName: "orlaivi" + uOgonek + " aptarnavimo",
			want:        "orlaivi" + uOgonek + " aptarnavimo",
		},
		{name: "uppercase lowercased", companyName: "ORLEN", want: "orlen"},
		{name: "mixed case lowercased", companyName: "OrLeN", want: "orlen"},
		{
			name:        "uppercase lithuanian letters lowercased",
			companyName: "ORLAIVI" + uOgonekUpper,
			want:        "orlaivi" + uOgonek,
		},
		{name: "quoted legal form", companyName: `UAB "ORLEN Apsauga"`, want: "uab orlen apsauga"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := normalizeCompanyName(testCase.companyName)
			if err != nil {
				t.Fatalf("normalizeCompanyName(%q) returned error %v", testCase.companyName, err)
			}
			if got != testCase.want {
				t.Errorf("normalizeCompanyName(%q) = %q, want %q", testCase.companyName, got, testCase.want)
			}
		})
	}
}

func TestNormalizeCompanyNameRejectsTermsWithoutALetterRun(t *testing.T) {
	cases := []struct {
		name        string
		companyName string
	}{
		{name: "empty", companyName: ""},
		{name: "whitespace only", companyName: "   "},
		{name: "punctuation only", companyName: ".-.-.-"},
		{name: "digits only", companyName: "302764291"},
		{name: "single letter", companyName: "o"},
		{name: "two letters", companyName: "or"},
		{name: "two runs of two", companyName: "or la"},
		{name: "three separated letters", companyName: "o r l"},
		{name: "three letters split by punctuation", companyName: "o.r.l"},
		{name: "two separated lithuanian letters", companyName: aOgonek + " " + cCaron},
		// Composing a decomposed term must not manufacture a run out of nothing: this is two
		// letters either way, and two is under the guard.
		{name: "two decomposed lithuanian letters", companyName: aOgonekNFD + cCaronNFD},
		// U+02BC MODIFIER LETTER APOSTROPHE (Lm) and U+00AA FEMININE ORDINAL INDICATOR (Lo)
		// are letters to unicode.IsLetter, so either would splice "or" and "l" into a run of
		// four and certify a term the registry then reads as the two short groups "or" and
		// "l". Only the cased categories count here, so both are separators.
		{name: "modifier letter apostrophe is not a letter", companyName: "or" + modifierMark + "l"},
		{name: "feminine ordinal indicator is not a letter", companyName: "or" + ordinalFemale + "l"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := normalizeCompanyName(testCase.companyName)
			if got != "" {
				t.Errorf("normalizeCompanyName(%q) = %q, want empty string", testCase.companyName, got)
			}

			var invalidCompanyName ErrInvalidCompanyName
			if !errors.As(err, &invalidCompanyName) {
				t.Fatalf("normalizeCompanyName(%q) error = %v, want ErrInvalidCompanyName", testCase.companyName, err)
			}
			if invalidCompanyName.CompanyName != testCase.companyName {
				t.Errorf("ErrInvalidCompanyName.CompanyName = %q, want %q", invalidCompanyName.CompanyName, testCase.companyName)
			}
		})
	}
}

// TestNormalizeCompanyNameComposesBeforeExtracting pins the reason the NFC pass exists: a
// decomposed term must come out as the letters it spells, not as its base letters with the
// combining marks dropped. Without composing, the decomposed spelling of "azuolas" with its
// ogonek and caron extracts as "a", "z" and "uolas" — three near-useless groups, and exactly
// the near-everything search the letter-run guard exists to prevent.
func TestNormalizeCompanyNameComposesBeforeExtracting(t *testing.T) {
	wantAzuolas := aOgonek + zCaron + "uolas"

	cases := []struct {
		name        string
		companyName string
		want        string
	}{
		{
			name:        "decomposed lowercase term",
			companyName: aOgonekNFD + zCaronNFD + "uolas",
			want:        wantAzuolas,
		},
		{
			name:        "decomposed uppercase term",
			companyName: aOgonekUpperNFD + zCaronUpperNFD + "uolas",
			want:        wantAzuolas,
		},
		{
			name:        "composed lowercase term is unchanged",
			companyName: wantAzuolas,
			want:        wantAzuolas,
		},
		{
			name:        "composed uppercase term",
			companyName: aOgonekUpper + zCaronUpper + "uolas",
			want:        wantAzuolas,
		},
		{
			name:        "three decomposed letters make a run",
			companyName: aOgonekNFD + cCaronNFD + eOgonekNFD,
			want:        aOgonek + cCaron + eOgonek,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := normalizeCompanyName(testCase.companyName)
			if err != nil {
				t.Fatalf("normalizeCompanyName(%q) returned error %v", testCase.companyName, err)
			}
			if got != testCase.want {
				t.Errorf("normalizeCompanyName(%q) = %q, want %q", testCase.companyName, got, testCase.want)
			}
		})
	}
}

// TestNormalizeCompanyNameAcceptsMicroSignAsALetter documents a gap the narrowing to the
// cased categories does not close. U+00B5 MICRO SIGN is category Ll, not Lo or Lm, so it
// passes the cased-letter test and still splices "or" and "l" into a run of four — while the
// registry, which ignores it, searches for "or" and "l". Closing it takes a rule beyond the
// category test. The case is here so the behavior is visible and a later fix is a test
// change rather than a discovery.
func TestNormalizeCompanyNameAcceptsMicroSignAsALetter(t *testing.T) {
	companyName := "or" + microSign + "l"

	got, err := normalizeCompanyName(companyName)
	if err != nil {
		t.Fatalf("normalizeCompanyName(%q) returned error %v", companyName, err)
	}
	if got != companyName {
		t.Errorf("normalizeCompanyName(%q) = %q, want %q", companyName, got, companyName)
	}
}

// TestNormalizeCompanyNameEmitsSpacesNotPluses pins the separator: the "+" the request URL
// carries is produced by the query-string encoding, not here.
func TestNormalizeCompanyNameEmitsSpacesNotPluses(t *testing.T) {
	got, err := normalizeCompanyName("orl+apt")
	if err != nil {
		t.Fatalf("normalizeCompanyName returned error %v", err)
	}
	if got != "orl apt" {
		t.Errorf("normalizeCompanyName(%q) = %q, want %q", "orl+apt", got, "orl apt")
	}
}

func TestExtractLetterRuns(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{name: "empty string", text: "", want: nil},
		{name: "no letters", text: "12 .-", want: nil},
		{name: "one run", text: "orl", want: []string{"orl"}},
		{name: "two runs", text: "orl apt", want: []string{"orl", "apt"}},
		{name: "surrounding junk", text: "..orl..apt..", want: []string{"orl", "apt"}},
		{name: "single letters", text: "o r l", want: []string{"o", "r", "l"}},
		{
			name: "lithuanian letters stay whole",
			text: "orlaivi" + uOgonek,
			want: []string{"orlaivi" + uOgonek},
		},
		// A combining mark is category Mn and so is not a letter here. extractLetterRuns is
		// handed composed text by normalizeCompanyName; this is what it does without that.
		{
			name: "combining mark splits a run",
			text: aOgonekNFD + "bc",
			want: []string{"a", "bc"},
		},
		{
			name: "modifier letter apostrophe splits a run",
			text: "or" + modifierMark + "l",
			want: []string{"or", "l"},
		},
		{
			name: "feminine ordinal indicator splits a run",
			text: "or" + ordinalFemale + "l",
			want: []string{"or", "l"},
		},
		{
			name: "micro sign does not split a run",
			text: "or" + microSign + "l",
			want: []string{"or" + microSign + "l"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := extractLetterRuns(testCase.text); !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("extractLetterRuns(%q) = %#v, want %#v", testCase.text, got, testCase.want)
			}
		})
	}
}

// TestLongestLetterRunLengthCountsLettersNotBytes is the test that catches a byte loop: three
// Lithuanian letters are written in six bytes.
func TestLongestLetterRunLengthCountsLettersNotBytes(t *testing.T) {
	cases := []struct {
		name       string
		letterRuns []string
		want       int
	}{
		{name: "no runs", letterRuns: nil, want: 0},
		{name: "one ascii run", letterRuns: []string{"orl"}, want: 3},
		{name: "longest of several", letterRuns: []string{"a", "orl", "ap"}, want: 3},
		{name: "three lithuanian letters", letterRuns: []string{aOgonek + cCaron + eOgonek}, want: 3},
		{name: "two lithuanian letters", letterRuns: []string{aOgonek + cCaron}, want: 2},
		{name: "mixed ascii and lithuanian", letterRuns: []string{"orlaivi" + uOgonek}, want: 8},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := longestLetterRunLength(testCase.letterRuns); got != testCase.want {
				t.Errorf("longestLetterRunLength(%#v) = %d, want %d", testCase.letterRuns, got, testCase.want)
			}
		})
	}
}
