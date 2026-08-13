// Package rekvizitupaieska looks Lithuanian companies up in the public business registry
// at https://rekvizitai.vz.lt and returns their published details.
//
// There are two entry points, both on [Client]. [Client.GetCompanyInfoByID] takes the
// nine-digit registration code, "Įmonės kodas", which is unique, and so returns exactly
// one [CompanyInfo]. [Client.GetCompanyInfoByName] takes a partial company name and
// returns a [CompanyInfo] for every company that matched.
//
//	client := rekvizitupaieska.NewClient(rekvizitupaieska.ClientParams{})
//	info, err := client.GetCompanyInfoByID(ctx, "302764291") // UAB "ORLEN Apsauga"
//
// # Cancellation
//
// Both entry points take a [context.Context] as their first argument, because both make
// requests to a public site that is not this library's to keep waiting on. Cancelling the
// context, or reaching its deadline, aborts the requests in flight and the call returns an
// error wrapping [context.Canceled] or [context.DeadlineExceeded].
//
// A server handler should pass its request's context straight through, so that a caller who
// disconnects stops the lookup instead of leaving it to scrape pages nobody is waiting for:
//
//	func handleLookup(writer http.ResponseWriter, request *http.Request) {
//		info, err := client.GetCompanyInfoByID(request.Context(), request.PathValue("id"))
//		// ...
//	}
//
// This is per call, and is the only way to bound one lookup differently from another: the
// timeout on [ClientParams.HTTPClient] is fixed when the client is built and applies to every
// request it makes. A search by name fans out to one request per matched company at once, and
// they all carry the one context, so a single cancellation stops the whole fan-out.
//
// # The client
//
// [NewClient] takes a [ClientParams] whose one field is optional, so the zero value is a
// working configuration and the call above needs nothing else. The field is there for the
// callers that do need it: [ClientParams.HTTPClient] supplies a transport, a proxy or a
// timeout other than the default 10 seconds. Everything else about a client is fixed — the
// registry it talks to, the User-Agent it names this library with, and the match gate
// described below.
//
// A [Client] is safe for concurrent use and worth keeping rather than rebuilding: it holds an
// [net/http.Client], which pools connections, and a search by name fetches its detail pages
// through it concurrently.
//
// # The result
//
// A [CompanyInfo] carries the company name, an absolute URL to its logo, its registered
// address, its registration code and its VAT payer code.
//
// The logo, the address and the VAT payer code are empty when the registry does not publish
// them — a company that is not registered for VAT has no VAT code, and one with no logo has
// none — and an empty field is a normal result, not an error. The name and the registration
// code are the two every company page carries, so a page missing either did not describe a
// company at all and comes back as an [ErrCompanyNotFound] rather than as a CompanyInfo of
// empty strings.
//
// # Searching by name
//
// The search by name is a fuzzy one, and the registry runs it on letters alone: every
// character that is not a letter is ignored, and the remaining groups of letters are what
// it matches on. A shorter term matches more companies than a longer one, and a term of
// several groups matches fewer than any one group alone: "orl" matches "Orlen",
// "Orlaivių aptarnavimo agentūra" and "Ramunės Orliukienės PĮ" alike, while "orl apt"
// matches only the second. Because non-letters are ignored, "orl apt", "apt orl" and
// "orl.-.-.-.-.apt" are one and the same search. How the registry matches a group is its
// own business and is not documented; the behavior described here is what it was observed
// to do.
//
// A search term must hold a run of at least three letters somewhere — "orl" and "orl a"
// qualify, "or la" does not. Without such a run the registry would match on single letters
// and return near enough everything, so the term is rejected before a request is made.
//
// # The match gate
//
// A search by name is two rounds of requests: one for the page of results, then one detail
// page per company it listed. The second round is what a vague term makes expensive, so there
// is a gate between them. A search that matched more than three companies returns an
// [ErrTooManyMatches] and fetches no detail page at all.
//
// This is caller-visible behavior and not an internal limit: the error carries the match count
// and the detail-page URLs, so a caller that wants them anyway can fetch them itself, and one
// that would rather narrow the term has the count to decide by.
//
// The count the gate trips on is the one the results page states for itself — "Rasta įmonių:
// 4" — because that is what the registry matched, while the links on the page are only what it
// chose to show. The two can disagree, and the gate follows the larger meaning. A page that
// states no count at all is counted by its links.
//
// # Errors
//
// Every error this package returns is a type rather than a sentinel value, so callers
// match them with [errors.As] and not with [errors.Is]. Each carries the value that made
// it fail:
//
//	info, err := client.GetCompanyInfoByID(ctx, "30276429")
//
//	var invalidCompanyID rekvizitupaieska.ErrInvalidCompanyID
//	if errors.As(err, &invalidCompanyID) {
//		log.Printf("bad code: %s", invalidCompanyID.CompanyID)
//	}
//
// A lookup that got no results is an [ErrCompanyNotFound] rather than an empty slice — as is a
// lookup the registry listed a match for whose page turned out to hold no company. Both carry
// the code or the search term that was looked up. A
// registry page that answered outside the 2xx range is an [ErrRegistryRequestFailed] carrying
// the status. A request that got no answer at all — a name that did not resolve, a refused
// connection, a timeout, a cancelled context — is not one of these types: it is wrapped, so
// [errors.Is] and [errors.As] reach the underlying error and a caller can still tell a timeout
// from a refusal, or either from a cancellation.
package rekvizitupaieska
