package domain

import (
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"Example.Com", "example.com"},
		{"www.example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"https://example.com", "example.com"},
		{"https://example.com/", "example.com"},
		{"https://example.com/path", "example.com"},
		{"https://example.com:8080", "example.com"},
		{"https://example.com.", "example.com"},
		{"https://example.com./", "example.com"},
		{"HTTPS://EXAMPLE.COM", "example.com"},
		{"http://www.example.com/path/to/page", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Normalize(tt.input)
			if result != tt.expected {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"https://example.com", "https://example.com/", false},
		{"http://example.com", "http://example.com/", false},
		{"https://example.com/path", "https://example.com/path", false},
		{"https://EXAMPLE.COM/Path", "https://example.com/Path", false},
		{"", "", true},
		{"example.com", "", true},
		{"ftp://example.com", "ftp://example.com/", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := NormalizeURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeURL(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("NormalizeURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://example.com", "example.com"},
		{"https://example.com/path", "example.com"},
		{"https://www.example.com:8080/path", "www.example.com"},
		{"http://example.com", "example.com"},
		{"example.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ExtractHost(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractHost(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsSameSiteHost(t *testing.T) {
	tests := []struct {
		host     string
		domain   string
		expected bool
	}{
		{"example.com", "example.com", true},
		{"www.example.com", "example.com", true},
		{"sub.example.com", "example.com", false},
		{"other.com", "example.com", false},
		{"EXAMPLE.COM", "example.com", true},
		{"example.com", "www.example.com", true},
		{"www.example.com", "www.example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.host+"_"+tt.domain, func(t *testing.T) {
			result := IsSameSiteHost(tt.host, tt.domain)
			if result != tt.expected {
				t.Errorf("IsSameSiteHost(%q, %q) = %v, want %v", tt.host, tt.domain, result, tt.expected)
			}
		})
	}
}

func TestIsSubdomain(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"example.com", false},
		{"www.example.com", false},
		{"sub.example.com", true},
		{"www.sub.example.com", true},
		{"a.b.c.example.com", true},
		{"example.com.", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsSubdomain(tt.input)
			if result != tt.expected {
				t.Errorf("IsSubdomain(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestApexDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"www.example.com", "example.com"},
		{"sub.example.com", "example.com"},
		{"www.sub.example.com", "example.com"},
		{"a.b.c.example.com", "example.com"},
		{"example.com.", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ApexDomain(tt.input)
			if result != tt.expected {
				t.Errorf("ApexDomain(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
