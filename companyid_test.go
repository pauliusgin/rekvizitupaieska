package rekvizitupaieska

import (
	"errors"
	"testing"
)

func TestValidateCompanyIDAcceptsNineDigitCodes(t *testing.T) {
	cases := []struct {
		name      string
		companyID string
		want      string
	}{
		{name: "nine digits", companyID: "302764291", want: "302764291"},
		{name: "all zeros", companyID: "000000000", want: "000000000"},
		{name: "leading zero preserved", companyID: "012345678", want: "012345678"},
		{name: "two leading zeros preserved", companyID: "001234567", want: "001234567"},
		{name: "leading whitespace trimmed", companyID: "  302764291", want: "302764291"},
		{name: "trailing whitespace trimmed", companyID: "302764291\n", want: "302764291"},
		{name: "surrounding whitespace trimmed", companyID: " \t302764291 \n", want: "302764291"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := validateCompanyID(testCase.companyID)
			if err != nil {
				t.Fatalf("validateCompanyID(%q) returned error %v", testCase.companyID, err)
			}
			if got != testCase.want {
				t.Errorf("validateCompanyID(%q) = %q, want %q", testCase.companyID, got, testCase.want)
			}
		})
	}
}

func TestValidateCompanyIDRejectsMalformedCodes(t *testing.T) {
	cases := []struct {
		name      string
		companyID string
	}{
		{name: "empty", companyID: ""},
		{name: "whitespace only", companyID: "   "},
		{name: "eight digits", companyID: "30276429"},
		{name: "ten digits", companyID: "3027642911"},
		{name: "one digit", companyID: "3"},
		{name: "letters mixed in", companyID: "30276429a"},
		{name: "letters only", companyID: "abcdefghi"},
		{name: "inner whitespace", companyID: "302 764291"},
		{name: "hyphenated", companyID: "302-764291"},
		{name: "full-width digits", companyID: "３０２７６４２９１"},
		{name: "eight digits and a full-width one", companyID: "30276429１"},
		{name: "lithuanian letters", companyID: "ąčęėįšųūž"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := validateCompanyID(testCase.companyID)
			if got != "" {
				t.Errorf("validateCompanyID(%q) = %q, want empty string", testCase.companyID, got)
			}

			var invalidCompanyID ErrInvalidCompanyID
			if !errors.As(err, &invalidCompanyID) {
				t.Fatalf("validateCompanyID(%q) error = %v, want ErrInvalidCompanyID", testCase.companyID, err)
			}
			if invalidCompanyID.CompanyID != testCase.companyID {
				t.Errorf("ErrInvalidCompanyID.CompanyID = %q, want %q", invalidCompanyID.CompanyID, testCase.companyID)
			}
		})
	}
}

func TestContainsOnlyASCIIDigits(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{name: "digits", text: "0123456789", want: true},
		{name: "single digit", text: "7", want: true},
		{name: "empty string", text: "", want: true},
		{name: "digit and letter", text: "1a", want: false},
		{name: "digit and space", text: "1 ", want: false},
		{name: "full-width digit", text: "１", want: false},
		{name: "lithuanian letter", text: "ą", want: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := containsOnlyASCIIDigits(testCase.text); got != testCase.want {
				t.Errorf("containsOnlyASCIIDigits(%q) = %t, want %t", testCase.text, got, testCase.want)
			}
		})
	}
}
