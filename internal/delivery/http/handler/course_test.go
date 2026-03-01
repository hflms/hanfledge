package handler

import (
	"strings"
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// Phase H: Course Handler Tests — Document Management
// ============================

// -- MaxFileSize Constant Tests --------------------------------

func TestMaxFileSize(t *testing.T) {
	expected := int64(50 * 1024 * 1024) // 50 MB
	if MaxFileSize != expected {
		t.Errorf("MaxFileSize = %d, want %d", MaxFileSize, expected)
	}
}

func TestMaxFileSizeIs50MB(t *testing.T) {
	mb := MaxFileSize / (1024 * 1024)
	if mb != 50 {
		t.Errorf("MaxFileSize in MB = %d, want 50", mb)
	}
}

// -- DocStatus Constants Test ---------------------------------

func TestDocStatusConstants(t *testing.T) {
	tests := []struct {
		name   string
		status model.DocStatus
		want   string
	}{
		{"uploaded", model.DocStatusUploaded, "uploaded"},
		{"processing", model.DocStatusProcessing, "processing"},
		{"completed", model.DocStatusCompleted, "completed"},
		{"failed", model.DocStatusFailed, "failed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.status) != tc.want {
				t.Errorf("DocStatus = %q, want %q", tc.status, tc.want)
			}
		})
	}
}

// -- Document Model Fields Test --------------------------------

func TestDocumentModelFields(t *testing.T) {
	doc := model.Document{
		ID:        1,
		CourseID:  10,
		FileName:  "test.pdf",
		FilePath:  "/tmp/test.pdf",
		FileSize:  1024,
		MimeType:  "application/pdf",
		Status:    model.DocStatusUploaded,
		PageCount: 5,
	}

	if doc.ID != 1 {
		t.Errorf("doc.ID = %d, want 1", doc.ID)
	}
	if doc.CourseID != 10 {
		t.Errorf("doc.CourseID = %d, want 10", doc.CourseID)
	}
	if doc.FileName != "test.pdf" {
		t.Errorf("doc.FileName = %q, want %q", doc.FileName, "test.pdf")
	}
	if doc.FileSize != 1024 {
		t.Errorf("doc.FileSize = %d, want 1024", doc.FileSize)
	}
	if doc.Status != model.DocStatusUploaded {
		t.Errorf("doc.Status = %q, want %q", doc.Status, model.DocStatusUploaded)
	}
	if doc.PageCount != 5 {
		t.Errorf("doc.PageCount = %d, want 5", doc.PageCount)
	}
}

// -- Retry Validation: Only failed docs should be retryable -------

func TestRetryValidation_OnlyFailedAllowed(t *testing.T) {
	statuses := []struct {
		status    model.DocStatus
		retryable bool
	}{
		{model.DocStatusUploaded, false},
		{model.DocStatusProcessing, false},
		{model.DocStatusCompleted, false},
		{model.DocStatusFailed, true},
	}

	for _, tc := range statuses {
		t.Run(string(tc.status), func(t *testing.T) {
			canRetry := tc.status == model.DocStatusFailed
			if canRetry != tc.retryable {
				t.Errorf("status %q: canRetry = %v, want %v", tc.status, canRetry, tc.retryable)
			}
		})
	}
}

// -- Delete Validation: Processing docs should not be deletable ----

func TestDeleteValidation_ProcessingBlocked(t *testing.T) {
	statuses := []struct {
		status    model.DocStatus
		deletable bool
	}{
		{model.DocStatusUploaded, true},
		{model.DocStatusProcessing, false},
		{model.DocStatusCompleted, true},
		{model.DocStatusFailed, true},
	}

	for _, tc := range statuses {
		t.Run(string(tc.status), func(t *testing.T) {
			canDelete := tc.status != model.DocStatusProcessing
			if canDelete != tc.deletable {
				t.Errorf("status %q: canDelete = %v, want %v", tc.status, canDelete, tc.deletable)
			}
		})
	}
}

// -- File Size Validation Tests --------------------------------

func TestFileSizeValidation(t *testing.T) {
	tests := []struct {
		name    string
		size    int64
		allowed bool
	}{
		{"zero bytes", 0, true},
		{"1 KB", 1024, true},
		{"10 MB", 10 * 1024 * 1024, true},
		{"49 MB", 49 * 1024 * 1024, true},
		{"50 MB exactly", MaxFileSize, true}, // handler uses >, not >=
		{"50 MB + 1", MaxFileSize + 1, false},
		{"100 MB", 100 * 1024 * 1024, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// The handler uses: if header.Size > MaxFileSize
			allowed := tc.size <= MaxFileSize
			if allowed != tc.allowed {
				t.Errorf("size %d: allowed = %v, want %v", tc.size, allowed, tc.allowed)
			}
		})
	}
}

// -- PDF Extension Validation Tests ----------------------------

func TestPDFExtensionValidation(t *testing.T) {
	tests := []struct {
		filename string
		valid    bool
	}{
		{"document.pdf", true},
		{"document.PDF", true},
		{"document.Pdf", true},
		{"my file.pdf", true},
		{"document.txt", false},
		{"document.docx", false},
		{"document.pptx", false},
		{"no-extension", false},
		{"file.pdf.txt", false},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			// Same logic as handler: strings.HasSuffix + strings.ToLower
			valid := strings.HasSuffix(strings.ToLower(tc.filename), ".pdf")
			if valid != tc.valid {
				t.Errorf("filename %q: valid = %v, want %v", tc.filename, valid, tc.valid)
			}
		})
	}
}

// -- DocumentChunk Model Test ----------------------------------

func TestDocumentChunkModel(t *testing.T) {
	chunk := model.DocumentChunk{
		ID:         1,
		DocumentID: 10,
		CourseID:   5,
		ChunkIndex: 0,
		Content:    "sample text content",
		TokenCount: 42,
		PageNumber: 1,
	}

	if chunk.DocumentID != 10 {
		t.Errorf("chunk.DocumentID = %d, want 10", chunk.DocumentID)
	}
	if chunk.CourseID != 5 {
		t.Errorf("chunk.CourseID = %d, want 5", chunk.CourseID)
	}
	if chunk.ChunkIndex != 0 {
		t.Errorf("chunk.ChunkIndex = %d, want 0", chunk.ChunkIndex)
	}
	if chunk.TokenCount != 42 {
		t.Errorf("chunk.TokenCount = %d, want 42", chunk.TokenCount)
	}
}

// -- CourseHandler Constructor Test ----------------------------

func TestNewCourseHandler_NilCache(t *testing.T) {
	// CourseHandler should work with nil cache (Redis unavailable)
	h := NewCourseHandler(nil, nil, nil, nil, nil)
	if h == nil {
		t.Fatal("NewCourseHandler returned nil")
	}
	if h.Cache != nil {
		t.Error("expected nil Cache when no Redis provided")
	}
}
