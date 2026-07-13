//go:build unit

package service

import "testing"

func TestIsOAuthCredentialFailureReason(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   bool
	}{
		{name: "oauth 401", reason: "OAuth 401: access token expired", want: true},
		{name: "refresh exhausted", reason: "token refresh retry exhausted: timeout", want: true},
		{name: "missing refresh token", reason: "Authentication failed (401): refresh_token missing", want: true},
		{name: "real quota limit", reason: "rate limit exceeded until five hour window resets", want: false},
		{name: "workspace deactivated", reason: "Workspace deactivated (402)", want: false},
		{name: "empty", reason: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOAuthCredentialFailureReason(tt.reason); got != tt.want {
				t.Fatalf("isOAuthCredentialFailureReason(%q) = %v, want %v", tt.reason, got, tt.want)
			}
		})
	}
}
