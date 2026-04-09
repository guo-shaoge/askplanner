package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lab/askplanner/internal/config"
)

func TestUsageAccessControlDisabledWhenNoPasswordHash(t *testing.T) {
	control, err := newUsageAccessControl(&config.Config{})
	if err != nil {
		t.Fatalf("newUsageAccessControl error: %v", err)
	}
	if control != nil {
		t.Fatalf("control = %#v, want nil", control)
	}
}

func TestUsageAccessControlRejectsUnauthorizedRequests(t *testing.T) {
	control, err := newUsageAccessControl(testUsageAuthConfig("secret"))
	if err != nil {
		t.Fatalf("newUsageAccessControl error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()

	control.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Fatal("missing WWW-Authenticate header")
	}
	if body := rec.Body.String(); body == "" {
		t.Fatal("missing auth response body")
	}
}

func TestUsageAccessControlAllowsValidCredentials(t *testing.T) {
	server, err := NewServer(&Collector{}, testUsageAuthConfig("secret"))
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.SetBasicAuth("askplanner", "secret")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "ok\n" {
		t.Fatalf("body = %q, want ok", body)
	}
}

func TestUsageAccessControlRateLimitsRepeatedFailures(t *testing.T) {
	control, err := newUsageAccessControl(testUsageAuthConfig("secret"))
	if err != nil {
		t.Fatalf("newUsageAccessControl error: %v", err)
	}

	now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	control.failures.now = func() time.Time { return now }

	handler := control.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < usageAuthFailureLimit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.9:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", i+1, rec.Code, http.StatusUnauthorized)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	now = now.Add(usageAuthBlockDuration + time.Second)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	req.SetBasicAuth("askplanner", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status after cooldown = %d, want %d", rec.Code, http.StatusOK)
	}
}

func testUsageAuthConfig(password string) *config.Config {
	sum := sha256.Sum256([]byte(password))
	return &config.Config{
		UsageAuthUsername:       "askplanner",
		UsageAuthPasswordSHA256: hex.EncodeToString(sum[:]),
		UsageAuthRealm:          "askplanner dashboard - contact guojiangtao for access",
	}
}
