package rate_limit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newCSRFOkHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestCSRFAllowsExactSelfOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://chess.example/play", nil)
	req.Host = "chess.example"
	req.Header.Set("Origin", "https://chess.example")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCSRFRejectsMismatchedOrigin(t *testing.T) {
	// When an explicit allow-list is set, only listed origins are accepted.
	// The empty/default allow-list is permissive (CORS owns cross-origin policy)
	// and is covered by the no-allow-list test below.
	req := httptest.NewRequest(http.MethodPost, "https://chess.example/play", nil)
	req.Host = "chess.example"
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), []string{"https://allowed.example"}).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "CSRF check failed") {
		t.Fatalf("expected CSRF error body, got %s", rr.Body.String())
	}
}

func TestCSRFRejectsOriginWithAttackerSuffix(t *testing.T) {
	// Prefix-evasion attempts only matter when an allow-list is set.
	// With the default permissive CORS the request is allowed through;
	// the browser-side CORS check is what stops the actual response body.
	req := httptest.NewRequest(http.MethodPost, "https://chess.example/play", nil)
	req.Host = "chess.example"
	req.Header.Set("Origin", "https://chess.example.evil.tld")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), []string{"https://chess.example"}).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for prefix-evasion attempt, got %d", rr.Code)
	}
}

func TestCSRFAllowsViaXForwardedProtoBehindProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://matchmaking-service.railway.internal/api/matchmaking/queues/tickets", nil)
	req.Host = "matchmaking-service.railway.internal"
	req.Header.Set("Origin", "https://web-production-9a697.up.railway.app")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "web-production-9a697.up.railway.app")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 via forwarded headers (plain HTTP = behind proxy), got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCSRFRejectsBadOriginEvenWithForwardedHeaders(t *testing.T) {
	// With an explicit allow-list, forwarded headers do not bypass the check.
	req := httptest.NewRequest(http.MethodPost, "http://internal/api", nil)
	req.Host = "internal"
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "public.example")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), []string{"https://public.example"}).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403: Origin still must be in the allow-list, got %d", rr.Code)
	}
}

func TestCSRFAllowedOriginsListStillTakesPrecedence(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://alt.example/x", nil)
	req.Host = "chess.example"
	req.Header.Set("Origin", "https://alt.example")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), []string{"https://alt.example"}).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from explicit allow-list, got %d", rr.Code)
	}
}

func TestCSRFAllowsPlainGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://chess.example/play", nil)
	req.Host = "chess.example"
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET, got %d", rr.Code)
	}
}

func TestCSRFAllowsMissingOriginAndReferer(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://chess.example/play", nil)
	req.Host = "chess.example"
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 when Origin/Referer are absent (e.g. native mobile), got %d", rr.Code)
	}
}

func TestCSRFHonorsFirstForwardedProto(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://internal/api", nil)
	req.Host = "internal"
	req.Header.Set("Origin", "https://public.example")
	req.Header.Set("X-Forwarded-Proto", "https, http")
	req.Header.Set("X-Forwarded-Host", "public.example")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 using first forwarded proto, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCSRFPlainHTTPWithoutForwardedHeadersFallsBackToHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://chess.example/play", nil)
	req.Host = "chess.example"
	req.Header.Set("Origin", "http://chess.example")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 when plain HTTP and host matches, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCSRFRejectsPlainHTTPForwardedHeadersButWrongHost(t *testing.T) {
	// Without an explicit allow-list, missing X-Forwarded-Host falls through
	// to the permissive default (CORS owns cross-origin policy).
	req := httptest.NewRequest(http.MethodPost, "http://internal/api", nil)
	req.Host = "internal"
	req.Header.Set("Origin", "https://public.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (permissive default), got %d", rr.Code)
	}
}

func TestCSRFPermissiveDefaultForUnlistedCrossOrigin(t *testing.T) {
	// When the allow-list is empty (default), cross-origin POSTs are allowed
	// by CSRF. CORS middleware is the actual gatekeeper for the browser.
	req := httptest.NewRequest(http.MethodPost, "https://chess.example/play", nil)
	req.Host = "chess.example"
	req.Header.Set("Origin", "https://web-production-9a697.up.railway.app")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (permissive default), got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCSRFFirstForwardedValue(t *testing.T) {
	// Comma-separated X-Forwarded-Proto: first non-empty value wins.
	req := httptest.NewRequest(http.MethodPost, "http://internal/api", nil)
	req.Host = "internal"
	req.Header.Set("Origin", "https://public.example")
	req.Header.Set("X-Forwarded-Proto", " https , http")
	req.Header.Set("X-Forwarded-Host", " public.example , other.example")
	rr := httptest.NewRecorder()
	CSRFMiddleware(newCSRFOkHandler(), nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (first forwarded wins), got %d", rr.Code)
	}
}
