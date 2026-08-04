package openapi

import (
	"fmt"
	"sort"
	"strings"
)

// Index holds parsed endpoint data for a single service.
type Index struct {
	Service   string
	Endpoints map[string]map[string]*EndpointDetail // path -> method -> detail
}

// Count returns the total number of endpoints.
func (idx *Index) Count() int {
	n := 0
	for _, methods := range idx.Endpoints {
		n += len(methods)
	}
	return n
}

// Filter returns endpoint summaries matching optional tag and method filters.
func (idx *Index) Filter(tag, method string) []EndpointSummary {
	var results []EndpointSummary
	tag = strings.ToLower(tag)
	method = strings.ToUpper(method)

	for path, methods := range idx.Endpoints {
		for m, detail := range methods {
			if method != "" && m != method {
				continue
			}
			if tag != "" {
				matched := false
				for _, t := range detail.Tags {
					if strings.EqualFold(t, tag) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			t := ""
			if len(detail.Tags) > 0 {
				t = detail.Tags[0]
			}
			results = append(results, EndpointSummary{
				Service: idx.Service,
				Method:  m,
				Path:    path,
				Summary: detail.Summary,
				Tag:     t,
			})
		}
	}
	return sortSummaries(results)
}

// GetDetail returns full details for a specific endpoint.
// Falls back to the closest matching path, preferring a prefix match over a
// suffix one and the shortest candidate within each. Map iteration order is
// random, so the fallback must sort or the same query returns a different
// endpoint on every call.
func (idx *Index) GetDetail(path, method string) (*EndpointDetail, error) {
	methods, ok := idx.Endpoints[path]
	if !ok {
		var prefixed, suffixed []string
		for p := range idx.Endpoints {
			switch {
			case strings.HasPrefix(p, path):
				prefixed = append(prefixed, p)
			case strings.HasSuffix(p, path):
				suffixed = append(suffixed, p)
			}
		}
		candidates := prefixed
		if len(candidates) == 0 {
			candidates = suffixed
		}
		if len(candidates) == 0 {
			return nil, fmt.Errorf("endpoint %s not found", path)
		}
		sort.Slice(candidates, func(i, j int) bool {
			if len(candidates[i]) != len(candidates[j]) {
				return len(candidates[i]) < len(candidates[j])
			}
			return candidates[i] < candidates[j]
		})
		path = candidates[0]
		methods = idx.Endpoints[path]
	}

	if detail, ok := methods[method]; ok {
		return detail, nil
	}

	// Fall back to the first method by name, so repeated calls agree.
	names := make([]string, 0, len(methods))
	for m := range methods {
		names = append(names, m)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("method %s not found for %s", method, path)
	}
	sort.Strings(names)
	return methods[names[0]], nil
}

// sortSummaries orders results by path then method so tool output is stable
// across calls.
func sortSummaries(s []EndpointSummary) []EndpointSummary {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Service != s[j].Service {
			return s[i].Service < s[j].Service
		}
		if s[i].Path != s[j].Path {
			return s[i].Path < s[j].Path
		}
		return s[i].Method < s[j].Method
	})
	return s
}

// Search searches across the index for matching endpoints.
func (idx *Index) Search(query string) []EndpointSummary {
	query = strings.ToLower(query)
	var results []EndpointSummary

	for path, methods := range idx.Endpoints {
		for m, detail := range methods {
			if matches(query, path, detail) {
				t := ""
				if len(detail.Tags) > 0 {
					t = detail.Tags[0]
				}
				results = append(results, EndpointSummary{
					Service: idx.Service,
					Method:  m,
					Path:    path,
					Summary: detail.Summary,
					Tag:     t,
				})
			}
		}
	}
	return sortSummaries(results)
}

func matches(query, path string, detail *EndpointDetail) bool {
	if strings.Contains(strings.ToLower(path), query) {
		return true
	}
	if strings.Contains(strings.ToLower(detail.Summary), query) {
		return true
	}
	if strings.Contains(strings.ToLower(detail.Description), query) {
		return true
	}
	for _, tag := range detail.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
