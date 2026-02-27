package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jakenesler/navigatorr/arrservice"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerAPICallTool(s *server.MCPServer, registry *arrservice.Registry) {
	s.AddTool(
		mcp.NewTool("call_api",
			mcp.WithDescription("Make an authenticated API call to any configured *arr service. Returns the JSON response. Use fields/limit/filter to reduce response size."),
			mcp.WithString("service", mcp.Required(), mcp.Description("Service name (e.g. sonarr, radarr)")),
			mcp.WithString("method", mcp.Description("HTTP method (default: GET)")),
			mcp.WithString("path", mcp.Required(), mcp.Description("API path (e.g. /series, /movie). The API version prefix is added automatically.")),
			mcp.WithString("query", mcp.Description("Query parameters as JSON object (e.g. {\"term\": \"breaking bad\"})")),
			mcp.WithString("body", mcp.Description("Request body as JSON string")),
			mcp.WithString("fields", mcp.Description("Comma-separated fields to include in response. Supports nested fields with dot notation (e.g. \"id,title,statistics.sizeOnDisk\")")),
			mcp.WithString("filter", mcp.Description("Filter array results. Format: \"field:op:value\". Ops: contains, eq, ne, gt, lt (e.g. \"title:contains:Pirates\", \"year:gt:2000\", \"hasFile:eq:true\")")),
			mcp.WithString("limit", mcp.Description("Max number of items to return from array responses")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handleCallAPI(ctx, req, registry)
		},
	)
}

func handleCallAPI(ctx context.Context, req mcp.CallToolRequest, registry *arrservice.Registry) (*mcp.CallToolResult, error) {
	svcName := mcp.ParseString(req, "service", "")
	method := strings.ToUpper(mcp.ParseString(req, "method", "GET"))
	path := mcp.ParseString(req, "path", "")
	queryStr := mcp.ParseString(req, "query", "")
	bodyStr := mcp.ParseString(req, "body", "")
	fieldsStr := mcp.ParseString(req, "fields", "")
	filterStr := mcp.ParseString(req, "filter", "")
	limitStr := mcp.ParseString(req, "limit", "")

	if svcName == "" || path == "" {
		return mcp.NewToolResultError("service and path are required"), nil
	}

	svc, err := registry.Get(svcName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Parse query params
	var query map[string]string
	if queryStr != "" {
		var raw map[string]any
		if err := json.Unmarshal([]byte(queryStr), &raw); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid query JSON: %v", err)), nil
		}
		query = make(map[string]string)
		for k, v := range raw {
			query[k] = fmt.Sprintf("%v", v)
		}
	}

	// Parse body
	var body []byte
	if bodyStr != "" {
		if !json.Valid([]byte(bodyStr)) {
			return mcp.NewToolResultError("invalid body JSON"), nil
		}
		body = []byte(bodyStr)
	}

	respBody, statusCode, err := svc.DoRequest(ctx, method, path, query, body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("request failed: %v", err)), nil
	}

	// Parse response JSON
	var jsonResp any
	if err := json.Unmarshal(respBody, &jsonResp); err != nil {
		// Not JSON, return raw
		return mcp.NewToolResultText(fmt.Sprintf("status: %d\n%s", statusCode, string(respBody))), nil
	}

	// Apply filter, fields, limit to array responses
	needsProcessing := fieldsStr != "" || filterStr != "" || limitStr != ""
	if needsProcessing {
		jsonResp = processResponse(jsonResp, fieldsStr, filterStr, limitStr)
	}

	data, _ := json.MarshalIndent(jsonResp, "", "  ")
	return mcp.NewToolResultText(string(data)), nil
}

// processResponse applies filter, fields, and limit to the API response.
func processResponse(resp any, fieldsStr, filterStr, limitStr string) any {
	// If response is an array, apply filter and limit
	arr, isArray := resp.([]any)
	if !isArray {
		// Single object — just apply fields
		if obj, ok := resp.(map[string]any); ok && fieldsStr != "" {
			return pickFields(obj, parseFields(fieldsStr))
		}
		return resp
	}

	// Apply filter
	if filterStr != "" {
		arr = applyFilter(arr, filterStr)
	}

	// Apply limit
	if limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(arr) {
			arr = arr[:limit]
		}
	}

	// Apply field selection
	if fieldsStr != "" {
		fields := parseFields(fieldsStr)
		result := make([]any, len(arr))
		for i, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				result[i] = pickFields(obj, fields)
			} else {
				result[i] = item
			}
		}
		return result
	}

	return arr
}

// parseFields splits a comma-separated fields string.
func parseFields(s string) []string {
	parts := strings.Split(s, ",")
	fields := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			fields = append(fields, p)
		}
	}
	return fields
}

// pickFields extracts only the specified fields from an object.
// Supports dot notation for nested fields (e.g. "statistics.sizeOnDisk").
func pickFields(obj map[string]any, fields []string) map[string]any {
	result := make(map[string]any)
	for _, f := range fields {
		parts := strings.SplitN(f, ".", 2)
		key := parts[0]
		val, ok := obj[key]
		if !ok {
			continue
		}
		if len(parts) == 1 {
			result[key] = val
		} else {
			// Nested field
			if nested, ok := val.(map[string]any); ok {
				if existing, ok := result[key].(map[string]any); ok {
					// Merge into existing picked nested object
					for k, v := range pickFields(nested, []string{parts[1]}) {
						existing[k] = v
					}
				} else {
					result[key] = pickFields(nested, []string{parts[1]})
				}
			}
		}
	}
	return result
}

// applyFilter filters array items. Format: "field:op:value"
// Ops: contains, eq, ne, gt, lt
func applyFilter(arr []any, filterStr string) []any {
	parts := strings.SplitN(filterStr, ":", 3)
	if len(parts) != 3 {
		return arr
	}
	field, op, value := parts[0], parts[1], parts[2]

	var result []any
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fieldVal := getNestedField(obj, field)
		if fieldVal == nil {
			continue
		}
		if matchFilter(fieldVal, op, value) {
			result = append(result, item)
		}
	}
	return result
}

// getNestedField retrieves a value using dot notation.
func getNestedField(obj map[string]any, field string) any {
	parts := strings.SplitN(field, ".", 2)
	val, ok := obj[parts[0]]
	if !ok {
		return nil
	}
	if len(parts) == 1 {
		return val
	}
	if nested, ok := val.(map[string]any); ok {
		return getNestedField(nested, parts[1])
	}
	return nil
}

// matchFilter checks if a value matches the filter operation.
func matchFilter(fieldVal any, op, value string) bool {
	fieldStr := fmt.Sprintf("%v", fieldVal)

	switch op {
	case "contains":
		return strings.Contains(strings.ToLower(fieldStr), strings.ToLower(value))
	case "eq":
		return strings.EqualFold(fieldStr, value)
	case "ne":
		return !strings.EqualFold(fieldStr, value)
	case "gt":
		fv, err1 := strconv.ParseFloat(fieldStr, 64)
		cv, err2 := strconv.ParseFloat(value, 64)
		return err1 == nil && err2 == nil && fv > cv
	case "lt":
		fv, err1 := strconv.ParseFloat(fieldStr, 64)
		cv, err2 := strconv.ParseFloat(value, 64)
		return err1 == nil && err2 == nil && fv < cv
	}
	return false
}
