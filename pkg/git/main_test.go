package git

import (
	"regexp"
	"testing"
)

func TestInternalLinkProcessor_Regex(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		baseURL string
		want    bool
	}{
		{"external domain should not match", "https://google.com", "https://google.com", false},
		{"same host but base equals URL should not count as internal file link", "https://github.com", "https://github.com", false},
		{"github blob README internal", "https://github.com/your-ko/link-validator/blob/main/README.md", "https://github.com", true},
		{"github blob Dockerfile internal", "https://github.com/your-ko/link-validator/blob/main/Dockerfile", "https://github.com", true},
		{"query string stays internal", "https://github.com/your-ko/link-validator/blob/main/README.md?utm=1", "https://github.com", true},
		{"different host must not match", "https://example.com/your-ko/link-validator/blob/main/README.md", "https://github.com", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proc := New(tt.baseURL, "")
			re := proc.Regex()
			if re == nil {
				t.Fatalf("Regex() returned nil")
			}
			if _, ok := any(re).(*regexp.Regexp); !ok {
				t.Fatalf("Regex() did not return *regexp.Regexp")
			}

			// We expect the regex to match the whole URL when it's "internal".
			got := re.FindString(tt.url)
			matched := got == tt.url

			if matched != tt.want {
				t.Fatalf("Regex().FindString(%q) = %q (matched=%v), want matched=%v; baseURL=%q",
					tt.url, got, matched, tt.want, tt.baseURL)
			}

			// Optional: be stricter — if not wanted, ensure no partial matches either.
			if !tt.want {
				if idx := re.FindStringIndex(tt.url); idx != nil {
					t.Fatalf("Regex() unexpectedly found a partial match at %v in %q; baseURL=%q",
						idx, tt.url, tt.baseURL)
				}
			}
		})
	}
}
