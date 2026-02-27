# Navigatorr LLM Instructions

This guide explains how an AI assistant uses Navigatorr to manage your media stack. Every example below is based on real interactions, anonymized for sharing.

---

## How It Works

Navigatorr gives the LLM 16 tools over MCP. The LLM doesn't need to know API docs beforehand — it discovers endpoints at runtime, figures out the right parameters, and chains calls together to complete complex tasks from a single sentence.

---

## Tool Reference

### Discovery Tools

| Tool | When to Use |
|------|-------------|
| `list_services` | First call in any session. Shows what's configured, connection status, and endpoint counts. |
| `search_api` | When you need to find an endpoint but don't know the exact path. Searches across all services. |
| `list_endpoints` | Browse all endpoints for a service, optionally filtered by tag (e.g. "Series", "Movie") or method. |
| `get_endpoint_details` | Get full parameter schemas and request/response bodies for a specific endpoint before making a call. |

### The `call_api` Tool

This is the workhorse. It makes authenticated HTTP requests to any configured service.

**Parameters:**

| Param | Required | Description |
|-------|----------|-------------|
| `service` | Yes | Service name: `sonarr`, `radarr`, `lidarr`, `readarr`, `prowlarr`, `bazarr`, `overseerr` |
| `path` | Yes | API path without version prefix (e.g. `/series`, `/movie`). The version prefix (`/api/v3`, `/api/v1`) is added automatically. |
| `method` | No | HTTP method. Defaults to `GET`. |
| `query` | No | Query parameters as a JSON object: `{"term": "some show name"}` |
| `body` | No | Request body as a JSON string. |
| `fields` | No | Comma-separated fields to include in the response. Supports dot notation for nested fields and array drilling. |
| `filter` | No | Filter array results. Format: `field:op:value`. Ops: `contains`, `eq`, `ne`, `gt`, `lt`. |
| `limit` | No | Max items to return from array responses. |

**Field Selection Examples:**

```
# Simple — just get IDs and titles
fields: "id,title,year"

# Nested — reach into sub-objects
fields: "id,title,statistics.sizeOnDisk"

# Array drilling — for paginated responses like /queue
# The response is {records: [...], page: 1, totalRecords: 50}
# This drills into the records array and picks fields from each item:
fields: "records.id,records.title,records.status"
```

**Filter Examples:**

```
# Find movies containing a keyword in the title
filter: "title:contains:keyword"

# Only movies from after 2020
filter: "year:gt:2020"

# Only items that have a file on disk
filter: "hasFile:eq:true"
```

### Safety Features

**Response Size Guard:** If a response exceeds the configured threshold (default 50KB), the tool returns an error with:
- The response size and item count
- All available field names discovered from the first item
- An example `fields` parameter to retry with

This prevents a single API call from consuming the LLM's entire context window.

**DELETE Protection:** By default, all DELETE requests are blocked. The LLM will receive a clear error message. To enable deletions, set `allow_destructive: true` in your config.

### Torrent Tools

**Transmission:**
- `transmission_list_torrents` — List all with status and progress
- `transmission_add_torrent` — Add by magnet link or URL
- `transmission_manage_torrent` — Actions: `start`, `stop`, `remove`, `remove_data`, `verify`
- `transmission_free_space` — Check disk space at a path

**qBittorrent:**
- `qbit_list_torrents` — List all with status and progress
- `qbit_add_torrent` — Add by magnet link or URL
- `qbit_manage_torrent` — Actions: `pause`, `resume`, `delete`, `delete_files`
- `qbit_transfer_info` — Global transfer speeds and stats

---

## Real-World Patterns

These patterns show how the LLM chains tools to accomplish tasks. All data is anonymized.

### Pattern 1: Self-Discovery

The LLM doesn't come pre-loaded with API knowledge. It discovers everything at runtime.

