package openapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Parse loads an OpenAPI spec from raw bytes and builds an Index.
func Parse(ctx context.Context, service string, data []byte) (*Index, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("parsing OpenAPI spec for %s: %w", service, err)
	}

	// Validate (non-fatal — some specs have minor issues)
	if err := doc.Validate(ctx); err != nil {
		// Log but don't fail
		fmt.Printf("warning: spec validation for %s: %v\n", service, err)
	}

	return buildIndex(service, doc), nil
}

func buildIndex(service string, doc *openapi3.T) *Index {
	idx := &Index{
		Service:   service,
		Endpoints: make(map[string]map[string]*EndpointDetail),
	}

	for path, pathItem := range doc.Paths.Map() {
		for method, op := range pathItem.Operations() {
			if op == nil {
				continue
			}

			method = strings.ToUpper(method)

			detail := &EndpointDetail{
				Service:     service,
				Method:      method,
				Path:        path,
				Summary:     op.Summary,
				Description: op.Description,
				Tags:        op.Tags,
				Responses:   make(map[string]string),
			}

			// Parameters
			for _, pRef := range op.Parameters {
				if pRef.Value == nil {
					continue
				}
				p := pRef.Value
				pi := ParameterInfo{
					Name:        p.Name,
					In:          p.In,
					Required:    p.Required,
					Description: p.Description,
				}
				if p.Schema != nil && p.Schema.Value != nil {
					pi.Type = p.Schema.Value.Type.Slice()[0]
				}
				detail.Parameters = append(detail.Parameters, pi)
			}

			// Request body
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				for ct, mediaType := range op.RequestBody.Value.Content {
					si := &SchemaInfo{ContentType: ct}
					if mediaType.Schema != nil && mediaType.Schema.Value != nil {
						si.Properties = flattenSchema(mediaType.Schema.Value)
						si.Required = mediaType.Schema.Value.Required
					}
					detail.RequestBody = si
					break // Take first content type
				}
			}

			// Responses
			if op.Responses != nil {
				for code, respRef := range op.Responses.Map() {
					if respRef.Value != nil && respRef.Value.Description != nil {
						detail.Responses[code] = *respRef.Value.Description
					}
				}
			}

			if idx.Endpoints[path] == nil {
				idx.Endpoints[path] = make(map[string]*EndpointDetail)
			}
			idx.Endpoints[path][method] = detail
		}
	}

	return idx
}

// flattenSchema extracts property names and types from a schema.
func flattenSchema(schema *openapi3.Schema) map[string]any {
	if schema == nil || len(schema.Properties) == 0 {
		return nil
	}

	props := make(map[string]any)
	for name, propRef := range schema.Properties {
		if propRef.Value == nil {
			props[name] = "unknown"
			continue
		}
		p := propRef.Value
		types := p.Type.Slice()
		t := "unknown"
		if len(types) > 0 {
			t = types[0]
		}
		if p.Description != "" {
			props[name] = map[string]string{"type": t, "description": p.Description}
		} else {
			props[name] = t
		}
	}
	return props
}
