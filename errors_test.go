package rekvizitupaieska

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorMessagesCarryTheOffendingValue(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "invalid company ID",
			err:  ErrInvalidCompanyID{CompanyID: "30276429"},
			want: `invalid company ID "30276429": expected exactly 9 digits`,
		},
		{
			name: "invalid company name",
			err:  ErrInvalidCompanyName{CompanyName: "or la"},
			want: `invalid company name "or la": expected at least 3 consecutive letters`,
		},
		{
			name: "too many matches",
			err: ErrTooManyMatches{
				CompanyName: "orl",
				MatchCount:  7,
				MatchURLs:   []string{"https://rekvizitai.vz.lt/imone/orlen/"},
			},
			want: `company name "orl" matched 7 companies: narrow the search term`,
		},
		{
			name: "company not found",
			err:  ErrCompanyNotFound{Query: "302764291"},
			want: `no company found for "302764291"`,
		},
		{
			name: "registry request failed",
			err: ErrRegistryRequestFailed{
				URL:        "https://rekvizitai.vz.lt/imone/orlen_apsauga/",
				StatusCode: 503,
			},
			want: `registry request for "https://rekvizitai.vz.lt/imone/orlen_apsauga/" returned status 503`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.err.Error(); got != testCase.want {
				t.Errorf("Error() = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestErrorsAreReachableThroughWrapping checks that every error type survives being wrapped
// by a caller, which is the reason they are types matched with errors.As rather than
// sentinel values.
func TestErrorsAreReachableThroughWrapping(t *testing.T) {
	t.Run("invalid company ID", func(t *testing.T) {
		wrapped := fmt.Errorf("lookup failed: %w", ErrInvalidCompanyID{CompanyID: "30276429"})

		var invalidCompanyID ErrInvalidCompanyID
		if !errors.As(wrapped, &invalidCompanyID) {
			t.Fatalf("errors.As(%v) did not match ErrInvalidCompanyID", wrapped)
		}
		if invalidCompanyID.CompanyID != "30276429" {
			t.Errorf("ErrInvalidCompanyID.CompanyID = %q, want %q", invalidCompanyID.CompanyID, "30276429")
		}
	})

	t.Run("invalid company name", func(t *testing.T) {
		wrapped := fmt.Errorf("lookup failed: %w", ErrInvalidCompanyName{CompanyName: "or la"})

		var invalidCompanyName ErrInvalidCompanyName
		if !errors.As(wrapped, &invalidCompanyName) {
			t.Fatalf("errors.As(%v) did not match ErrInvalidCompanyName", wrapped)
		}
		if invalidCompanyName.CompanyName != "or la" {
			t.Errorf("ErrInvalidCompanyName.CompanyName = %q, want %q", invalidCompanyName.CompanyName, "or la")
		}
	})

	t.Run("too many matches", func(t *testing.T) {
		matchURLs := []string{
			"https://rekvizitai.vz.lt/imone/orlen/",
			"https://rekvizitai.vz.lt/imone/orlen_apsauga/",
		}
		wrapped := fmt.Errorf("lookup failed: %w", ErrTooManyMatches{
			CompanyName: "orl",
			MatchCount:  len(matchURLs),
			MatchURLs:   matchURLs,
		})

		var tooManyMatches ErrTooManyMatches
		if !errors.As(wrapped, &tooManyMatches) {
			t.Fatalf("errors.As(%v) did not match ErrTooManyMatches", wrapped)
		}
		if tooManyMatches.MatchCount != len(matchURLs) {
			t.Errorf("ErrTooManyMatches.MatchCount = %d, want %d", tooManyMatches.MatchCount, len(matchURLs))
		}
		if len(tooManyMatches.MatchURLs) != len(matchURLs) {
			t.Fatalf("ErrTooManyMatches.MatchURLs has %d entries, want %d", len(tooManyMatches.MatchURLs), len(matchURLs))
		}
		for index, matchURL := range matchURLs {
			if tooManyMatches.MatchURLs[index] != matchURL {
				t.Errorf("ErrTooManyMatches.MatchURLs[%d] = %q, want %q", index, tooManyMatches.MatchURLs[index], matchURL)
			}
		}
	})

	t.Run("company not found", func(t *testing.T) {
		wrapped := fmt.Errorf("lookup failed: %w", ErrCompanyNotFound{Query: "orl apt"})

		var companyNotFound ErrCompanyNotFound
		if !errors.As(wrapped, &companyNotFound) {
			t.Fatalf("errors.As(%v) did not match ErrCompanyNotFound", wrapped)
		}
		if companyNotFound.Query != "orl apt" {
			t.Errorf("ErrCompanyNotFound.Query = %q, want %q", companyNotFound.Query, "orl apt")
		}
	})

	t.Run("registry request failed", func(t *testing.T) {
		wrapped := fmt.Errorf("lookup failed: %w", ErrRegistryRequestFailed{
			URL:        "https://rekvizitai.vz.lt/imone/orlen_apsauga/",
			StatusCode: 429,
		})

		var requestFailed ErrRegistryRequestFailed
		if !errors.As(wrapped, &requestFailed) {
			t.Fatalf("errors.As(%v) did not match ErrRegistryRequestFailed", wrapped)
		}
		if requestFailed.StatusCode != 429 {
			t.Errorf("ErrRegistryRequestFailed.StatusCode = %d, want %d", requestFailed.StatusCode, 429)
		}
	})
}
