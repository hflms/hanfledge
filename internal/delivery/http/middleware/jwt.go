package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims defines the custom claims in the JWT token.
type JWTClaims struct {
	UserID      uint   `json:"user_id"`
	Phone       string `json:"phone"`
	DisplayName string `json:"display_name"`
	jwt.RegisteredClaims
}

// JWTAuth returns a Gin middleware that validates JWT tokens.
// On success, it injects user info into the Gin context.
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// WebSocket 连接无法设置 HTTP 头，回退到 query 参数 ?token=xxx
			if tokenQuery := c.Query("token"); tokenQuery != "" {
				authHeader = "Bearer " + tokenQuery
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "缺少认证令牌",
				})
				return
			}
		}

		// Expect "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "认证令牌格式错误",
			})
			return
		}

		tokenStr := parts[1]

		// Parse and validate token
		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "认证令牌无效或已过期",
			})
			return
		}

		// Inject user info into context
		c.Set("user_id", claims.UserID)
		c.Set("phone", claims.Phone)
		c.Set("display_name", claims.DisplayName)

		c.Next()
	}
}

// GetUserID extracts the authenticated user ID from the Gin context.
// Returns 0 if the value is missing or has an unexpected type.
func GetUserID(c *gin.Context) uint {
	val, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	switch id := val.(type) {
	case uint:
		return id
	case float64:
		// JWT numeric claims may be parsed as float64 in some paths
		return uint(id)
	default:
		return 0
	}
}
