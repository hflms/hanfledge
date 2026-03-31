package handler

import (
	"strconv"
	"testing"
)

// ============================
// Pagination Unit Tests
// ============================

// -- ParsePagination Tests ------------------------------------

func TestParsePagination_Defaults(t *testing.T) {
	w, c := newTestContextWithQuery("GET", "/test", 0)
	_ = w

	p := ParsePagination(c)
	if p.Page != DefaultPage {
		t.Errorf("Page = %d, want %d", p.Page, DefaultPage)
	}
	if p.Limit != DefaultLimit {
		t.Errorf("Limit = %d, want %d", p.Limit, DefaultLimit)
	}
	if p.Offset != 0 {
		t.Errorf("Offset = %d, want 0", p.Offset)
	}
}

func TestParsePagination_CustomValues(t *testing.T) {
	w, c := newTestContextWithQuery("GET", "/test?page=3&limit=10", 0)
	_ = w

	p := ParsePagination(c)
	if p.Page != 3 {
		t.Errorf("Page = %d, want 3", p.Page)
	}
	if p.Limit != 10 {
		t.Errorf("Limit = %d, want 10", p.Limit)
	}
	if p.Offset != 20 {
		t.Errorf("Offset = %d, want 20 ((3-1)*10)", p.Offset)
	}
}

func TestParsePagination_PageOne(t *testing.T) {
	w, c := newTestContextWithQuery("GET", "/test?page=1&limit=5", 0)
	_ = w

	p := ParsePagination(c)
	if p.Offset != 0 {
		t.Errorf("Offset = %d, want 0 for page 1", p.Offset)
	}
}

func TestParsePagination_NegativePage(t *testing.T) {
	w, c := newTestContextWithQuery("GET", "/test?page=-1&limit=10", 0)
	_ = w

	p := ParsePagination(c)
	if p.Page != DefaultPage {
		t.Errorf("Page = %d, want %d for negative page", p.Page, DefaultPage)
	}
}

func TestParsePagination_ZeroPage(t *testing.T) {
	w, c := newTestContextWithQuery("GET", "/test?page=0&limit=10", 0)
	_ = w

	p := ParsePagination(c)
	if p.Page != DefaultPage {
		t.Errorf("Page = %d, want %d for zero page", p.Page, DefaultPage)
	}
}

func TestParsePagination_ZeroLimit(t *testing.T) {
	w, c := newTestContextWithQuery("GET", "/test?page=1&limit=0", 0)
	_ = w

	p := ParsePagination(c)
	if p.Limit != DefaultLimit {
		t.Errorf("Limit = %d, want %d for zero limit", p.Limit, DefaultLimit)
	}
}

func TestParsePagination_NegativeLimit(t *testing.T) {
	w, c := newTestContextWithQuery("GET", "/test?page=1&limit=-5", 0)
	_ = w

	p := ParsePagination(c)
	if p.Limit != DefaultLimit {
		t.Errorf("Limit = %d, want %d for negative limit", p.Limit, DefaultLimit)
	}
}

func TestParsePagination_LimitExceedsMax(t *testing.T) {
	w, c := newTestContextWithQuery("GET", "/test?page=1&limit=200", 0)
	_ = w

	p := ParsePagination(c)
	if p.Limit != DefaultLimit {
		t.Errorf("Limit = %d, want %d for limit exceeding MaxLimit", p.Limit, DefaultLimit)
	}
}

func TestParsePagination_LimitAtMax(t *testing.T) {
	w, c := newTestContextWithQuery("GET", "/test?page=1&limit=100", 0)
	_ = w

	p := ParsePagination(c)
	if p.Limit != MaxLimit {
		t.Errorf("Limit = %d, want %d for limit at MaxLimit", p.Limit, MaxLimit)
	}
}

func TestParsePagination_InvalidPageString(t *testing.T) {
	w, c := newTestContextWithQuery("GET", "/test?page=abc&limit=10", 0)
	_ = w

	p := ParsePagination(c)
	// strconv.Atoi("abc") returns 0, which is < 1, so page defaults
	if p.Page != DefaultPage {
		t.Errorf("Page = %d, want %d for invalid page string", p.Page, DefaultPage)
	}
}

func TestParsePagination_InvalidLimitString(t *testing.T) {
	w, c := newTestContextWithQuery("GET", "/test?page=1&limit=abc", 0)
	_ = w

	p := ParsePagination(c)
	// strconv.Atoi("abc") returns 0, which is < 1, so limit defaults
	if p.Limit != DefaultLimit {
		t.Errorf("Limit = %d, want %d for invalid limit string", p.Limit, DefaultLimit)
	}
}

func TestParsePagination_OffsetCalculation(t *testing.T) {
	tests := []struct {
		name           string
		page, limit    int
		expectedOffset int
	}{
		{"page1_limit20", 1, 20, 0},
		{"page2_limit20", 2, 20, 20},
		{"page3_limit10", 3, 10, 20},
		{"page5_limit50", 5, 50, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := "/test?page=" + itoa(tt.page) + "&limit=" + itoa(tt.limit)
			w, c := newTestContextWithQuery("GET", query, 0)
			_ = w

			p := ParsePagination(c)
			if p.Offset != tt.expectedOffset {
				t.Errorf("Offset = %d, want %d", p.Offset, tt.expectedOffset)
			}
		})
	}
}

// -- NewPaginatedResponse Tests -------------------------------

func TestNewPaginatedResponse_BasicFields(t *testing.T) {
	items := []string{"a", "b", "c"}
	params := PaginationParams{Page: 1, Limit: 20, Offset: 0}

	resp := NewPaginatedResponse(items, 3, params)
	if resp.Total != 3 {
		t.Errorf("Total = %d, want 3", resp.Total)
	}
	if resp.Page != 1 {
		t.Errorf("Page = %d, want 1", resp.Page)
	}
	if resp.Limit != 20 {
		t.Errorf("Limit = %d, want 20", resp.Limit)
	}
}

func TestNewPaginatedResponse_NilItems(t *testing.T) {
	params := PaginationParams{Page: 1, Limit: 10, Offset: 0}
	resp := NewPaginatedResponse(nil, 0, params)
	if resp.Items != nil {
		t.Error("expected nil items")
	}
	if resp.Total != 0 {
		t.Errorf("Total = %d, want 0", resp.Total)
	}
}

func TestNewPaginatedResponse_LargeTotal(t *testing.T) {
	items := []int{1}
	params := PaginationParams{Page: 500, Limit: 10, Offset: 4990}

	resp := NewPaginatedResponse(items, 5000, params)
	if resp.Total != 5000 {
		t.Errorf("Total = %d, want 5000", resp.Total)
	}
	if resp.Page != 500 {
		t.Errorf("Page = %d, want 500", resp.Page)
	}
}

// -- Constants Tests ------------------------------------------

func TestPaginationConstants(t *testing.T) {
	if DefaultPage != 1 {
		t.Errorf("DefaultPage = %d, want 1", DefaultPage)
	}
	if DefaultLimit != 20 {
		t.Errorf("DefaultLimit = %d, want 20", DefaultLimit)
	}
	if MaxLimit != 100 {
		t.Errorf("MaxLimit = %d, want 100", MaxLimit)
	}
}

// -- Helper ---------------------------------------------------

func itoa(n int) string {
	return strconv.Itoa(n)
}
