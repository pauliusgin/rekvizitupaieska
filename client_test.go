package rekvizitupaieska

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// registryBase is the live registry's base URL. The parsers and the URL builders take a base
// URL so a client can point them at a test server; the tests that check them against the saved
// pages and the pinned URL strings pass this one, which is what those expectations are written
// against.
var registryBase = &url.URL{Scheme: registryScheme, Host: registryHost}

func TestNewClientAppliesTheDocumentedDefaults(t *testing.T) {
	client := NewClient(ClientParams{})

	if client.baseURL.String() != registryBaseURL {
		t.Errorf("baseURL = %q, want %q", client.baseURL, registryBaseURL)
	}
	if client.userAgent != userAgent {
		t.Errorf("userAgent = %q, want %q", client.userAgent, userAgent)
	}
	if client.maxMatches != maxMatches {
		t.Errorf("maxMatches = %d, want %d", client.maxMatches, maxMatches)
	}
	if client.httpClient == nil {
		t.Fatal("httpClient is nil, want a client with a timeout")
	}
	if client.httpClient.Timeout != requestTimeout {
		t.Errorf("httpClient.Timeout = %s, want %s", client.httpClient.Timeout, requestTimeout)
	}
}

func TestNewClientKeepsTheGivenHTTPClient(t *testing.T) {
	httpClient := &http.Client{Timeout: time.Second}

	client := NewClient(ClientParams{HTTPClient: httpClient})

	if client.httpClient != httpClient {
		t.Errorf("httpClient = %p, want the one given, %p", client.httpClient, httpClient)
	}
}

// TestClientSendsTheUserAgent checks the header actually reaches the server, on both the
// lookup request and the detail request. A public site is entitled to know who is calling, and
// a User-Agent that is set on the request but never sent would be invisible from the returned
// values alone.
func TestClientSendsTheUserAgent(t *testing.T) {
	server := newRegistryTestServer(t, registryTestPages{
		registryLookupPath: {Body: resultsPageWith(0, orlenCompanyPath)},
		orlenCompanyPath:   {FixturePath: companyInfoFixturePath},
	})
	client := newClientForTestServer(t, server)

	if _, err := client.GetCompanyInfoByID(t.Context(), goldenCompanyInfo.CompanyID); err != nil {
		t.Fatalf("GetCompanyInfoByID returned error %v", err)
	}

	sentUserAgents := server.UserAgents()
	if len(sentUserAgents) != 2 {
		t.Fatalf("server saw %d requests, want 2", len(sentUserAgents))
	}
	for index, sentUserAgent := range sentUserAgents {
		if sentUserAgent != userAgent {
			t.Errorf("request %d User-Agent = %q, want %q", index, sentUserAgent, userAgent)
		}
	}
}

// TestFetchRegistryDocumentReportsAnUnbuildableRequest covers the branch no server can reach:
// a URL that [net/http.NewRequest] itself refuses. It is called directly because a client's
// base URL can never produce one — it is built from the registry's own scheme and host.
func TestFetchRegistryDocumentReportsAnUnbuildableRequest(t *testing.T) {
	client := NewClient(ClientParams{})

	document, err := client.fetchRegistryDocument(t.Context(), "://not a url")
	if document != nil {
		t.Errorf("fetchRegistryDocument returned a document, want nil")
	}
	if err == nil {
		t.Fatal("fetchRegistryDocument returned no error, want one")
	}
	if !strings.Contains(err.Error(), "://not a url") {
		t.Errorf("error %q does not name the URL it failed on", err)
	}
}

// registryTestPage is what the test server answers with for one path. The zero value is a 200
// with an empty body.
type registryTestPage struct {
	// Body is the page to serve. It is ignored when FixturePath is set.
	Body string

	// FixturePath is a saved registry page under testdata/ to serve verbatim.
	FixturePath string

	// StatusCode is the status to answer with. Zero means 200.
	StatusCode int

	// HangUp closes the connection without answering at all, which is the transport-level
	// failure — no status, no body, no page.
	HangUp bool

	// Truncate answers with a Content-Length larger than Body and then drops the connection,
	// which is the page that starts arriving and stops. Unlike HangUp this one reaches the
	// client as a successful status whose body then fails to read.
	Truncate bool

	// BlockUntil holds the response back until the channel is closed, which is how a test gets
	// hold of a request that is still in flight — the only state a cancellation can act on. A
	// nil channel, which is the zero value, answers straight away.
	BlockUntil <-chan struct{}
}

