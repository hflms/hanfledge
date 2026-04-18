package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/infrastructure/i18n"
)

// APIError represents a standardized API error response.
type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Error codes
const (
	ErrCodeUnauthorized     = "UNAUTHORIZED"
	ErrCodeForbidden        = "FORBIDDEN"
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeBadRequest       = "BAD_REQUEST"
	ErrCodeConflict         = "CONFLICT"
	ErrCodeInternalError    = "INTERNAL_ERROR"
	ErrCodeValidationFailed = "VALIDATION_FAILED"
)

// RespondError sends a standardized error response with i18n support.
func RespondError(c *gin.Context, status int, code string, details ...interface{}) {
	locale := i18n.GetLocale(c)
	translator := c.MustGet("translator").(*i18n.Translator)

	message := translator.T(locale, code)
	if message == code {
		// Fallback if translation not found
		message = defaultErrorMessage(code)
	}

	var detail interface{}
	if len(details) > 0 {
		detail = details[0]
	}

	c.JSON(status, APIError{
		Code:    code,
		Message: message,
		Details: detail,
	})
}

// defaultErrorMessage provides fallback messages when i18n is unavailable.
func defaultErrorMessage(code string) string {
	messages := map[string]string{
		ErrCodeUnauthorized:     "未授权",
		ErrCodeForbidden:        "无权限",
		ErrCodeNotFound:         "资源不存在",
		ErrCodeBadRequest:       "请求参数错误",
		ErrCodeConflict:         "资源冲突",
		ErrCodeInternalError:    "服务器内部错误",
		ErrCodeValidationFailed: "数据验证失败",
	}
	if msg, ok := messages[code]; ok {
		return msg
	}
	return "未知错误"
}

// RespondSuccess sends a standardized success response.
func RespondSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}
