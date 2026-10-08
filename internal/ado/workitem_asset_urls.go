package ado

import (
	"net/url"
	"strings"
)

// resolveInlineAssetURL deliberately accepts only WIT attachment endpoints.
// The configured base, never a URL supplied by remote content, defines trust.
func resolveInlineAssetURL(raw, baseURL, project string) (resolved, identity, reason string) {
	base, err := url.Parse(baseURL)
	if err != nil || base.Host == "" || base.User != nil || (base.Scheme != "http" && base.Scheme != "https") {
		return "", "", "missing or invalid configured base URL"
	}
	reference, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || strings.TrimSpace(raw) == "" {
		return "", "", "empty or invalid image URL"
	}
	if reference.User != nil || unsafeAssetPath(reference) {
		return "", "", "unsafe image URL"
	}
	contextURL := *base
	contextURL.Path = strings.TrimRight(base.Path, "/") + "/" + project + "/"
	contextURL.RawPath, contextURL.RawQuery, contextURL.Fragment = "", "", ""
	u := contextURL.ResolveReference(reference)
	if u.Scheme != base.Scheme || u.Host != base.Host {
		return "", "", "image URL is outside the configured origin"
	}
	prefix := strings.TrimRight(base.Path, "/") + "/"
	if !strings.HasPrefix(u.Path, prefix) {
		return "", "", "image URL is outside the configured organization or collection"
	}
	segments := strings.Split(strings.TrimPrefix(u.Path, prefix), "/")
	if len(segments) == 5 && segments[0] != "" {
		segments = segments[1:] // Optional project segment within the configured collection.
	}
	if len(segments) != 4 || !strings.EqualFold(segments[0], "_apis") || !strings.EqualFold(segments[1], "wit") || !strings.EqualFold(segments[2], "attachments") || segments[3] == "" {
		return "", "", "image URL is not a work-item attachment endpoint"
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", "", "invalid image URL query"
	}
	u.Fragment, u.RawFragment = "", ""
	resolved = u.String()
	for key := range query {
		switch strings.ToLower(key) {
		case "filename", "download", "api-version":
			delete(query, key)
		}
	}
	keyURL := *base
	keyURL.Path = prefix + "_apis/wit/attachments/" + segments[3]
	keyURL.RawPath, keyURL.Fragment, keyURL.RawFragment = "", "", ""
	keyURL.RawQuery = query.Encode()
	return resolved, keyURL.String(), ""
}

func unsafeAssetPath(u *url.URL) bool {
	encoded := strings.ToLower(u.EscapedPath())
	if strings.ContainsAny(u.Path, "\\\x00\r\n") || strings.Contains(encoded, "%2f") || strings.Contains(encoded, "%5c") || strings.Contains(encoded, "%25") {
		return true
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return true
		}
	}
	return false
}
