package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// -- Pagination -----------------------------------------------

// ErrorResponse is the standard error envelope for all API endpoints.
type ErrorResponse struct {
	Error string `json:"error" example:"操作失败"`
}

// PaginationParams holds parsed pagination query parameters.
type PaginationParams struct {
	Page   int `json:"page"`
	Limit  int `json:"limit"`
	Offset int `json:"-"`
}

// PaginatedResponse wraps a list result with pagination metadata.
type PaginatedResponse struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

// DefaultPage is the default page number when not specified.
const DefaultPage = 1

// DefaultLimit is the default page size when not specified.
const DefaultLimit = 20

// MaxLimit is the maximum allowed page size.
const MaxLimit = 100

// ParsePagination extracts page/limit from query parameters with defaults.
// Callers that omit page/limit will get DefaultPage and DefaultLimit,
// maintaining backward compatibility.
func ParsePagination(c *gin.Context) PaginationParams {
	page, _ := strconv.Atoi(c.DefaultQuery("page", strconv.Itoa(DefaultPage)))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(DefaultLimit)))

	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 || limit > MaxLimit {
		limit = DefaultLimit
	}

	return PaginationParams{
		Page:   page,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
}

// NewPaginatedResponse creates a PaginatedResponse.
func NewPaginatedResponse(items interface{}, total int64, p PaginationParams) PaginatedResponse {
	return PaginatedResponse{
		Items: items,
		Total: total,
		Page:  p.Page,
		Limit: p.Limit,
	}
}
