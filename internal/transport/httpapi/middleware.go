package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) requireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !tokenMatches(r.Header.Get("Authorization"), "Bearer "+s.mcpToken) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.validSessionCookie(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			origin := r.Header.Get("Origin")
			if origin != "" && !sameOrigin(origin, r.Host) {
				http.Error(w, "origin rejected", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) validSessionCookie(r *http.Request) bool {
	cookie, err := r.Cookie(browserSessionCookieName(r.Host))
	return err == nil && tokenMatches(cookie.Value, s.browserToken)
}

func browserSessionCookieName(host string) string {
	const name = "xlsx_session"
	// Cookie storage is shared across ports of the same host. Name each local
	// endpoint separately so its launch cannot replace another server's cookie.
	_, port, err := net.SplitHostPort(host)
	if err != nil {
		return name
	}
	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || number == 0 {
		return name
	}
	return name + "_" + strconv.FormatUint(number, 10)
}

func tokenMatches(got, want string) bool {
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func sameOrigin(origin, host string) bool {
	return origin == "http://"+host || origin == "https://"+host
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		// The workspace embeds only authenticated, same-origin document editors.
		if r.URL.Query().Get("embedded") == "1" && r.URL.Query().Get("document") != "" {
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		}
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		policy := "default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data: blob:; font-src 'self' data:; worker-src 'self' blob:"
		if r.URL.Query().Get("embedded") == "1" && r.URL.Query().Get("document") != "" {
			policy = strings.Replace(policy, "frame-ancestors 'none'", "frame-ancestors 'self'", 1)
		}
		w.Header().Set("Content-Security-Policy", policy)
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
