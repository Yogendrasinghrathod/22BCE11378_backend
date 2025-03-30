package file

import (
	"context"
	// "database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"
	// "log"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type FileHandler struct {
	storageDir  string
	db          *sqlx.DB
	redisClient *redis.Client
}

func NewFileHandler(storageDir string, db *sqlx.DB, redisClient *redis.Client) *FileHandler {
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		panic("failed to create storage directory: " + err.Error())
	}

	return &FileHandler{
		storageDir:  storageDir,
		db:          db,
		redisClient: redisClient,
	}
}

func (h *FileHandler) Upload(c *gin.Context) {
	// 1. Get user ID from auth middleware
	userID, err := uuid.Parse(c.MustGet("userID").(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// 2. Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// 3. Validate file size
	if file.Size == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File cannot be empty"})
		return
	}

	// 4. Create user directory if not exists
	userDir := filepath.Join(h.storageDir, userID.String())
	if err := os.MkdirAll(userDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create user directory",
			"details": err.Error(),
		})
		return
	}

	// 5. Generate unique filename and paths
	fileExt := filepath.Ext(file.Filename)
	newFilename := uuid.New().String() + fileExt
	storagePath := filepath.Join(userID.String(), newFilename)
	fullPath := filepath.Join(h.storageDir, storagePath)

	// 6. Save the file
	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to save file",
			"details": err.Error(),
		})
		return
	}

	// 7. Determine MIME type
	mimeType := "application/octet-stream"
	if mimes := file.Header["Content-Type"]; len(mimes) > 0 {
		mimeType = mimes[0]
	}

	// 8. Store metadata in database
	tx, err := h.db.Beginx()
	if err != nil {
		os.Remove(fullPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Generate file ID
	fileID := uuid.New()
	url := fmt.Sprintf("/files/%s", fileID)

	_, err = tx.Exec(`
		INSERT INTO files (
			id, user_id, name, original_name, storage_path,
			size, mime_type, is_public, url
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		fileID,
		userID,
		newFilename,
		file.Filename,
		storagePath,
		file.Size,
		mimeType,
		false,
		url,
	)
	if err != nil {
		os.Remove(fullPath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":          "Failed to store file metadata",
			"database_error": err.Error(),
		})
		return
	}

	if err := tx.Commit(); err != nil {
		os.Remove(fullPath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Transaction failed",
			"commit_error": err.Error(),
		})
		return
	}

	// 9. Success response (matches your desired format)
	c.JSON(http.StatusOK, gin.H{
		"file": gin.H{
			"id":         fileID,
			"user_id":    userID,
			"name":       newFilename,
			"path":       storagePath,
			"size":       file.Size,
			"mime_type":  mimeType,
			"created_at": time.Now().Format(time.RFC3339),
			"is_public":  false,
		},
		"message": "File uploaded successfully",
		"url":     url,
	})
}

func (h *FileHandler) GetUserFiles(c *gin.Context) {
	// 1. Get user ID from auth middleware with proper UUID parsing
	userIDString := c.MustGet("userID").(string)
	userID, err := uuid.Parse(userIDString)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	ctx := context.Background()

	// 2. Use consistent cache key format
	cacheKey := "user_files:" + userID.String()
	val, err := h.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		c.Data(http.StatusOK, "application/json", []byte(val))
		return
	}

	// 3. Define response structure including all needed fields from Upload
	type fileResponse struct {
		ID           uuid.UUID `json:"id" db:"id"`
		Name         string    `json:"name" db:"name"`
		OriginalName string    `json:"filename" db:"original_name"`
		Size         int64     `json:"size" db:"size"`
		MimeType     string    `json:"mime_type" db:"mime_type"`
		StoragePath  string    `json:"path" db:"storage_path"`
		CreatedAt    time.Time `json:"created_at" db:"created_at"`
	}

	// 4. Query the database with the UUID parameter and select all required fields
	var files []fileResponse
	err = h.db.Select(&files, `
        SELECT id, name, original_name, size, mime_type, storage_path, created_at
        FROM files
        WHERE user_id = $1
        ORDER BY created_at DESC`, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get files"})
		return
	}

	jsonData, err := json.Marshal(files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal files"})
		return
	}

	h.redisClient.Set(ctx, cacheKey, jsonData, 5*time.Minute)
	c.Data(http.StatusOK, "application/json", jsonData)
}

func (h *FileHandler) CreateShareLink(c *gin.Context) {
	userID := c.GetString("userID")
	fileID := c.Param("file_id")

	var count int
	err := h.db.Get(&count, `
		SELECT COUNT(*) 
		FROM files 
		WHERE id = $1 AND user_id = $2`, fileID, userID)

	if err != nil || count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	token := uuid.New().String()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	_, err = h.db.Exec(`
		INSERT INTO file_shares (file_id, token, expires_at)
		VALUES ($1, $2, $3)`, fileID, token, expiresAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create share link"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"share_url":  "/share/" + token,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

func (h *FileHandler) ServeSharedFile(c *gin.Context) {
	token := c.Param("token")
	ctx := context.Background()

	cacheKey := "file_share:" + token
	cachedPath, err := h.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		c.File(filepath.Join(h.storageDir, cachedPath))
		return
	}

	var file struct {
		StoragePath string    `db:"storage_path"`
		Name        string    `db:"name"`
		MimeType    string    `db:"mime_type"`
		ExpiresAt   time.Time `db:"expires_at"`
	}

	err = h.db.Get(&file, `
		SELECT f.storage_path, f.name, f.mime_type, s.expires_at
		FROM file_shares s
		JOIN files f ON s.file_id = f.id
		WHERE s.token = $1`, token)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid share link"})
		return
	}

	if !file.ExpiresAt.IsZero() && time.Now().After(file.ExpiresAt) {
		c.JSON(http.StatusGone, gin.H{"error": "Share link expired"})
		return
	}

	h.redisClient.Set(ctx, cacheKey, file.StoragePath, time.Hour)
	c.Header("Content-Disposition", "inline; filename=\""+file.Name+"\"")
	c.Header("Content-Type", file.MimeType)
	c.File(filepath.Join(h.storageDir, file.StoragePath))
}

func (h *FileHandler) Download(c *gin.Context) {
	userID := c.GetString("userID")
	fileID := c.Param("file_id")

	var file struct {
		StoragePath string `db:"storage_path"`
		Name        string `db:"name"`
		MimeType    string `db:"mime_type"`
	}

	err := h.db.Get(&file, `
		SELECT storage_path, name, mime_type 
		FROM files 
		WHERE id = $1 AND user_id = $2`, fileID, userID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	fullPath := filepath.Join(h.storageDir, file.StoragePath)
	c.Header("Content-Disposition", "attachment; filename=\""+file.Name+"\"")
	c.Header("Content-Type", file.MimeType)
	c.File(fullPath)
}
