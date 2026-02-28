package i18n

import (
	"github.com/gin-gonic/gin"
)

// ContextKey is the gin context key for the current locale.
const ContextKey = "locale"

// Middleware returns a Gin middleware that detects the request locale
// from the "lang" query parameter or Accept-Language header.
func Middleware(translator *Translator) gin.HandlerFunc {
	return func(c *gin.Context) {
		var locale Locale

		// Priority 1: Query parameter ?lang=en-US
		if lang := c.Query("lang"); lang != "" {
			locale = Locale(lang)
			if !translator.HasLocale(locale) {
				locale = ""
			}
		}

		// Priority 2: Accept-Language header
		if locale == "" {
			locale = translator.ParseLocale(c.GetHeader("Accept-Language"))
		}

		// Store in context
		c.Set(ContextKey, locale)
		c.Next()
	}
}

// GetLocale retrieves the locale from the Gin context.
func GetLocale(c *gin.Context) Locale {
	if val, exists := c.Get(ContextKey); exists {
		if locale, ok := val.(Locale); ok {
			return locale
		}
	}
	return DefaultLocale
}
