package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// RBAC returns a Gin middleware that checks if the authenticated user
// has at least one of the required roles.
// Must be used AFTER JWTAuth middleware.
func RBAC(db *gorm.DB, requiredRoles ...model.RoleName) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)

		// Query user's roles
		var userRoles []model.UserSchoolRole
		if err := db.Preload("Role").Where("user_id = ?", userID).Find(&userRoles).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "查询用户角色失败",
			})
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, ur := range userRoles {
			for _, required := range requiredRoles {
				if ur.Role.Name == required {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "权限不足，需要角色: " + joinRoleNames(requiredRoles),
			})
			return
		}

		// Inject user roles into context for downstream handlers
		c.Set("user_roles", userRoles)
		c.Next()
	}
}

// joinRoleNames joins role names with comma for error messages.
func joinRoleNames(roles []model.RoleName) string {
	result := ""
	for i, r := range roles {
		if i > 0 {
			result += ", "
		}
		result += string(r)
	}
	return result
}
