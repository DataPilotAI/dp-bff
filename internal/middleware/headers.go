package middleware

import (
	"net/http"
)

// Matches what the frontend actually loads: its own scripts, styles and API, nothing external.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

const hstsValue = "max-age=63072000; includeSubDomains"

// SecurityHeaders sets browser hardening headers on every response.
// HSTS is only sent on direct TLS; behind a TLS-terminating proxy, set it there.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", contentSecurityPolicy)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		header.Set("Cross-Origin-Opener-Policy", "same-origin")
		if r.TLS != nil {
			header.Set("Strict-Transport-Security", hstsValue)
		}
		next.ServeHTTP(w, r)
	})
}
