package rekvizitupaieska

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// The detail page paths the synthetic results pages link to. They are root-relative on
// purpose: an absolute href resolves to whatever host it names, so a synthetic page written
// with the registry's own URLs would send the client at the live site instead of the test
// server. The saved results-page fixture does carry absolute registry URLs — it is only ever
// used where the gate stops the client before it fetches any of them.
const (
	orlenCompanyPath  = "/imone/orlen_apsauga/"
	citmaCompanyPath  = "/imone/citma/"
	ventusCompanyPath = "/imone/citma_ventus/"
)

func TestGetCompanyInfoByIDReadsTheSavedDetailPage(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: resultsPageWith(1, orlenCompanyPath)},
		orlenCompanyPath:   {FixturePath: companyInfoFixturePath},
	})
	client := newClientForTestServer(t, server)

	got, err := client.GetCompanyInfoByID(t.Context(), goldenCompanyInfo.CompanyID)
	if err != nil {
		t.Fatalf("GetCompanyInfoByID returned error %v", err)
	}

	assertCompanyInfoEqual(t, got, companyInfoServedFrom(server, goldenCompanyInfo))
	if server.RequestCount() != 2 {
		t.Errorf("server saw %d requests, want 2 — one lookup and one detail page", server.RequestCount())
	}
}

// TestGetCompanyInfoByIDFindsAHighlightedResult covers a company whose only result block is a
// promoted listing, as Diginet LTU's (126222639) is on the live registry.
func TestGetCompanyInfoByIDFindsAHighlightedResult(t *testing.T) {
	highlightedResultsPage := strings.Replace(
		resultsPageWith(1, orlenCompanyPath), `class="company"`, `class="company highlighted"`, 1)
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: highlightedResultsPage},
		orlenCompanyPath:   {FixturePath: companyInfoFixturePath},
	})
	client := newClientForTestServer(t, server)

	got, err := client.GetCompanyInfoByID(t.Context(), goldenCompanyInfo.CompanyID)
	if err != nil {
		t.Fatalf("GetCompanyInfoByID returned error %v", err)
	}

	assertCompanyInfoEqual(t, got, companyInfoServedFrom(server, goldenCompanyInfo))
}

func TestGetCompanyInfoByIDReportsNoResults(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: resultsPageWith(0)},
	})
	client := newClientForTestServer(t, server)

	got, err := client.GetCompanyInfoByID(t.Context(), "302764291")
	if got != (CompanyInfo{}) {
		t.Errorf("GetCompanyInfoByID = %+v, want the zero CompanyInfo", got)
	}

	var companyNotFound ErrCompanyNotFound
	if !errors.As(err, &companyNotFound) {
		t.Fatalf("GetCompanyInfoByID error = %v, want ErrCompanyNotFound", err)
	}
	if companyNotFound.Query != "302764291" {
		t.Errorf("ErrCompanyNotFound.Query = %q, want %q", companyNotFound.Query, "302764291")
	}
}

func TestGetCompanyInfoByNameReturnsOneCompanyPerMatchInPageOrder(t *testing.T) {
	cases := []struct {
		name         string
		companyPaths []string
		want         []CompanyInfo
	}{
		{
			name:         "one match",
			companyPaths: []string{citmaCompanyPath},
			want:         []CompanyInfo{citmaCompanyInfo},
		},
		{
			name:         "three matches in the order the page listed them",
			companyPaths: []string{ventusCompanyPath, citmaCompanyPath, orlenCompanyPath},
			want:         []CompanyInfo{ventusCompanyInfo, citmaCompanyInfo, orlenCompanyInfo},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := newRegistryTestServer(t, registryTestPages{
				registryLookupPath: {Body: resultsPageWith(len(testCase.companyPaths), testCase.companyPaths...)},
				citmaCompanyPath:   {Body: detailPageFor(citmaCompanyInfo)},
				ventusCompanyPath:  {Body: detailPageFor(ventusCompanyInfo)},
				orlenCompanyPath:   {Body: detailPageFor(orlenCompanyInfo)},
			})
			client := newClientForTestServer(t, server)

			got, err := client.GetCompanyInfoByName(t.Context(), "citma")
			if err != nil {
				t.Fatalf("GetCompanyInfoByName returned error %v", err)
			}
			if len(got) != len(testCase.want) {
				t.Fatalf("GetCompanyInfoByName returned %d companies, want %d", len(got), len(testCase.want))
			}

			for index, wantCompanyInfo := range testCase.want {
				assertCompanyInfoEqual(t, got[index], companyInfoServedFrom(server, wantCompanyInfo))
			}
		})
	}
}

