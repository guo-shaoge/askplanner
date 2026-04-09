package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestUsageAccessControlRedirectsUnauthorizedRequestsToLogin(t *testing.T) {
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

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := rec.Header().Get("Location"), "/login?next=%2Fhealthz"; got != want {
		t.Fatalf("location = %q, want %q", got, want)
	}
}

func TestUsageAccessControlServesLoginPage(t *testing.T) {
	control, err := newUsageAccessControl(testUsageAuthConfig("secret"))
	if err != nil {
		t.Fatalf("newUsageAccessControl error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/login?next=%2Fquestions", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	control.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("content-type = %q, want html", got)
	}
	if body := rec.Body.String(); !containsAll(body, "<!doctype html>", "Contact guojiangtao for the password", `value="/questions"`) {
		t.Fatalf("unexpected login page body: %q", body)
	}
}

func TestUsageAccessControlCreatesSessionCookieOnSuccessfulLogin(t *testing.T) {
	server, err := NewServer(&Collector{}, testUsageAuthConfig("secret"))
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}

	form := url.Values{
		"username": []string{"askplanner"},
		"password": []string{"secret"},
		"next":     []string{"/healthz"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := rec.Header().Get("Location"), "/healthz"; got != want {
		t.Fatalf("location = %q, want %q", got, want)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing session cookie")
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "ok\n" {
		t.Fatalf("body = %q, want ok", body)
	}
}

func TestUsageAccessControlStillAllowsBasicAuth(t *testing.T) {
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
}

func TestUsageAccessControlRateLimitsRepeatedLoginFailures(t *testing.T) {
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
		form := url.Values{
			"username": []string{"askplanner"},
			"password": []string{"wrong"},
		}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.RemoteAddr = "203.0.113.9:1234"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", i+1, rec.Code, http.StatusUnauthorized)
		}
	}

	form := url.Values{
		"username": []string{"askplanner"},
		"password": []string{"wrong"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.RemoteAddr = "203.0.113.9:1234"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("content-type = %q, want html", got)
	}

	now = now.Add(usageAuthBlockDuration + time.Second)
	form = url.Values{
		"username": []string{"askplanner"},
		"password": []string{"secret"},
	}
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.RemoteAddr = "203.0.113.9:1234"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status after cooldown = %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

func testUsageAuthConfig(password string) *config.Config {
	sum := sha256.Sum256([]byte(password))
	return &config.Config{
		UsageAuthUsername:       "askplanner",
		UsageAuthPasswordSHA256: hex.EncodeToString(sum[:]),
		UsageAuthRealm:          "askplanner dashboard - contact guojiangtao for access",
	}
}