// registryTestPages maps a request path to the page the server answers with. A path that is
// not in the map is answered as a 404 — both because that is what the registry gives for an
// address that no longer exists, and so that a test which gets a path wrong fails loudly
// instead of quietly parsing an empty page.
type registryTestPages map[string]registryTestPage

// registryTestServer is a stand-in for the registry. It serves the pages a test registered and
// records every request it received, so a test can assert on what was *not* fetched — which is
// the whole point of the too-many-matches gate and is invisible from the return value alone.
type registryTestServer struct {
	server *httptest.Server

	// arrivals carries one value per request received, so a test can wait for a request to
	// arrive instead of polling the counters or sleeping. It is buffered and written to
	// without blocking, so a test that never reads it costs the handler nothing.
	arrivals chan string

	mutex           sync.Mutex
	requestedPaths  []string
	sentUserAgents  []string
	requestedByPath map[string]int
}

// arrivalBufferSize is larger than any one test's request count, so an unread arrivals channel
// never drops the value a later test would have waited for.
const arrivalBufferSize = 64

// awaitRequestsTimeout bounds the wait for requests that should already be on their way. It is
// a failure deadline and not a delay: a passing test never reaches it.
const awaitRequestsTimeout = 5 * time.Second

// newClientForTestServer returns a client that talks to a test server instead of the live
// registry. The base URL is not a [ClientParams] field — a caller has no business pointing this
// library at another host — so the seam is the unexported field, which only this package can
// reach.
func newClientForTestServer(t *testing.T, server *registryTestServer) *Client {
	t.Helper()

	client := NewClient(ClientParams{})
	client.baseURL = mustParseURL(t, server.URL())

	return client
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) returned error %v", rawURL, err)
	}

	return parsedURL
}

func newRegistryTestServer(t *testing.T, pages registryTestPages) *registryTestServer {
	t.Helper()

	servedPages := resolveFixtureBodies(t, pages)

	registryServer := &registryTestServer{
		arrivals:        make(chan string, arrivalBufferSize),
		requestedByPath: map[string]int{},
	}
	registryServer.server = httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			registryServer.record(request)

			page, registered := servedPages[request.URL.Path]
			if !registered {
				http.Error(writer, "no such page", http.StatusNotFound)
				return
			}
			serveRegistryTestPage(t, writer, request, page)
		}))
	t.Cleanup(registryServer.server.Close)

	return registryServer
}

// resolveFixtureBodies reads every page's fixture up front, on the test's own goroutine.
//
// A handler runs on the server's goroutine, where [testing.T.Fatalf] would stop the wrong one
// and leave a missing fixture to surface as a puzzling response rather than as a failed test.
// Reading each fixture once instead of once per request also keeps 300 KB of file off the
// path every request takes.
func resolveFixtureBodies(t *testing.T, pages registryTestPages) registryTestPages {
	t.Helper()

	resolvedPages := make(registryTestPages, len(pages))
	for path, page := range pages {
		if page.FixturePath != "" {
			page.Body = readFixture(t, page.FixturePath)
		}
		resolvedPages[path] = page
	}

	return resolvedPages
}

func (s *registryTestServer) record(request *http.Request) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.requestedPaths = append(s.requestedPaths, request.URL.Path)
	s.sentUserAgents = append(s.sentUserAgents, request.Header.Get(userAgentHeader))
	s.requestedByPath[request.URL.Path]++

	select {
	case s.arrivals <- request.URL.Path:
	default:
	}
}

// AwaitRequests blocks until requestCount requests have been received, so a test can act on
// requests that are still in flight without polling a counter or sleeping for a guess.
//
// It must be called from the test's own goroutine, because it fails the test on a timeout.
func (s *registryTestServer) AwaitRequests(t *testing.T, requestCount int) {
	t.Helper()

	for range requestCount {
		select {
		case <-s.arrivals:
		case <-time.After(awaitRequestsTimeout):
			t.Fatalf("timed out waiting for %d requests, server saw %d", requestCount, s.RequestCount())
		}
	}
}

