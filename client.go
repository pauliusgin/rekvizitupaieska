package rekvizitupaieska

import (
	"net/http"
	"net/url"
	"time"
)

const (
	registryScheme  = "https"
	registryHost    = "rekvizitai.vz.lt"
	registryBaseURL = registryScheme + "://" + registryHost

	requestTimeout = 10 * time.Second
	maxMatches     = 3
)

type ClientParams struct {
	HTTPClient *http.Client
}

type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	userAgent  string
	maxMatches int
}

func NewClient(params ClientParams) *Client {
	httpClient := params.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    &url.URL{Scheme: registryScheme, Host: registryHost},
		userAgent:  userAgent,
		maxMatches: maxMatches,
	}
}
