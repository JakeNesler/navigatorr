package config

// DefaultPorts maps service type to default port.
var DefaultPorts = map[string]int{
	"sonarr":         8989,
	"radarr":         7878,
	"lidarr":         8686,
	"readarr":        8787,
	"chaptarr":       8789,
	"prowlarr":       9696,
	"profilarr":      6868,
	"bazarr":         6767,
	"overseerr":      5055,
	"jellyseerr":     5055,
	"seerr":          5055,
	"audiobookshelf": 13378,
}

// DefaultAPIVersions maps service type to API path prefix.
var DefaultAPIVersions = map[string]string{
	"sonarr":         "/api/v3",
	"radarr":         "/api/v3",
	"lidarr":         "/api/v1",
	"readarr":        "/api/v1",
	"chaptarr":       "/api/v1",
	"prowlarr":       "/api/v1",
	"profilarr":      "/api/v1",
	"bazarr":         "/api",
	"overseerr":      "/api/v1",
	"jellyseerr":     "/api/v1",
	"seerr":          "/api/v1",
	"audiobookshelf": "/api",
}

// DefaultOpenAPIURLs maps service type to raw GitHub URL for OpenAPI spec.
var DefaultOpenAPIURLs = map[string]string{
	"sonarr":         "https://raw.githubusercontent.com/Sonarr/Sonarr/develop/src/Sonarr.Api.V3/openapi.json",
	"radarr":         "https://raw.githubusercontent.com/Radarr/Radarr/develop/src/Radarr.Api.V3/openapi.json",
	"lidarr":         "https://raw.githubusercontent.com/Lidarr/Lidarr/develop/src/Lidarr.Api.V1/openapi.json",
	"readarr":        "https://raw.githubusercontent.com/Readarr/Readarr/develop/src/Readarr.Api.V1/openapi.json",
	"chaptarr":       "https://raw.githubusercontent.com/Chaptarr/chaptarr/develop/src/Chaptarr.Api.V1/openapi.json",
	"prowlarr":       "https://raw.githubusercontent.com/Prowlarr/Prowlarr/develop/src/Prowlarr.Api.V1/openapi.json",
	"profilarr":      "https://raw.githubusercontent.com/Dictionarry-Hub/profilarr/develop/src/lib/api/v1.openapi.json",
	"overseerr":      "https://raw.githubusercontent.com/sct/overseerr/develop/overseerr-api.yml",
	"jellyseerr":     "https://raw.githubusercontent.com/seerr-team/seerr/refs/heads/develop/seerr-api.yml",
	"seerr":          "https://raw.githubusercontent.com/seerr-team/seerr/refs/heads/develop/seerr-api.yml",
	"audiobookshelf": "https://raw.githubusercontent.com/advplyr/audiobookshelf/master/docs/openapi.json",
}

// DefaultAuthMethods maps service type to authentication method.
var DefaultAuthMethods = map[string]string{
	"sonarr":         "header", // X-Api-Key header
	"radarr":         "header", // X-Api-Key header
	"lidarr":         "header", // X-Api-Key header
	"readarr":        "header", // X-Api-Key header
	"chaptarr":       "header", // X-Api-Key header
	"prowlarr":       "header", // X-Api-Key header
	"profilarr":      "header", // X-Api-Key header
	"bazarr":         "header", // X-Api-Key header
	"overseerr":      "header", // X-Api-Key header
	"jellyseerr":     "header", // X-Api-Key header
	"seerr":          "header", // X-Api-Key header
	"audiobookshelf": "header", // Authorization: Bearer <token>
}

// DefaultAuthHeaders maps service type to auth header name override.
var DefaultAuthHeaders = map[string]string{
	"audiobookshelf": "Authorization",
}

// DefaultStatusPaths maps service type to a cheap authenticated endpoint used
// to report connection status. Relative to the service's API version prefix.
var DefaultStatusPaths = map[string]string{
	"sonarr":     "/system/status",
	"radarr":     "/system/status",
	"lidarr":     "/system/status",
	"readarr":    "/system/status",
	"chaptarr":   "/system/status",
	"prowlarr":   "/system/status",
	"profilarr":  "/status",
	"bazarr":     "/system/status",
	"overseerr":  "/status",
	"jellyseerr": "/status",
	"seerr":      "/status",
	// Audiobookshelf serves /ping at the root rather than under /api, so
	// pinging it through the API prefix is always a 404. /me is under /api and
	// is authenticated, so it also proves the token works rather than only
	// that the host is up.
	"audiobookshelf": "/me",
}

// DefaultRequestTimeoutSeconds is the ceiling on a single call_api request,
// unchanged from the value that was hardcoded before it was configurable.
const DefaultRequestTimeoutSeconds = 30

// DefaultSelfHostedSpecPaths maps service type to the instance-relative path
// for services that publish no spec to GitHub but serve one themselves. These
// resolve against the configured instance URL instead of a fixed URL.
var DefaultSelfHostedSpecPaths = map[string]string{
	"bazarr": "/api/swagger.json",
}

// DefaultAuthPrefixes maps service type to auth value prefix.
var DefaultAuthPrefixes = map[string]string{
	"audiobookshelf": "Bearer",
}
