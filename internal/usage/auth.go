package usage

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"lab/askplanner/internal/config"
)

const (
	usageAuthFailureWindow = time.Minute
	usageAuthFailureLimit  = 6
	usageAuthBlockDuration = 5 * time.Minute
	usageAuthEntryTTL      = 15 * time.Minute
)

type usageAccessControl struct {
	usernameHash [32]byte
	passwordHash [32]byte
	realm        string
	failures     *usageAuthFailures
}

type usageAuthFailures struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]usageAuthFailure
}

type usageAuthFailure struct {
	firstFailure time.Time
	lastSeen     time.Time
	blockedUntil time.Time
	count        int
}

func newUsageAccessControl(cfg *config.Config) (*usageAccessControl, error) {
	if cfg == nil || strings.TrimSpace(cfg.UsageAuthPasswordSHA256) == "" {
		return nil, nil
	}

	passwordHashBytes, err := hex.DecodeString(strings.TrimSpace(cfg.UsageAuthPasswordSHA256))
	if err != nil {
		return nil, fmt.Errorf("decode usage auth password hash: %w", err)
	}
	if len(passwordHashBytes) != sha256.Size {
		return nil, fmt.Errorf("usage auth password hash must be %d bytes", sha256.Size)
	}

	username := strings.TrimSpace(cfg.UsageAuthUsername)
	if username == "" {
		username = "askplanner"
	}
	realm := strings.TrimSpace(cfg.UsageAuthRealm)
	if realm == "" {
		realm = "askplanner dashboard - contact guojiangtao for access"
	}

	control := &usageAccessControl{
		usernameHash: sha256.Sum256([]byte(username)),
		realm:        realm,
		failures: &usageAuthFailures{
			now:     time.Now,
			entries: make(map[string]usageAuthFailure),
		},
	}
	copy(control.passwordHash[:], passwordHashBytes)
	return control, nil
}

func (a *usageAccessControl) Wrap(next http.Handler) http.Handler {
	if a == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := requestClientIP(r)
		if a.failures.IsBlocked(clientIP) {
			writeAuthRateLimited(w)
			return
		}

		username, password, ok := r.BasicAuth()
		if ok && compareSHA256(a.usernameHash, username) && compareSHA256(a.passwordHash, password) {
			a.failures.Reset(clientIP)
			next.ServeHTTP(w, r)
			return
		}

		if blocked := a.failures.RecordFailure(clientIP); blocked {
			log.Printf("[usage] auth rate limited ip=%s", clientIP)
			writeAuthRateLimited(w)
			return
		}
		writeAuthChallenge(w, a.realm)
	})
}

func compareSHA256(expected [32]byte, value string) bool {
	actual := sha256.Sum256([]byte(value))
	return subtle.ConstantTimeCompare(expected[:], actual[:]) == 1
}

func writeAuthChallenge(w http.ResponseWriter, realm string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm=%q, charset="UTF-8"`, realm))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.Error(w, "authentication required; contact guojiangtao for access\n", http.StatusUnauthorized)
}

func writeAuthRateLimited(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Retry-After", "300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.Error(w, "too many authentication attempts\n", http.StatusTooManyRequests)
}

func requestClientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		if ip := net.ParseIP(forwarded); ip != nil {
			return ip.String()
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-Ip")); realIP != "" {
		if ip := net.ParseIP(realIP); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (f *usageAuthFailures) IsBlocked(ip string) bool {
	if f == nil || ip == "" {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	now := f.now()
	f.cleanupLocked(now)
	entry, ok := f.entries[ip]
	return ok && now.Before(entry.blockedUntil)
}

func (f *usageAuthFailures) RecordFailure(ip string) bool {
	if f == nil || ip == "" {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	now := f.now()
	f.cleanupLocked(now)
	entry := f.entries[ip]
	if now.Before(entry.blockedUntil) {
		entry.lastSeen = now
		f.entries[ip] = entry
		return true
	}
	if entry.firstFailure.IsZero() || now.Sub(entry.firstFailure) > usageAuthFailureWindow {
		entry.firstFailure = now
		entry.count = 0
	}
	entry.count++
	entry.lastSeen = now
	if entry.count > usageAuthFailureLimit {
		entry.blockedUntil = now.Add(usageAuthBlockDuration)
		f.entries[ip] = entry
		return true
	}
	f.entries[ip] = entry
	return false
}

func (f *usageAuthFailures) Reset(ip string) {
	if f == nil || ip == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.entries, ip)
}

func (f *usageAuthFailures) cleanupLocked(now time.Time) {
	for ip, entry := range f.entries {
		if now.Sub(entry.lastSeen) > usageAuthEntryTTL && !now.Before(entry.blockedUntil) {
			delete(f.entries, ip)
		}
	}
}
