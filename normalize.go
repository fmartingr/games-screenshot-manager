package gamesscreenshotmanager

import "golang.org/x/text/unicode/norm"

// normalizeName returns the NFC form of a name read from disk. A name can reach
// the gallery in NFD form, and a web server matches the raw bytes of a URL, so
// every generated link and every visible title must use one form.
func normalizeName(name string) string {
	return norm.NFC.String(name)
}
