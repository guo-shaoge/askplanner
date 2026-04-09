package usage

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html/template"
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm=%q, charset="UTF-8"`, realm))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(renderUsageAuthPage("Authentication Required", "Reload this page to open the browser login prompt. Contact guojiangtao for access.")))
}

func writeAuthRateLimited(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Retry-After", "300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(renderUsageAuthPage("Too Many Attempts", "Too many authentication attempts were detected. Wait a few minutes, then try again.")))
}

func renderUsageAuthPage(title, message string) string {
	const page = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f3efe5;
      --panel: rgba(255,255,255,0.92);
      --ink: #16212b;
      --muted: #5f6c77;
      --line: rgba(22,33,43,0.1);
      --accent: #b44a18;
      --shadow: 0 18px 60px rgba(42, 35, 23, 0.12);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 24px;
      font-family: "Iowan Old Style", "Palatino Linotype", "Book Antiqua", Georgia, serif;
      color: var(--ink);
      background:
        radial-gradient(circle at top left, rgba(224, 140, 76, 0.26), transparent 28%),
        radial-gradient(circle at top right, rgba(14, 109, 115, 0.16), transparent 24%),
        linear-gradient(180deg, #faf7f1 0%, var(--bg) 100%);
    }
    main {
      width: min(100%, 560px);
      padding: 28px 28px 24px;
      border: 1px solid var(--line);
      border-radius: 24px;
      background: var(--panel);
      box-shadow: var(--shadow);
    }
    .kicker {
      margin: 0 0 10px;
      color: var(--accent);
      font-size: 12px;
      letter-spacing: 0.14em;
      text-transform: uppercase;
      font-weight: 700;
    }
    h1 {
      margin: 0 0 12px;
      font-size: clamp(28px, 5vw, 44px);
      line-height: 0.98;
    }
    p {
      margin: 0;
      color: var(--muted);
      font-size: 16px;
      line-height: 1.55;
    }
  </style>
</head>
<body>
  <main>
    <p class="kicker">askplanner dashboard</p>
    <h1>{{.Title}}</h1>
    <p>{{.Message}}</p>
  </main>
</body>
</html>
`

	var out strings.Builder
	tpl := template.Must(template.New("usage_auth_page").Parse(page))
	_ = tpl.Execute(&out, struct {
		Title   string
		Message string
	}{
		Title:   title,
		Message: message,
	})
	return out.String()
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
