# Navigatorr

An MCP (Model Context Protocol) server that gives AI assistants like Claude direct access to your *arr media stack and Transmission torrent client. Browse API documentation, make authenticated calls, and manage downloads — all through natural language.

## What It Does

Navigatorr acts as a bridge between AI coding assistants and your self-hosted media services. Instead of manually navigating web UIs or crafting API calls, you describe what you want and the AI handles the rest through Navigatorr's MCP tools.

**Supported services:**
- **Sonarr** — TV show management
- **Radarr** — Movie management
- **Lidarr** — Music management
- **Readarr** — Book management
- **Prowlarr** — Indexer management
- **Bazarr** — Subtitle management
- **Overseerr** — Request management
- **Transmission** — Torrent client

## Architecture

```
Claude Code / MCP Client
        │
        ▼
   ┌─────────────┐
   │ Navigatorr  │  MCP Server (stdio transport)
   │              │
   │  ┌────────┐  │
   │  │ Tools  │  │  12 MCP tools exposed
   │  └───┬────┘  │
   │      │       │
   │  ┌───▼────┐  │
   │  │Registry│  │  Service registry + auth
   │  └───┬────┘  │
   │      │       │
   │  ┌───▼────┐  │
   │  │OpenAPI │  │  Spec parsing + caching
   │  │ Store  │  │
   │  └────────┘  │
   └──────┬───────┘
          │
          ▼
   *arr services + Transmission
```

### Package Structure

| Package | Purpose |
|---------|---------|
| `main` | Entry point, config loading, MCP server setup |
| `config` | YAML config parsing with service defaults |
| `arrservice` | Service registry, HTTP client, auth strategies (header/query/basic) |
| `openapi` | OpenAPI spec fetching, parsing, caching, and search |
| `tools` | MCP tool registration and handlers |
| `transmission` | Transmission RPC client |
| `internal` | Shared logging utilities |

### How It Works

1. **Config Loading** — Reads `~/.config/navigatorr/config.yaml` to discover services, API keys, and Transmission settings. Applies sensible defaults for known service types (API versions, auth methods, OpenAPI spec URLs).

2. **Service Registry** — Each configured service gets an authenticated HTTP client with the appropriate auth strategy (API key header, query parameter, or basic auth). The registry provides lookup by name.

3. **OpenAPI Spec Store** — On startup, fetches and parses OpenAPI specs from each service's official GitHub repo. Specs are cached to disk (`~/.cache/navigatorr/`) and indexed for fast endpoint lookup and full-text search.

4. **Tool Registration** — Three categories of tools are registered with the MCP server:
   - **API Documentation tools** — browse and search service endpoints without making calls
   - **API Call tool** — make authenticated requests with field selection, filtering, and pagination
   - **Transmission tools** — manage torrents (list, add, start, stop, remove, verify, free space)

5. **Stdio Transport** — Communicates with the MCP client over stdin/stdout using JSON-RPC, making it compatible with any MCP host (Claude Code, Cursor, etc.).

## MCP Tools

### API Documentation

| Tool | Description |
|------|-------------|
| `list_services` | List all configured services with URLs and connection status |
| `list_endpoints` | Browse API endpoints for a service, filterable by tag or HTTP method |
| `get_endpoint_details` | Full endpoint info including parameters, request body, and response schemas |
| `search_api` | Full-text search across all API specs |
| `refresh_api_specs` | Re-fetch OpenAPI specs from upstream |

### API Calls

| Tool | Description |
|------|-------------|
| `call_api` | Make authenticated API calls to any service. Supports field selection, filtering (`field:op:value`), and result limiting to keep responses manageable |

### Transmission

| Tool | Description |
|------|-------------|
| `transmission_list_torrents` | List all torrents with status, progress, and speeds |
| `transmission_add_torrent` | Add a torrent by magnet link or URL |
| `transmission_manage_torrent` | Start, stop, remove, or verify torrents |
| `transmission_free_space` | Check available disk space |

## Setup

### Prerequisites

- Go 1.25+
- Running *arr services with API keys
- (Optional) Transmission torrent client

### Option 1: Build from Source

```bash
go build -o navigatorr .
```

### Option 2: Docker

```bash
docker build -t navigatorr .
```

### Configure

Copy the example config and fill in your values:

