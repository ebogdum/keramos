package repo

import (
	"net/url"
	"strings"

	"github.com/ebogdum/keramos/v2/internal/plugin"
)

// TryDownloaderFetchPublic exposes the plugin-downloader dispatch to callers
// outside the repo package (notably `keramos pull`).
func TryDownloaderFetchPublic(rawURL string) ([]byte, bool, error) {
	return tryDownloaderFetch(rawURL)
}

// tryDownloaderFetch checks whether a downloader plugin handles the URL's
// scheme and, if so, invokes it. Returns (data, true, err) on plugin handling;
// (nil, false, nil) when no plugin claims the scheme, leaving the caller to
// fall back to HTTP. If the URL belongs to a configured repository, that
// repo's TLS material (caFile/certFile/keyFile) is forwarded to the plugin.
func tryDownloaderFetch(rawURL string) ([]byte, bool, error) {
	u, parseErr := url.Parse(rawURL)
	if nil != parseErr {
		return nil, false, nil
	}
	scheme := strings.ToLower(u.Scheme)
	if "http" == scheme || "https" == scheme || "" == scheme {
		return nil, false, nil
	}
	if _, _, found := plugin.FindDownloader(rawURL); !found {
		return nil, false, nil
	}
	cert, key, ca := tlsForURL(rawURL)
	data, err := plugin.RunDownloader(rawURL, cert, key, ca)
	return data, true, err
}

// tlsForURL looks up TLS material declared on the configured repository whose
// URL is a prefix of rawURL. Empty strings mean "no material" (the downloader
// is expected to use system defaults).
func tlsForURL(rawURL string) (cert, key, ca string) {
	rf, err := LoadRepoFile()
	if nil != err {
		return "", "", ""
	}
	for _, r := range rf.Repositories {
		if strings.HasPrefix(rawURL, r.URL) {
			return r.CertFile, r.KeyFile, r.CAFile
		}
	}
	return "", "", ""
}
