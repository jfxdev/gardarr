package middlewares

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/pkg/env"
)

// TrustedOrigin rejects browser write requests that do not originate from a
// configured Gardarr origin. It is intended to run after SessionMiddleware on
// cookie-authenticated routes.
func TrustedOrigin() gin.HandlerFunc {
	trusted := trustedOrigins()
	return func(c *gin.Context) {
		origin := normalizeOrigin(c.GetHeader("Origin"))
		if origin == "" || !trusted[origin] {
			c.JSON(http.StatusForbidden, gin.H{"error": "Untrusted request origin"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func trustedOrigins() map[string]bool {
	origins := make(map[string]bool)
	add := func(value string) {
		if origin := normalizeOrigin(value); origin != "" {
			origins[origin] = true
		}
	}
	appURL := env.Get(constants.AppURLEnv).Value()
	if appURL == "" {
		appURL = "http://localhost:" + env.Get(constants.AppPortEnv).Default("3200").Value()
	}
	add(appURL)
	parsed, err := url.Parse(appURL)
	if err == nil && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1") {
		add("http://localhost:3500")
	}
	for _, domain := range strings.Split(env.Get(constants.AppDomainsEnv).Default("").Value(), ",") {
		add(strings.TrimSpace(domain))
	}
	return origins
}

func normalizeOrigin(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
}
