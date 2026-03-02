package model

import "time"

// ============================
// 文档切片与向量存储模型
// ============================

// Document 上传的教材文档记录。
type Document struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CourseID  uint      `gorm:"not null;index" json:"course_id"`
	FileName  string    `gorm:"size:500;not null" json:"file_name"`
	FilePath  string    `gorm:"size:1000;not null" json:"file_path"`
	FileSize  int64     `json:"file_size"`
	MimeType  string    `gorm:"size:100" json:"mime_type"`
	Status    DocStatus `gorm:"size:20;default:uploaded" json:"status"` // uploaded, processing, completed, failed
	PageCount int       `json:"page_count"`
	ErrorMessage string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Course Course          `gorm:"foreignKey:CourseID" json:"-"`
	Chunks []DocumentChunk `gorm:"foreignKey:DocumentID" json:"chunks,omitempty"`
}

// DocStatus 文档处理状态。
type DocStatus string

const (
	DocStatusUploaded   DocStatus = "uploaded"
	DocStatusProcessing DocStatus = "processing"
	DocStatusCompleted  DocStatus = "completed"
	DocStatusFailed     DocStatus = "failed"
)

// DocumentChunk 文档切片（含向量嵌入）。
type DocumentChunk struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DocumentID uint      `gorm:"not null;index" json:"document_id"`
	CourseID   uint      `gorm:"not null;index" json:"course_id"`
	ChunkIndex int       `gorm:"not null" json:"chunk_index"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	TokenCount int       `json:"token_count"`
	PageNumber int       `json:"page_number"`
	Embedding  string    `gorm:"type:vector(1024);default:NULL" json:"-"` // pgvector 1024-dim for bge-m3
	CreatedAt  time.Time `json:"created_at"`
}