```
User: "Add a new indexer to Prowlarr"

LLM thinking:
  1. I need to find the right endpoint for adding indexers
  2. search_api → query: "indexer", service: "prowlarr"
     → Finds: POST /api/v1/indexer
  3. I need the request body schema
  4. get_endpoint_details → service: "prowlarr", path: "/indexer", method: "POST"
     → Returns full schema with required fields
  5. But I need to know what fields are valid — let me check existing indexers
  6. call_api → service: "prowlarr", path: "/indexer", fields: "id,name,fields"
     → Returns existing indexer configs to use as a template
  7. I also need the app profile ID
  8. call_api → service: "prowlarr", path: "/appprofile"
     → Returns [{id: 1, name: "Standard"}]
  9. Now I can construct the POST body
  10. call_api → POST /indexer with the constructed body
```

The LLM figured out 4 things it didn't know (endpoint path, body schema, field values, required appProfileId) by chaining discovery tools before making the actual call.

### Pattern 2: Bulk Operations with Filtering

```
User: "Unmonitor all movies in a franchise except the recent ones"

LLM approach:
  1. call_api → service: "radarr", path: "/movie",
     filter: "title:contains:franchise name", fields: "id,title,year,monitored"
     → Returns 25 movies in that franchise

  2. Identifies the recent entries (e.g. 2006-2024) from the results

  3. For each older entry:
     call_api → PUT /movie/{id}, body: {"monitored": false, ...}

  4. Confirms: "Unmonitored 20 movies. Kept 5 recent entries monitored."
```

### Pattern 3: Cross-Service Coordination

```
User: "Add qBittorrent as a download client in both Radarr and Sonarr"

LLM approach:
  1. search_api → query: "downloadclient"
     → Finds endpoints across both services

  2. get_endpoint_details → path: "/downloadclient", method: "POST"
     → Gets the schema with all required fields

  3. call_api → service: "radarr", path: "/downloadclient/schema"
     → Gets implementation-specific field definitions for qBittorrent

  4. Constructs the POST body with host, port, credentials, category settings

  5. call_api → POST /downloadclient on radarr
  6. call_api → POST /downloadclient on sonarr
     → Same body works for both since they share the API
```

### Pattern 4: Queue Management

```
User: "Why are these movies still downloading? I unmonitored them"

LLM approach:
  1. call_api → service: "radarr", path: "/queue",
     fields: "records.id,records.title,records.status,records.movieId"
     → Discovers 28 items still in the download queue

  2. The nested array drilling (records.id, records.title) keeps the
     response small — without it, each queue record contains download
     client details, quality info, and status history that would be
     hundreds of KB

  3. call_api → DELETE /queue/bulk,
     body: {"ids": [1,2,3,...], "removeFromClient": true}
     → Removes from queue AND tells the download client to stop
```

### Pattern 5: Handling the Size Guard

```
User: "Show me all my movies"

LLM calls:
  call_api → service: "radarr", path: "/movie"

Response (from Navigatorr, not the API):
  "Response too large (1451KB, 216 items). This would consume
   excessive tokens.

   Retry with fields param. Available fields: id, title, year,
   status, hasFile, monitored, runtime, genres, added, ...

   Example: fields: "id,title,status"

   Do NOT retry without fields, filter, or limit."

LLM retries:
  call_api → service: "radarr", path: "/movie",
  fields: "id,title,year,hasFile", limit: "20"
  → Clean 2KB response
```

The LLM never sees the raw 1.4MB dump. The guard teaches it how to ask efficiently.

### Pattern 6: Adding Media

```
User: "Search for a show and add it to Sonarr"

LLM approach:
  1. search_api → query: "lookup", service: "sonarr"
     → Finds GET /series/lookup

  2. call_api → service: "sonarr", path: "/series/lookup",
     query: {"term": "show name"},
     fields: "title,year,tvdbId,overview"
     → Returns search results from TVDB

  3. Confirms with user: "Found Show Name (2020). Add it?"

  4. Needs to know quality profiles and root folders:
     call_api → GET /qualityprofile, fields: "id,name"
     call_api → GET /rootfolder, fields: "id,path,freeSpace"

  5. call_api → POST /series with full body including
     tvdbId, qualityProfileId, rootFolderPath, monitored: true

  6. call_api → POST /command,
     body: {"name": "SeriesSearch", "seriesId": 42}
     → Triggers an immediate search for episodes
```