// TestGetCompanyInfoByNameGatesTooManyMatches is the gate against the saved results page,
// which is the "citma" search and states four matches. The count of requests is asserted
// because "no detail page was fetched" is the entire point of the gate and nothing in the
// returned error would show it.
func TestGetCompanyInfoByNameGatesTooManyMatches(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {FixturePath: searchResultsFixturePath},
	})
	client := newClientForTestServer(t, server)

	got, err := client.GetCompanyInfoByName(t.Context(), "citma")
	if got != nil {
		t.Errorf("GetCompanyInfoByName = %+v, want nil", got)
	}

	var tooManyMatches ErrTooManyMatches
	if !errors.As(err, &tooManyMatches) {
		t.Fatalf("GetCompanyInfoByName error = %v, want ErrTooManyMatches", err)
	}
	if tooManyMatches.CompanyName != "citma" {
		t.Errorf("ErrTooManyMatches.CompanyName = %q, want %q", tooManyMatches.CompanyName, "citma")
	}
	if tooManyMatches.MatchCount != goldenSearchResultCount {
		t.Errorf("ErrTooManyMatches.MatchCount = %d, want %d", tooManyMatches.MatchCount, goldenSearchResultCount)
	}
	assertURLsEqual(t, tooManyMatches.MatchURLs, goldenSearchResultURLs)

	if server.RequestCount() != 1 {
		t.Errorf("server saw %d requests, want 1 — the gate must fetch no detail page", server.RequestCount())
	}
}

// TestGetCompanyInfoByNameGatesOnTheStatedCount covers the disagreement between what a results
// page states and what it renders. Two links, five matches stated: the gate has to trip on the
// five, because the search really did match more companies than the library will fetch.
func TestGetCompanyInfoByNameGatesOnTheStatedCount(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: resultsPageWith(5, citmaCompanyPath, ventusCompanyPath)},
		citmaCompanyPath:   {Body: detailPageFor(citmaCompanyInfo)},
		ventusCompanyPath:  {Body: detailPageFor(ventusCompanyInfo)},
	})
	client := newClientForTestServer(t, server)

	_, err := client.GetCompanyInfoByName(t.Context(), "citma")

	var tooManyMatches ErrTooManyMatches
	if !errors.As(err, &tooManyMatches) {
		t.Fatalf("GetCompanyInfoByName error = %v, want ErrTooManyMatches", err)
	}
	if tooManyMatches.MatchCount != 5 {
		t.Errorf("ErrTooManyMatches.MatchCount = %d, want the stated 5", tooManyMatches.MatchCount)
	}
	if server.RequestCount() != 1 {
		t.Errorf("server saw %d requests, want 1 — the gate must fetch no detail page", server.RequestCount())
	}
}

// TestGetCompanyInfoByNameRespectsMaxMatches checks the gate moves with the client's limit
// rather than being pinned to the three the constant holds.
func TestGetCompanyInfoByNameRespectsMaxMatches(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: resultsPageWith(2, citmaCompanyPath, ventusCompanyPath)},
		citmaCompanyPath:   {Body: detailPageFor(citmaCompanyInfo)},
		ventusCompanyPath:  {Body: detailPageFor(ventusCompanyInfo)},
	})
	client := newClientForTestServer(t, server)
	client.maxMatches = 1

	_, err := client.GetCompanyInfoByName(t.Context(), "citma")

	var tooManyMatches ErrTooManyMatches
	if !errors.As(err, &tooManyMatches) {
		t.Fatalf("GetCompanyInfoByName error = %v, want ErrTooManyMatches", err)
	}
	if server.RequestCount() != 1 {
		t.Errorf("server saw %d requests, want 1 — the gate must fetch no detail page", server.RequestCount())
	}
}

