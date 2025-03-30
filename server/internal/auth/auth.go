package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db            *sqlx.DB
	jwtSecret     []byte
	tokenDuration time.Duration
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type User struct {
	ID           string `db:"id"`
	Email        string `db:"email"`
	PasswordHash string `db:"password_hash"`
}

func NewAuthHandler(db *sqlx.DB) (*AuthHandler, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 characters long")
	}

	expHours := 24 // Default to 24 hours
	if expStr := os.Getenv("JWT_EXPIRATION_HOURS"); expStr != "" {
		var err error
		expHours, err = strconv.Atoi(expStr)
		if err != nil || expHours <= 0 {
			return nil, errors.New("JWT_EXPIRATION_HOURS must be a positive integer")
		}
	}

	return &AuthHandler{
		db:            db,
		jwtSecret:     []byte(jwtSecret),
		tokenDuration: time.Duration(expHours) * time.Hour,
	}, nil
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	var count int
	err := h.db.Get(&count, "SELECT COUNT(*) FROM users WHERE email = $1", req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "email already exists"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	// Create user
	_, err = h.db.Exec(
		"INSERT INTO users (email, password_hash) VALUES ($1, $2)",
		req.Email, string(hashedPassword),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "user created successfully"})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user from database
	var user User
	err := h.db.Get(&user, "SELECT id, email, password_hash FROM users WHERE email = $1", req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Compare passwords
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Generate token
	tokenString, err := h.GenerateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// Store token in database
	_, err = h.db.Exec(
		"INSERT INTO auth_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)",
		user.ID, tokenString, time.Now().Add(h.tokenDuration),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
		"expires_in": h.tokenDuration.Seconds(),
	})
}

func (h *AuthHandler) GenerateToken(userID string) (string, error) {
	tokenID := make([]byte, 16)
	if _, err := rand.Read(tokenID); err != nil {
		return "", err
	}

	claims := jwt.MapClaims{
		"sub": userID,
		"jti": hex.EncodeToString(tokenID),
		"exp": time.Now().Add(h.tokenDuration).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}

func (h *AuthHandler) VerifyToken(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return h.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return token, nil
}

func (h *AuthHandler) GetJWTSecret() []byte {
	return h.jwtSecret
}

func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization token required"})
			return
		}

		token, err := h.VerifyToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		userID, ok := claims["sub"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
			return
		}

		// Verify token exists in database
		var count int
		err = h.db.Get(&count,
			"SELECT COUNT(*) FROM auth_tokens WHERE token = $1 AND user_id = $2 AND expires_at > NOW()",
			tokenString, userID,
		)
		if err != nil || count == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	// Check Authorization header
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 && strings.EqualFold(authHeader[0:7], "Bearer ") {
		return authHeader[7:]
	}

	// Check URL query parameter
	if token := c.Query("token"); token != "" {
		return token
	}

	// Check form data
	if token := c.PostForm("token"); token != "" {
		return token
	}

	// Check multipart form
	if form, _ := c.MultipartForm(); form != nil {
		if tokens := form.Value["token"]; len(tokens) > 0 {
			return tokens[0]
		}
	}

	return ""
}