// URL is the base URL a client under test is pointed at.
func (s *registryTestServer) URL() string {
	return s.server.URL
}

// RequestCount is how many requests the server received in total.
func (s *registryTestServer) RequestCount() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return len(s.requestedPaths)
}

// RequestCountFor is how many requests the server received for one path.
func (s *registryTestServer) RequestCountFor(path string) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.requestedByPath[path]
}

// UserAgents are the User-Agent headers of every request, in the order they arrived.
func (s *registryTestServer) UserAgents() []string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return append([]string(nil), s.sentUserAgents...)
}

// serveRegistryTestPage writes one registered page.
func serveRegistryTestPage(t *testing.T, writer http.ResponseWriter, request *http.Request, page registryTestPage) {
	t.Helper()

	if !awaitPageRelease(request, page.BlockUntil) {
		return
	}
	if page.HangUp {
		hangUpOn(t, writer)
		return
	}
	if page.Truncate {
		truncateOn(t, writer, page.Body)
		return
	}
	if page.StatusCode != 0 {
		writer.WriteHeader(page.StatusCode)
	}
	fmt.Fprint(writer, page.Body)
}

// awaitPageRelease holds a response back until the test releases it, reporting whether it
// should still be written. A request whose context is cancelled while it waits is abandoned
// unanswered — which is exactly what a server sees when the caller has given up, and is what
// lets a test cancel a request that is genuinely in flight rather than one already answered.
func awaitPageRelease(request *http.Request, release <-chan struct{}) bool {
	if release == nil {
		return true
	}

	select {
	case <-release:
		return true
	case <-request.Context().Done():
		return false
	}
}

// hangUpOn drops the connection without writing a response, which reaches the client as a
// transport-level failure rather than as a status code.
func hangUpOn(t *testing.T, writer http.ResponseWriter) {
	t.Helper()

	connection, _, hijacked := hijackConnection(t, writer)
	if !hijacked {
		return
	}
	connection.Close()
}

// truncateOn writes a response announcing more bytes than it then sends, and drops the
// connection. The client reads the status and starts on the body, which ends early — an
// [io.ErrUnexpectedEOF] rather than an error from the request itself.
//
// The response is written by hand because [net/http.ResponseWriter] will not send a
// Content-Length it knows to be a lie.
func truncateOn(t *testing.T, writer http.ResponseWriter, body string) {
	t.Helper()

	connection, buffered, hijacked := hijackConnection(t, writer)
	if !hijacked {
		return
	}
	defer connection.Close()

	fmt.Fprintf(buffered, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: %d\r\n\r\n%s",
		len(body)+truncatedByteCount, body)
	if err := buffered.Flush(); err != nil {
		t.Errorf("flushing the truncated response returned error %v", err)
	}
}

// truncatedByteCount is how many bytes a truncated response promises beyond what it sends.
const truncatedByteCount = 1024

// hijackConnection takes the raw connection out from under the HTTP server, so a handler can
// answer with something [net/http.ResponseWriter] would refuse to write, or with nothing at
// all. It reports whether it got the connection.
//
// Both failures are marked with [testing.T.Errorf] and not [testing.T.Fatalf]: this runs on
// the server's goroutine, and Fatalf there would stop that goroutine rather than the test.
// Neither is reachable from a test — httptest's HTTP/1 responses are always hijackable — but
// a handler that silently did nothing would be a puzzling way to find that out.
func hijackConnection(t *testing.T, writer http.ResponseWriter) (net.Conn, *bufio.ReadWriter, bool) {
	t.Helper()

	hijacker, canHijack := writer.(http.Hijacker)
	if !canHijack {
		t.Errorf("test server response writer cannot be hijacked, cannot drop a connection")
		return nil, nil, false
	}

	connection, buffered, err := hijacker.Hijack()
	if err != nil {
		t.Errorf("hijacking the connection returned error %v", err)
		return nil, nil, false
	}

	return connection, buffered, true
}

// readFixture reads one of the saved registry pages under testdata/ whole, for a test server
// to serve verbatim. The parser tests stream the same files instead; a handler has to hold the
// bytes, because it may be asked for them more than once.
func readFixture(t *testing.T, fixturePath string) string {
	t.Helper()

	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) returned error %v", fixturePath, err)
	}

	return string(fixture)
}