func TestGetCompanyInfoByNameReportsNoResults(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: resultsPageWith(0)},
	})
	client := newClientForTestServer(t, server)

	got, err := client.GetCompanyInfoByName(t.Context(), "citma")
	if got != nil {
		t.Errorf("GetCompanyInfoByName = %+v, want nil", got)
	}

	var companyNotFound ErrCompanyNotFound
	if !errors.As(err, &companyNotFound) {
		t.Fatalf("GetCompanyInfoByName error = %v, want ErrCompanyNotFound", err)
	}
	if companyNotFound.Query != "citma" {
		t.Errorf("ErrCompanyNotFound.Query = %q, want %q", companyNotFound.Query, "citma")
	}
}

// TestLookupsRejectInvalidInputWithoutARequest is the half of validation that no return value
// shows: a code or a term the library already knows the registry cannot use must not become a
// request at all.
func TestLookupsRejectInvalidInputWithoutARequest(t *testing.T) {
	cases := []struct {
		name   string
		lookup func(ctx context.Context, client *Client) error
	}{
		{
			name: "company ID too short",
			lookup: func(ctx context.Context, client *Client) error {
				_, err := client.GetCompanyInfoByID(ctx, "30276429")
				return err
			},
		},
		{
			name: "company ID not digits",
			lookup: func(ctx context.Context, client *Client) error {
				_, err := client.GetCompanyInfoByID(ctx, "30276429a")
				return err
			},
		},
		{
			name: "company name without a long enough letter run",
			lookup: func(ctx context.Context, client *Client) error {
				_, err := client.GetCompanyInfoByName(ctx, "or la")
				return err
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := newRegistryTestServer(t, registryTestPages{
				registryLookupPath: {Body: resultsPageWith(1, citmaCompanyPath)},
				citmaCompanyPath:   {Body: detailPageFor(citmaCompanyInfo)},
			})
			client := newClientForTestServer(t, server)

			if err := testCase.lookup(t.Context(), client); err == nil {
				t.Fatal("lookup returned no error, want a validation error")
			}
			if server.RequestCount() != 0 {
				t.Errorf("server saw %d requests, want 0 — invalid input must never be sent",
					server.RequestCount())
			}
		})
	}
}

func TestLookupsReportANon2xxLookupPage(t *testing.T) {
	cases := []struct {
		name   string
		lookup func(ctx context.Context, client *Client) error
	}{
		{
			name: "by ID",
			lookup: func(ctx context.Context, client *Client) error {
				_, err := client.GetCompanyInfoByID(ctx, "302764291")
				return err
			},
		},
		{
			name: "by name",
			lookup: func(ctx context.Context, client *Client) error {
				_, err := client.GetCompanyInfoByName(ctx, "citma")
				return err
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := newRegistryTestServer(t, registryTestPages{
				registryLookupPath: {StatusCode: http.StatusServiceUnavailable, Body: "down"},
			})
			client := newClientForTestServer(t, server)

			err := testCase.lookup(t.Context(), client)

			var requestFailed ErrRegistryRequestFailed
			if !errors.As(err, &requestFailed) {
				t.Fatalf("lookup error = %v, want ErrRegistryRequestFailed", err)
			}
			if requestFailed.StatusCode != http.StatusServiceUnavailable {
				t.Errorf("ErrRegistryRequestFailed.StatusCode = %d, want %d",
					requestFailed.StatusCode, http.StatusServiceUnavailable)
			}
		})
	}
}

func TestGetCompanyInfoByIDReportsANon2xxDetailPage(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: resultsPageWith(1, orlenCompanyPath)},
		orlenCompanyPath:   {StatusCode: http.StatusInternalServerError, Body: "boom"},
	})
	client := newClientForTestServer(t, server)

	got, err := client.GetCompanyInfoByID(t.Context(), "302764291")
	if got != (CompanyInfo{}) {
		t.Errorf("GetCompanyInfoByID = %+v, want the zero CompanyInfo", got)
	}

	var requestFailed ErrRegistryRequestFailed
	if !errors.As(err, &requestFailed) {
		t.Fatalf("GetCompanyInfoByID error = %v, want ErrRegistryRequestFailed", err)
	}
	if requestFailed.StatusCode != http.StatusInternalServerError {
		t.Errorf("ErrRegistryRequestFailed.StatusCode = %d, want %d",
			requestFailed.StatusCode, http.StatusInternalServerError)
	}
	if requestFailed.URL != server.URL()+orlenCompanyPath {
		t.Errorf("ErrRegistryRequestFailed.URL = %q, want %q", requestFailed.URL, server.URL()+orlenCompanyPath)
	}
}

