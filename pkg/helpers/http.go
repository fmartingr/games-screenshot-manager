package helpers

import (
	"net/http"
	"net/url"
)

func DoRequest(method string, requestURL string) (*http.Response, error) {
	parsedURL, _ := url.Parse(requestURL)
	request := http.Request{
		Method: method,
		URL:    parsedURL,
		Header: map[string][]string{
			"User-Agent": {"github.com/fmartingr/games-screenshot-manager"},
		},
		ProtoMajor: 2,
		ProtoMinor: 1,
	}
	return http.DefaultClient.Do(&request)
}
