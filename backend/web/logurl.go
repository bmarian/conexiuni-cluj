package web

import "strings"

// Plans and place searches name where people are going.
func LogURL(url string) string {
	path, query, ok := strings.Cut(url, "?")
	if !ok {
		return url
	}
	trimmed := strings.TrimSuffix(path, "/")
	last := trimmed[strings.LastIndex(trimmed, "/")+1:]
	if !strings.EqualFold(last, "plan") && !strings.EqualFold(last, "places") {
		return url
	}
	params := strings.Split(query, "&")
	for i, p := range params {
		name, _, _ := strings.Cut(p, "=")
		params[i] = name + "=-"
	}
	return path + "?" + strings.Join(params, "&")
}