// TestGetCompanyInfoByIDWrapsATransportError covers the request that never got an answer. The
// error must stay reachable through the wrapping — a caller deciding whether to retry needs
// the underlying error, and a message-only wrap would take it away.
func TestGetCompanyInfoByIDWrapsATransportError(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {HangUp: true},
	})
	client := newClientForTestServer(t, server)

	_, err := client.GetCompanyInfoByID(t.Context(), "302764291")
	if err == nil {
		t.Fatal("GetCompanyInfoByID returned no error, want a transport error")
	}

	var urlError *url.Error
	if !errors.As(err, &urlError) {
		t.Fatalf("GetCompanyInfoByID error = %v, want a wrapped *url.Error", err)
	}
	if !strings.Contains(err.Error(), "registry request for") {
		t.Errorf("error %q does not say which request failed", err)
	}
	if errors.As(err, new(ErrRegistryRequestFailed)) {
		t.Error("a request that got no answer must not be an ErrRegistryRequestFailed")
	}
}

// TestGetCompanyInfoByNameReturnsTheFirstErrorInIndexOrder pins determinism, which is the one
// property concurrency can quietly take away. Two of the three detail pages fail, with
// different statuses so they can be told apart; the error must always be the one from the
// earlier page in results-page order, never whichever goroutine happened to finish first.
func TestGetCompanyInfoByNameReturnsTheFirstErrorInIndexOrder(t *testing.T) {
	const failingPageRuns = 20

	for range failingPageRuns {
		server := newRegistryTestServer(t, registryTestPages{
			registryLookupPath: {Body: resultsPageWith(3, citmaCompanyPath, ventusCompanyPath, orlenCompanyPath)},
			citmaCompanyPath:   {Body: detailPageFor(citmaCompanyInfo)},
			ventusCompanyPath:  {StatusCode: http.StatusBadGateway, Body: "second"},
			orlenCompanyPath:   {StatusCode: http.StatusNotFound, Body: "third"},
		})
		client := newClientForTestServer(t, server)

		got, err := client.GetCompanyInfoByName(t.Context(), "citma")
		if got != nil {
			t.Fatalf("GetCompanyInfoByName = %+v, want nil", got)
		}

		var requestFailed ErrRegistryRequestFailed
		if !errors.As(err, &requestFailed) {
			t.Fatalf("GetCompanyInfoByName error = %v, want ErrRegistryRequestFailed", err)
		}
		if requestFailed.StatusCode != http.StatusBadGateway {
			t.Fatalf("ErrRegistryRequestFailed.StatusCode = %d, want %d from the second result",
				requestFailed.StatusCode, http.StatusBadGateway)
		}

		// Every goroutine must have finished before the error came back, so the third page
		// was fetched even though the second one already doomed the call.
		if server.RequestCountFor(orlenCompanyPath) != 1 {
			t.Fatalf("third detail page requested %d times, want 1 — every fetch must finish before returning",
				server.RequestCountFor(orlenCompanyPath))
		}
	}
}

// TestGetCompanyInfoByIDReportsAPageThatIsNotACompany covers the case where the registry lists
// a match but the page behind it holds no company. It is an [ErrCompanyNotFound], and the Query
// it carries is the registration code the caller passed — not the URL this library derived from
// it, which the caller never saw and cannot act on.
func TestGetCompanyInfoByIDReportsAPageThatIsNotACompany(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: resultsPageWith(1, orlenCompanyPath)},
		orlenCompanyPath:   {Body: `<html><body><p>not a company page</p></body></html>`},
	})
	client := newClientForTestServer(t, server)

	got, err := client.GetCompanyInfoByID(t.Context(), "302764291")
	if got != (CompanyInfo{}) {
		t.Errorf("GetCompanyInfoByID = %+v, want the zero CompanyInfo", got)
	}

	var companyNotFound ErrCompanyNotFound
	if !errors.As(err, &companyNotFound) {
		t.Fatalf("GetCompanyInfoByID error = %v, want ErrCompanyNotFound", err)
	}
	if companyNotFound.Query != "302764291" {
		t.Errorf("ErrCompanyNotFound.Query = %q, want %q", companyNotFound.Query, "302764291")
	}
}

