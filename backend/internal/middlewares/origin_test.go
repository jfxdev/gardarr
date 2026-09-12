package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jfxdev/gardarr/internal/constants"
)

func TestTrustedOriginRejectsCrossSiteWrites(t *testing.T) {
	t.Setenv(constants.AppURLEnv, "https://gardarr.example.test")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/write", TrustedOrigin(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, tc := range []struct {
		name   string
		origin string
		want   int
	}{
		{name: "trusted origin", origin: "https://gardarr.example.test", want: http.StatusNoContent},
		{name: "cross site", origin: "https://attacker.example.test", want: http.StatusForbidden},
		{name: "missing origin", want: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/write", nil)
			if tc.origin != "" {
				request.Header.Set("Origin", tc.origin)
			}
			writer := httptest.NewRecorder()
			router.ServeHTTP(writer, request)
			if writer.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", writer.Code, tc.want, writer.Body.String())
			}
		})
	}
}

func TestTrustedOriginAllowsDevelopmentFrontendForLocalBackend(t *testing.T) {
	t.Setenv(constants.AppURLEnv, "")
	t.Setenv(constants.AppPortEnv, "3501")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/write", TrustedOrigin(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodPost, "/write", nil)
	request.Header.Set("Origin", "http://localhost:3500")
	writer := httptest.NewRecorder()
	router.ServeHTTP(writer, request)
	if writer.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", writer.Code, http.StatusNoContent, writer.Body.String())
	}
}
