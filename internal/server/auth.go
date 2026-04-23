package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

const sessionCookieName = "kb-session"

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/healthz" || path == "/login" || strings.HasPrefix(path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(path, "/git/") {
			_, pass, ok := r.BasicAuth()
			if ok && subtle.ConstantTimeCompare([]byte(pass), []byte(s.token)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="kb"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, "Bearer ")), []byte(s.token)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		cookie, err := r.Cookie(sessionCookieName)
		if err == nil && verifyToken(cookie.Value, s.token) {
			next.ServeHTTP(w, r)
			return
		}

		if wantsJSON(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
}

func signToken(token string) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte("kb-session"))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyToken(cookieValue, token string) bool {
	expected := signToken(token)
	return hmac.Equal([]byte(cookieValue), []byte(expected))
}

func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") != ""
}
