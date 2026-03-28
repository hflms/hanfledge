package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tr := setupTranslator(t) // Reuse setupTranslator from i18n_test.go

	tests := []struct {
		name           string
		queryLang      string
		acceptLang     string
		expectedLocale Locale
	}{
		{
			name:           "Priority 1: Query parameter Exact Match",
			queryLang:      "en-US",
			acceptLang:     "zh-CN",
			expectedLocale: LocaleEnUS,
		},
		{
			name:           "Priority 1: Query parameter Invalid falls back to Header",
			queryLang:      "invalid",
			acceptLang:     "en-US",
			expectedLocale: LocaleEnUS, // tr.ParseLocale falls back to en-US based on header
		},
		{
			name:           "Priority 2: Accept-Language Header Match",
			queryLang:      "",
			acceptLang:     "en-US,en;q=0.9",
			expectedLocale: LocaleEnUS,
		},
		{
			name:           "Priority 2: Accept-Language Header Fallback",
			queryLang:      "",
			acceptLang:     "fr-FR",
			expectedLocale: LocaleZhCN, // Default fallback
		},
		{
			name:           "No Language specified, default fallback",
			queryLang:      "",
			acceptLang:     "",
			expectedLocale: LocaleZhCN,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			_, r := gin.CreateTestContext(w)

			r.Use(Middleware(tr))
			r.GET("/test", func(c *gin.Context) {
				locale := GetLocale(c)
				if locale != tc.expectedLocale {
					t.Errorf("Expected locale %q, got %q", tc.expectedLocale, locale)
				}
				c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tc.queryLang != "" {
				q := req.URL.Query()
				q.Add("lang", tc.queryLang)
				req.URL.RawQuery = q.Encode()
			}
			if tc.acceptLang != "" {
				req.Header.Add("Accept-Language", tc.acceptLang)
			}

			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

func TestGetLocale(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupContext   func(*gin.Context)
		expectedLocale Locale
	}{
		{
			name: "Locale exists in context",
			setupContext: func(c *gin.Context) {
				c.Set(ContextKey, LocaleEnUS)
			},
			expectedLocale: LocaleEnUS,
		},
		{
			name: "Locale missing in context",
			setupContext: func(c *gin.Context) {
				// Do not set anything
			},
			expectedLocale: DefaultLocale,
		},
		{
			name: "Locale in context has wrong type",
			setupContext: func(c *gin.Context) {
				c.Set(ContextKey, "en-US") // string instead of Locale type
			},
			expectedLocale: DefaultLocale,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			tc.setupContext(c)

			locale := GetLocale(c)
			if locale != tc.expectedLocale {
				t.Errorf("Expected locale %q, got %q", tc.expectedLocale, locale)
			}
		})
	}
}
