package openapi

// EndpointSummary is a compact listing entry.
type EndpointSummary struct {
	Service string `json:"service"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Summary string `json:"summary"`
	Tag     string `json:"tag"`
}

// EndpointDetail is the full detail for a specific endpoint.
type EndpointDetail struct {
	Service     string            `json:"service"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Summary     string            `json:"summary,omitempty"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Parameters  []ParameterInfo   `json:"parameters,omitempty"`
	RequestBody *SchemaInfo       `json:"request_body,omitempty"`
	Responses   map[string]string `json:"responses,omitempty"`
}

// ParameterInfo describes a single parameter.
type ParameterInfo struct {
	Name        string `json:"name"`
	In          string `json:"in"` // query, path, header
	Required    bool   `json:"required"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// SchemaInfo is a simplified schema representation.
type SchemaInfo struct {
	ContentType string         `json:"content_type"`
	Properties  map[string]any `json:"properties,omitempty"`
	Required    []string       `json:"required,omitempty"`
	Example     any            `json:"example,omitempty"`
}