// TestGetCompanyInfoByNameReportsAPageThatIsNotACompany is the same rule on the name path,
// where the Query is the search term rather than a code.
func TestGetCompanyInfoByNameReportsAPageThatIsNotACompany(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: resultsPageWith(1, orlenCompanyPath)},
		orlenCompanyPath:   {Body: `<html><body><p>not a company page</p></body></html>`},
	})
	client := newClientForTestServer(t, server)

	got, err := client.GetCompanyInfoByName(t.Context(), "citma")
	if got != nil {
		t.Errorf("GetCompanyInfoByName = %+v, want nil", got)
	}

	var companyNotFound ErrCompanyNotFound
	if !errors.As(err, &companyNotFound) {
		t.Fatalf("GetCompanyInfoByName error = %v, want ErrCompanyNotFound", err)
	}
	if companyNotFound.Query != "citma" {
		t.Errorf("ErrCompanyNotFound.Query = %q, want %q", companyNotFound.Query, "citma")
	}
}

// TestGetCompanyInfoByNameReportsAPageThatDiedHalfway covers the response that starts
// arriving and stops: a status and a Content-Length that promised more than the server sent.
// The body is streamed straight into the parser, so this surfaces as a read failure and not as
// a page that parsed into nothing.
func TestGetCompanyInfoByNameReportsAPageThatDiedHalfway(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: "<html><body><div class=\"block-list\">", Truncate: true},
	})
	client := newClientForTestServer(t, server)

	got, err := client.GetCompanyInfoByName(t.Context(), "citma")
	if got != nil {
		t.Errorf("GetCompanyInfoByName = %+v, want nil", got)
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("GetCompanyInfoByName error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

// TestLookupsAbortOnAContextCancelledUpFront is the simplest shape of the contract the context
// parameter exists for. A caller that has already given up must not become a request at all,
// and must be able to tell cancellation apart from every other failure — which is why the
// error has to wrap [context.Canceled] rather than merely mention it.
func TestLookupsAbortOnAContextCancelledUpFront(t *testing.T) {
	cases := []struct {
		name   string
		lookup func(ctx context.Context, client *Client) error
	}{
		{
			name: "by ID",
			lookup: func(ctx context.Context, client *Client) error {
				_, err := client.GetCompanyInfoByID(ctx, "302764291")
				return err
			},
		},
		{
			name: "by name",
			lookup: func(ctx context.Context, client *Client) error {
				_, err := client.GetCompanyInfoByName(ctx, "citma")
				return err
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := newRegistryTestServer(t, registryTestPages{
				registryLookupPath: {Body: resultsPageWith(1, citmaCompanyPath)},
				citmaCompanyPath:   {Body: detailPageFor(citmaCompanyInfo)},
			})
			client := newClientForTestServer(t, server)

			cancelledContext, cancel := context.WithCancel(t.Context())
			cancel()

			err := testCase.lookup(cancelledContext, client)

			if !errors.Is(err, context.Canceled) {
				t.Fatalf("lookup error = %v, want one wrapping context.Canceled", err)
			}
			if server.RequestCount() != 0 {
				t.Errorf("server saw %d requests, want 0 — a cancelled caller must not be sent",
					server.RequestCount())
			}
		})
	}
}

// TestGetCompanyInfoByNameCancelsEveryDetailPageInFlight is the fan-out half of cancellation,
// and the reason the context is threaded down to the request rather than only checked between
// the two rounds. All three detail pages are held open at once; one cancellation has to end all
// three and return, rather than leaving the goroutines to run out the client's own timeout.
func TestGetCompanyInfoByNameCancelsEveryDetailPageInFlight(t *testing.T) {
	const detailPageCount = 3

	heldOpen := make(chan struct{})
	defer close(heldOpen)

	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: resultsPageWith(detailPageCount, citmaCompanyPath, ventusCompanyPath, orlenCompanyPath)},
		citmaCompanyPath:   {Body: detailPageFor(citmaCompanyInfo), BlockUntil: heldOpen},
		ventusCompanyPath:  {Body: detailPageFor(ventusCompanyInfo), BlockUntil: heldOpen},
		orlenCompanyPath:   {Body: detailPageFor(orlenCompanyInfo), BlockUntil: heldOpen},
	})
	client := newClientForTestServer(t, server)

	cancellableContext, cancel := context.WithCancel(t.Context())

	// The lookup runs on its own goroutine so this one stays free to wait for the requests and
	// then cancel them; every assertion still happens here, where a failure is legal.
	lookupErrors := make(chan error, 1)
	go func() {
		_, err := client.GetCompanyInfoByName(cancellableContext, "citma")
		lookupErrors <- err
	}()

	server.AwaitRequests(t, 1+detailPageCount)
	cancel()

	select {
	case err := <-lookupErrors:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GetCompanyInfoByName error = %v, want one wrapping context.Canceled", err)
		}
	case <-time.After(awaitRequestsTimeout):
		t.Fatal("GetCompanyInfoByName did not return after its context was cancelled")
	}

	for _, companyPath := range []string{citmaCompanyPath, ventusCompanyPath, orlenCompanyPath} {
		if server.RequestCountFor(companyPath) != 1 {
			t.Errorf("detail page %q requested %d times, want 1 — all three must have been in flight",
				companyPath, server.RequestCountFor(companyPath))
		}
	}
}

