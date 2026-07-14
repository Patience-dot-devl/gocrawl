package crawler

import "testing"

func TestSanitizeSeed(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantSeed string
		wantUser string
		wantPass string
	}{
		{
			name:     "no credentials is a no-op",
			raw:      "https://example.com/path",
			wantSeed: "https://example.com/path",
		},
		{
			name:     "user and password stripped",
			raw:      "https://alice:s3cret@example.com/path",
			wantSeed: "https://example.com/path",
			wantUser: "alice",
			wantPass: "s3cret",
		},
		{
			name:     "username only, no password",
			raw:      "https://alice@example.com",
			wantSeed: "https://example.com",
			wantUser: "alice",
			wantPass: "",
		},
		{
			name:     "invalid URL returned unchanged",
			raw:      "://not a url",
			wantSeed: "://not a url",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed, user, pass := SanitizeSeed(tt.raw)
			if seed != tt.wantSeed {
				t.Errorf("seed = %q, want %q", seed, tt.wantSeed)
			}
			if user != tt.wantUser {
				t.Errorf("user = %q, want %q", user, tt.wantUser)
			}
			if pass != tt.wantPass {
				t.Errorf("pass = %q, want %q", pass, tt.wantPass)
			}
		})
	}
}