### Pattern 7: Triggering Searches and Rescans

```
User: "Rescan everything and search for missing media"

LLM approach (parallel where possible):
  1. call_api → POST /command on radarr,
     body: {"name": "RescanMovie"}
  2. call_api → POST /command on sonarr,
     body: {"name": "RescanSeries"}
  3. call_api → POST /command on radarr,
     body: {"name": "MissingMoviesSearch"}
  4. call_api → POST /command on sonarr,
     body: {"name": "MissingEpisodeSearch"}

  The LLM discovers command names via search_api or
  get_endpoint_details for /command.
```

---

## Configuration Reference

```yaml
# ~/.config/navigatorr/config.yaml

services:
  sonarr:
    url: "http://your-server:8989"
    api_key: "your-api-key-here"
  radarr:
    url: "http://your-server:7878"
    api_key: "your-api-key-here"
  # Add any: lidarr, readarr, prowlarr, bazarr, overseerr, jellyseerr

# Response size guard threshold (default: 50KB)
# Increase if you have a large context window, decrease for smaller models
max_response_size_kb: 50

# Block DELETE requests unless explicitly enabled (default: false)
allow_destructive: false

# Optional torrent clients
transmission:
  url: "http://your-server:9091"
  username: ""
  password: ""

qbittorrent:
  url: "http://your-server:8080"
  username: "admin"
  password: "your-password"
```

### Service Defaults

Navigatorr auto-configures these for known services — you only need `url` and `api_key`:

| Service | API Version | Auth Method | OpenAPI Spec |
|---------|------------|-------------|--------------|
| Sonarr | `/api/v3` | `X-Api-Key` header | Auto-fetched from GitHub |
| Radarr | `/api/v3` | `X-Api-Key` header | Auto-fetched from GitHub |
| Lidarr | `/api/v1` | `X-Api-Key` header | Auto-fetched from GitHub |
| Readarr | `/api/v1` | `X-Api-Key` header | Auto-fetched from GitHub |
| Prowlarr | `/api/v1` | `X-Api-Key` header | Auto-fetched from GitHub |
| Bazarr | `/api` | `X-Api-Key` header | Auto-fetched from GitHub |
| Overseerr | `/api/v1` | `X-Api-Key` header | Auto-fetched from GitHub |
| Jellyseerr | `/api/v1` | `X-Api-Key` header | Auto-fetched from GitHub |

You can override any default with explicit config:

```yaml
services:
  my_custom_arr:
    url: "http://custom:9999"
    api_key: "key"
    api_version: "/api/v2"
    auth_method: "query"        # "header", "query", or "basic"
    auth_header: "Authorization" # custom header name
    openapi_url: "https://example.com/spec.json"
```

---

## Tips for LLM Users

1. **Start with `list_services`** to see what's available and confirm connectivity.

2. **Use `search_api` before guessing endpoints.** The LLM should never hardcode an API path — always discover it first.

3. **Always use `fields` on large collections.** `/movie`, `/series`, `/album` return entire libraries. Use `fields: "id,title,year"` to keep responses small.

4. **Use `filter` to narrow results** instead of fetching everything and filtering client-side.

5. **Check `get_endpoint_details` before POST/PUT calls** to understand required fields and valid values.

6. **The LLM can read the size guard hints.** When a response is too large, the error message contains the exact field names available — the LLM should use those to retry.

7. **Chain discovery + action.** The ideal pattern is: `search_api` -> `get_endpoint_details` -> `call_api`. This works even for APIs the LLM has never seen before.

8. **Torrent clients are separate tools.** Don't try to manage torrents through `call_api` — use the dedicated `transmission_*` or `qbit_*` tools.
