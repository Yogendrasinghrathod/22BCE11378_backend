package models

import (
	"time"
)

type File struct {
    ID           string    `db:"id"`
    UserID       string    `db:"user_id"`
    Name         string    `db:"name"`
    OriginalName string    `db:"original_name"`
    Size         int64     `db:"size"`
    MimeType     string    `db:"mime_type"`
    StoragePath  string    `db:"storage_path"`
    StorageType  string    `db:"storage_type"` // "s3" or "local"
    URL          string    `db:"url"`          // Changed from PublicURL to URL
    IsPublic     bool      `db:"is_public"`
    UploadedAt   time.Time `db:"uploaded_at"`
    UpdatedAt    time.Time `db:"updated_at"`
}


type FilePermission struct {
	FileID    string `db:"file_id"`
	UserID    string `db:"user_id"`
	CanView   bool   `db:"can_view"`
	CanEdit   bool   `db:"can_edit"`
	CanShare  bool   `db:"can_share"`
	GrantedBy string `db:"granted_by"`
	GrantedAt time.Time `db:"granted_at"`
}

type FileVersion struct {
	ID           string    `db:"id"`
	FileID       string    `db:"file_id"`
	Version      int       `db:"version"`
	StoragePath  string    `db:"storage_path"`
	Size         int64     `db:"size"`
	CreatedBy    string    `db:"created_by"`
	CreatedAt    time.Time `db:"created_at"`
}