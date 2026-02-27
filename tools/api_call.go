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

func registerAPICallTool(s *server.MCPServer, registry *arrservice.Registry, maxResponseSizeKB int, allowDestructive bool) {
	s.AddTool(
		mcp.NewTool("call_api",
			mcp.WithDescription("Make an authenticated API call to any configured *arr service. Returns the JSON response. Use fields/limit/filter to reduce response size."),
			mcp.WithString("service", mcp.Required(), mcp.Description("Service name (e.g. sonarr, radarr)")),
			mcp.WithString("method", mcp.Description("HTTP method (default: GET)")),
			mcp.WithString("path", mcp.Required(), mcp.Description("API path (e.g. /series, /movie). The API version prefix is added automatically.")),
			mcp.WithString("query", mcp.Description("Query parameters as JSON object (e.g. {\"term\": \"breaking bad\"})")),
			mcp.WithString("body", mcp.Description("Request body as JSON string")),
			mcp.WithString("fields", mcp.Description("Comma-separated fields to include in response. Supports nested fields with dot notation (e.g. \"id,title,statistics.sizeOnDisk\"). For paginated responses, drill into arrays: \"records.id,records.title,records.status\" to select fields from each item in the records array.")),
			mcp.WithString("filter", mcp.Description("Filter array results. Format: \"field:op:value\". Ops: contains, eq, ne, gt, lt (e.g. \"title:contains:Pirates\", \"year:gt:2000\", \"hasFile:eq:true\")")),
			mcp.WithString("limit", mcp.Description("Max number of items to return from array responses")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handleCallAPI(ctx, req, registry, maxResponseSizeKB, allowDestructive)
		},
	)
}

func handleCallAPI(ctx context.Context, req mcp.CallToolRequest, registry *arrservice.Registry, maxResponseSizeKB int, allowDestructive bool) (*mcp.CallToolResult, error) {
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

	if method == "DELETE" && !allowDestructive {
		return mcp.NewToolResultError("DELETE requests are disabled. Set allow_destructive: true in config.yaml to enable."), nil
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

	// Parse body — handle both string and object forms.
	// Some MCP clients may deserialize a JSON body string into an object.
	var body []byte
	if bodyStr != "" {
		if !json.Valid([]byte(bodyStr)) {
			return mcp.NewToolResultError("invalid body JSON"), nil
		}
		body = []byte(bodyStr)
	} else if raw, ok := req.GetArguments()["body"]; ok && raw != nil {
		// Body was passed as a JSON object, not a string — marshal it back.
		b, err := json.Marshal(raw)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid body: %v", err)), nil
		}
		body = b
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

	// Apply filter, fields, limit to responses
	needsProcessing := fieldsStr != "" || filterStr != "" || limitStr != ""
	if needsProcessing {
		jsonResp = processResponse(jsonResp, fieldsStr, filterStr, limitStr)
	}

	// Response size guard — catch oversized responses before they eat the
	// LLM's context window. Applies to both raw and processed responses.
	maxResponseBytes := maxResponseSizeKB * 1024
	data, _ := json.MarshalIndent(jsonResp, "", "  ")

	if len(data) > maxResponseBytes {
		// Find the largest array in the response (top-level or nested)
		arr, fieldPath := findLargestArray(jsonResp)
		if len(arr) > 0 {
			var availableFields []string
			if obj, ok := arr[0].(map[string]any); ok {
				for k := range obj {
					availableFields = append(availableFields, k)
				}
			}

			sizeKB := len(data) / 1024
			hint := fmt.Sprintf("⚠️ Response too large (%dKB, %d items", sizeKB, len(arr))
			if fieldPath != "" {
				hint += fmt.Sprintf(" in \"%s\"", fieldPath)
			}
			hint += "). This would consume excessive tokens.\n\n"
			hint += "Retry this call with the fields param to select only the fields you need.\n"
			if len(availableFields) > 0 {
				prefix := ""
				if fieldPath != "" {
					prefix = fieldPath + "."
				}
				hint += fmt.Sprintf("Available fields: %s\n", joinWithPrefix(availableFields, prefix))
				hint += fmt.Sprintf("\nExample: fields: \"%sid,%stitle,%sstatus\"\n", prefix, prefix, prefix)
			}
			hint += "\nYou can also use filter and limit params."
			hint += "\nDo NOT retry this call without fields, filter, or limit."

			return mcp.NewToolResultError(hint), nil
		}

		// No array found — generic size warning
		return mcp.NewToolResultError(fmt.Sprintf(
			"⚠️ Response too large (%dKB). Use fields param to reduce response size.",
			len(data)/1024)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

// processResponse applies filter, fields, and limit to the API response.
// Handles both top-level arrays and object responses with nested arrays
// (e.g. {records: [...], page: 1, totalRecords: 50}).
// Fields like "records.title" will drill into the "records" array and
// pick "title" from each item.
func processResponse(resp any, fieldsStr, filterStr, limitStr string) any {
	// Top-level array — apply directly
	if arr, ok := resp.([]any); ok {
		return processArray(arr, fieldsStr, filterStr, limitStr)
	}

	// Object response — check for nested array field selection
	obj, isObj := resp.(map[string]any)
	if !isObj {
		return resp
	}

	if fieldsStr != "" {
		fields := parseFields(fieldsStr)

		// Group fields by their top-level key to detect array drilling
		// e.g. "records.title,records.year,page" → {records: [title, year], page: []}
		grouped := make(map[string][]string)
		var topFields []string
		for _, f := range fields {
			parts := strings.SplitN(f, ".", 2)
			key := parts[0]
			if len(parts) == 2 {
				// Check if this key holds an array — if so, treat as array field selection
				if arr, ok := obj[key].([]any); ok {
					grouped[key] = append(grouped[key], parts[1])
					_ = arr
					continue
				}
			}
			topFields = append(topFields, f)
		}

		// Process array fields with sub-selection
		if len(grouped) > 0 {
			result := make(map[string]any)
			// Keep any requested top-level scalar fields
			if len(topFields) > 0 {
				picked := pickFields(obj, topFields)
				for k, v := range picked {
					result[k] = v
				}
			}
			// Drill into arrays
			for key, subFields := range grouped {
				arr, ok := obj[key].([]any)
				if !ok {
					continue
				}
				subFieldsStr := strings.Join(subFields, ",")
				result[key] = processArray(arr, subFieldsStr, filterStr, limitStr)
			}
			return result
		}

		// No array drilling — just pick top-level fields
		return pickFields(obj, fields)
	}

	// No fields but filter/limit — find and process nested arrays
	if filterStr != "" || limitStr != "" {
		for k, v := range obj {
			if arr, ok := v.([]any); ok {
				obj[k] = processArray(arr, "", filterStr, limitStr)
			}
		}
	}

	return obj
}

// processArray applies filter, limit, and field selection to an array.
func processArray(arr []any, fieldsStr, filterStr, limitStr string) any {
	if filterStr != "" {
		arr = applyFilter(arr, filterStr)
	}

	if limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(arr) {
			arr = arr[:limit]
		}
	}

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

// findLargestArray finds the largest []any in a response, checking both
// top-level arrays and arrays nested one level deep in objects.
// Returns the array and the field path (empty for top-level).
func findLargestArray(resp any) ([]any, string) {
	if arr, ok := resp.([]any); ok {
		return arr, ""
	}
	if obj, ok := resp.(map[string]any); ok {
		var largest []any
		var largestKey string
		for k, v := range obj {
			if arr, ok := v.([]any); ok && len(arr) > len(largest) {
				largest = arr
				largestKey = k
			}
		}
		return largest, largestKey
	}
	return nil, ""
}

// joinWithPrefix joins field names with a prefix for display.
func joinWithPrefix(fields []string, prefix string) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = prefix + f
	}
	return strings.Join(parts, ", ")
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
