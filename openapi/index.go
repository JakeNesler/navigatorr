package openapi

import (
	"fmt"
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
	return results
}

// GetDetail returns full details for a specific endpoint.
func (idx *Index) GetDetail(path, method string) (*EndpointDetail, error) {
	methods, ok := idx.Endpoints[path]
	if !ok {
		// Try prefix match
		for p, m := range idx.Endpoints {
			if strings.HasSuffix(p, path) || strings.HasPrefix(p, path) {
				methods = m
				path = p
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("endpoint %s not found", path)
		}
	}

	detail, ok := methods[method]
	if !ok {
		// Return first available method
		for _, d := range methods {
			return d, nil
		}
		return nil, fmt.Errorf("method %s not found for %s", method, path)
	}

	return detail, nil
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
	return results
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
