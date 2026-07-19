package render

import (
	"net/url"
	"path/filepath"
	"strings"
)

// hyperlink wraps text in an OSC 8 hyperlink (mirrors utils/hyperlinks.ts).
func hyperlink(uri, text string) string {
	return "\x1b]8;;" + uri + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// getFileHref converts a filesystem path to a file:// URL (mirrors getFileHref).
func getFileHref(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	slashed := filepath.ToSlash(abs)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed // Windows drive paths: /D:/...
	}
	u := &url.URL{Scheme: "file", Path: slashed}
	return u.String()
}

// safeHyperlink wraps text only for file:/https: URIs (mirrors safeHyperlink).
func safeHyperlink(uri, text string) string {
	if uri == "" {
		return text
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return text
	}
	if parsed.Scheme != "https" && parsed.Scheme != "file" {
		return text
	}
	return hyperlink(uri, text)
}
