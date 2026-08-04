package config

import "testing"

func TestResolveURL(t *testing.T) {
	tests := []struct {
		name    string
		service string
		raw     string
		want    string
		wantErr bool
	}{
		{"omitted url falls back to localhost", "sonarr", "", "http://localhost:8989", false},
		{"bare host gets default port", "sonarr", "http://10.0.0.100", "http://10.0.0.100:8989", false},
		{"missing scheme is filled in", "radarr", "10.0.0.100", "http://10.0.0.100:7878", false},
		{"explicit port is kept", "sonarr", "http://10.0.0.100:9999", "http://10.0.0.100:9999", false},
		{"https is preserved", "seerr", "https://seerr.example.com", "https://seerr.example.com:5055", false},
		{"trailing slash is trimmed", "sonarr", "http://10.0.0.100:8989/", "http://10.0.0.100:8989", false},
		{"subpath is kept", "sonarr", "http://10.0.0.100:8989/sonarr", "http://10.0.0.100:8989/sonarr", false},
		{"unknown service keeps url as given", "custom", "http://10.0.0.100:1234", "http://10.0.0.100:1234", false},
		{"unknown service without port is left alone", "custom", "http://10.0.0.100", "http://10.0.0.100", false},
		{"unknown service without url is an error", "custom", "", "", true},
		{"url without host is an error", "sonarr", "http://", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveURL(tt.service, tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveURL(%q, %q) = %q, want error", tt.service, tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveURL(%q, %q): unexpected error: %v", tt.service, tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("resolveURL(%q, %q) = %q, want %q", tt.service, tt.raw, got, tt.want)
			}
		})
	}
}