// The companies the synthetic detail pages describe. They are deliberately different from one
// another, so a test asserting on results-page order proves something: three identical
// companies would come back in the right order however badly the code shuffled them.
var (
	citmaCompanyInfo = CompanyInfo{
		CompanyName:    "UAB Citma",
		CompanyLogo:    "/logos/citma-14580-139.jpg",
		CompanyAddress: "Kirtimų g. 51, LT-02244 Vilnius",
		CompanyID:      "123456789",
		CompanyVatID:   "LT234567890",
	}
	ventusCompanyInfo = CompanyInfo{
		CompanyName:    "UAB CITMA ventus",
		CompanyAddress: "Metalo g. 2A, LT-02190 Vilnius",
		CompanyID:      "987654321",
	}
	orlenCompanyInfo = CompanyInfo{
		CompanyName:    goldenCompanyName,
		CompanyLogo:    "/logos/orlen_apsauga_142511_021.jpg",
		CompanyAddress: "Mažeikių g. 75, Juodeikių k., LT-89467 Mažeikių r.",
		CompanyID:      "302764291",
		CompanyVatID:   "LT100006806712",
	}
)

// companyInfoServedFrom is what a company looks like once it has been fetched from a server:
// the logo URL a test writes root-relative comes back resolved against the base the client was
// pointed at. An absolute logo URL — which is what the saved fixture carries — is left alone,
// which is what resolution does to it.
func companyInfoServedFrom(server *registryTestServer, companyInfo CompanyInfo) CompanyInfo {
	if strings.HasPrefix(companyInfo.CompanyLogo, "/") {
		companyInfo.CompanyLogo = server.URL() + companyInfo.CompanyLogo
	}

	return companyInfo
}

// detailPageFor renders a company as the detail page the registry would serve for it.
func detailPageFor(companyInfo CompanyInfo) string {
	return companyInfoPageWith(companyInfoPageFields{
		Name:    companyInfo.CompanyName,
		LogoSrc: companyInfo.CompanyLogo,
		Address: companyInfo.CompanyAddress,
		ID:      companyInfo.CompanyID,
		VatID:   companyInfo.CompanyVatID,
	})
}

// resultsPageWith builds a lookup results page listing one company per path, in the order
// given, above a breadcrumb stating how many companies the search matched. A statedCount of
// zero leaves the breadcrumb out altogether, which is how a page that states no count arrives
// — the caller then falls back to counting the links.
//
// The paths are written into the page as hrefs and are root-relative, so they resolve against
// whichever server the client was pointed at.
func resultsPageWith(statedCount int, companyPaths ...string) string {
	var page strings.Builder

	page.WriteString(`<html><body>`)
	if statedCount > 0 {
		fmt.Fprintf(&page, `<div class="breadcrumb"><div>Rasta įmonių: <strong>%d</strong></div></div>`, statedCount)
	}
	page.WriteString(`<div class="block-list p-0"><div class="list-item">`)
	for _, companyPath := range companyPaths {
		fmt.Fprintf(&page,
			`<div class="company"><div class="company-info"><a class="company-title" href="%s">company</a></div></div>`,
			companyPath)
	}
	page.WriteString(`</div></div></body></html>`)

	return page.String()
}
