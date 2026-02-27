package config

// DefaultPorts maps service type to default port.
var DefaultPorts = map[string]int{
	"sonarr":    8989,
	"radarr":    7878,
	"lidarr":    8686,
	"readarr":   8787,
	"prowlarr":  9696,
	"bazarr":    6767,
	"overseerr": 5055,
}

// DefaultAPIVersions maps service type to API path prefix.
var DefaultAPIVersions = map[string]string{
	"sonarr":    "/api/v3",
	"radarr":    "/api/v3",
	"lidarr":    "/api/v1",
	"readarr":   "/api/v1",
	"prowlarr":  "/api/v1",
	"bazarr":    "/api",
	"overseerr": "/api/v1",
}

// DefaultOpenAPIURLs maps service type to raw GitHub URL for OpenAPI spec.
var DefaultOpenAPIURLs = map[string]string{
	"sonarr":    "https://raw.githubusercontent.com/Sonarr/Sonarr/develop/src/Sonarr.Api.V3/openapi.json",
	"radarr":    "https://raw.githubusercontent.com/Radarr/Radarr/develop/src/Radarr.Api.V3/openapi.json",
	"lidarr":    "https://raw.githubusercontent.com/Lidarr/Lidarr/develop/src/Lidarr.Api.V1/openapi.json",
	"readarr":   "https://raw.githubusercontent.com/Readarr/Readarr/develop/src/Readarr.Api.V1/openapi.json",
	"prowlarr":  "https://raw.githubusercontent.com/Prowlarr/Prowlarr/develop/src/Prowlarr.Api.V1/openapi.json",
	"overseerr": "https://raw.githubusercontent.com/sct/overseerr/develop/overseerr-api.yml",
}

// DefaultAuthMethods maps service type to authentication method.
var DefaultAuthMethods = map[string]string{
	"sonarr":    "header",   // X-Api-Key header
	"radarr":    "header",   // X-Api-Key header
	"lidarr":    "header",   // X-Api-Key header
	"readarr":   "header",   // X-Api-Key header
	"prowlarr":  "header",   // X-Api-Key header
	"bazarr":    "header",   // X-Api-Key header
	"overseerr": "header",   // X-Api-Key header
}
