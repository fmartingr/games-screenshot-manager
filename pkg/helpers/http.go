package helpers

import (
	"io/ioutil"
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

func DownloadURLIntoTempFile(requestURL string) (string, error) {
	response, err := DoRequest("GET", requestURL)
	if err != nil {
		return "", err
	}

	if response.Body != nil {
		defer response.Body.Close()
	}

	tmpfile, err := ioutil.TempFile("", "games-screenshot-manager")
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	tmpfile.Write(body)

	return tmpfile.Name(), nil
}
