package httputil

import "testing"

func TestRedactURLCredentialsEmpty(t *testing.T) {
	if got := RedactURLCredentials(""); got != "" {
		t.Fatalf("empty input should return empty, got %q", got)
	}
}

func TestRedactURLCredentialsNoUserInfo(t *testing.T) {
	in := "https://api.example.com/x"
	if got := RedactURLCredentials(in); got != in {
		t.Fatalf("URL without user-info should pass through, got %q", got)
	}
}

func TestRedactURLCredentialsWithPassword(t *testing.T) {
	in := "redis://default:sRHGauPpO0EVyr1CsBPHkOfOBFlnSZnCT@redis.railway.internal:6379"
	want := "redis://default:REDACTED@redis.railway.internal:6379"
	if got := RedactURLCredentials(in); got != want {
		t.Fatalf("password should be redacted, got %q want %q", got, want)
	}
}

func TestRedactURLCredentialsUsernameOnly(t *testing.T) {
	in := "redis://default@redis.railway.internal:6379"
	if got := RedactURLCredentials(in); got != in {
		t.Fatalf("URL with username only (no password) should pass through, got %q", got)
	}
}

func TestRedactURLCredentialsMalformed(t *testing.T) {
	in := "://not a url"
	if got := RedactURLCredentials(in); got != "<unparseable-url>" {
		t.Fatalf("malformed URL should return placeholder, got %q", got)
	}
}
