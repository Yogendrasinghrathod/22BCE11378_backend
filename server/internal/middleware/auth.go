package middleware

import (
	"net/http"
	"strings"
	"time"
	"fmt"
	

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
)

type AuthMiddleware struct {
	db        *sqlx.DB
	jwtSecret []byte
}

func NewAuthMiddleware(db *sqlx.DB, jwtSecret []byte) *AuthMiddleware {
	return &AuthMiddleware{
		db:        db,
		jwtSecret: jwtSecret,
	}
}

func (m *AuthMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c) 
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization token required",
			})
			return
		}

		// Parse token
		
// Copy
token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
    // 1. Verify the signing method
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
    }
    
    // 2. Return the secret key
    return m.jwtSecret, nil
})

		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			return
		}

		// Verify claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token claims",
			})
			return
		}

		// Verify token in database
		userID, ok := claims["sub"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid user ID in token",
			})
			return
		}

		var expiresAt time.Time
		err = m.db.QueryRow(
			"SELECT expires_at FROM auth_tokens WHERE token = $1 AND user_id = $2",
			tokenString, userID,
		).Scan(&expiresAt)

		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			return
		}

		if time.Now().After(expiresAt) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Token expired",
			})
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	// Check Authorization header
	bearerToken := c.GetHeader("Authorization")
	if len(bearerToken) > 7 && strings.EqualFold(bearerToken[0:7], "Bearer ") {
		return bearerToken[7:]
	}

	// Check URL query
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