```bash
mkdir -p ~/.config/navigatorr
cp config.yaml.example ~/.config/navigatorr/config.yaml
```

Edit `~/.config/navigatorr/config.yaml` with your service URLs and API keys. You can find API keys in each service's Settings > General page.

### Connect to Claude Code

**Using the binary directly:**

```json
{
  "mcpServers": {
    "navigatorr": {
      "type": "stdio",
      "command": "/path/to/navigatorr",
      "args": []
    }
  }
}
```

**Using Docker:**

```json
{
  "mcpServers": {
    "navigatorr": {
      "type": "stdio",
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-v", "~/.config/navigatorr/config.yaml:/root/.config/navigatorr/config.yaml:ro",
        "--network", "host",
        "navigatorr"
      ]
    }
  }
}
```

> **Note:** `--network host` is used so the container can reach your *arr services on the local network. If your services are on a remote host, you can use the default bridge network instead. The `-i` flag is required for stdio transport. Do not use `-t` as it interferes with the JSON-RPC communication.

**Custom config path:**

```json
{
  "mcpServers": {
    "navigatorr": {
      "type": "stdio",
      "command": "/path/to/navigatorr",
      "args": ["-config", "/path/to/config.yaml"]
    }
  }
}
```

## Usage Examples

Once connected, you can ask Claude things like:

- "What TV shows do I have in Sonarr?"
- "Search for a new series and add it"
- "Show me all movies missing from Radarr"
- "What torrents are currently downloading?"
- "Add this magnet link to Transmission"
- "How much free disk space do I have?"
- "Delete a series and re-add it with a different quality profile"

The AI uses Navigatorr's tools behind the scenes to browse API docs, discover the right endpoints, and make authenticated calls on your behalf.

### Demo Walkthrough

**1. Discover your services**
```
> "What services do I have?"
→ Tool: list_services
```
```
sonarr    → http://your-server:8989 (235 endpoints)
radarr    → http://your-server:7878 (238 endpoints)
lidarr    → http://your-server:8686 (236 endpoints)
prowlarr  → http://your-server:9696 (129 endpoints)
overseerr → http://your-server:5055 (170 endpoints)
...
```

**2. Search API endpoints without reading docs**
```
> "How do I manage quality profiles in Sonarr?"
→ Tool: search_api → query: "quality", service: "sonarr"
```
Returns 11 matching endpoints — GET, POST, PUT, DELETE for quality profiles and definitions. No digging through API docs.

**3. Browse endpoints by category**
```
> "What can I do with series?"
→ Tool: list_endpoints → service: "sonarr", tag: "Series"
```
```
GET    /api/v3/series       — List all series
POST   /api/v3/series       — Add a series
GET    /api/v3/series/{id}  — Get series details
PUT    /api/v3/series/{id}  — Update a series
DELETE /api/v3/series/{id}  — Delete a series
```

**4. Make authenticated API calls**
```
> "Show me all my TV shows"
→ Tool: call_api → service: "sonarr", path: "/series"
```
Returns full JSON with all series, episodes, and quality info. Handles auth headers, API versioning, and URL construction automatically. Supports field selection, filtering, and result limiting.

**5. Manage torrents**
```
> "What's downloading right now?"
→ Tool: transmission_list_torrents
```
```
Some.Show.S01E01.720p → downloading (45.2%)
Another.Show.S03.Pack → seeding (100%)
```

**6. Chain it all together**

The real power is that Claude chains tools automatically. Say:

> "Delete all my anime episodes and re-add the series with only dubbed releases"

Claude will:
1. `call_api` GET /series → find the series ID
2. `call_api` DELETE /series/{id} with deleteFiles=true
3. `search_api` → discover the custom format endpoints
4. `call_api` POST → create a "Dual Audio" custom format
5. `call_api` POST → create a quality profile requiring that format
6. `call_api` POST /series → re-add with the new profile
7. `call_api` POST /command → trigger a search

All from one sentence.

## Dependencies

| Dependency | Purpose |
|------------|---------|
| [mcp-go](https://github.com/mark3labs/mcp-go) | MCP server framework |
| [kin-openapi](https://github.com/getkin/kin-openapi) | OpenAPI 3.x spec parsing |
| [yaml.v3](https://gopkg.in/yaml.v3) | YAML config parsing |

## Built With

Code intelligence powered by [CartoGopher](https://cartogopher.com)

## License

MIT
