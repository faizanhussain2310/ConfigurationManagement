package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden: insufficient role")
)

// Claims is the JWT payload.
type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Config holds auth configuration.
type Config struct {
	JWTSecret     []byte
	TokenDuration time.Duration
}

// NewConfig creates auth config. Generates a random secret if none provided.
func NewConfig(secret string) *Config {
	var secretBytes []byte
	if secret != "" {
		secretBytes = []byte(secret)
	} else {
		secretBytes = make([]byte, 32)
		rand.Read(secretBytes)
	}
	return &Config{
		JWTSecret:     secretBytes,
		TokenDuration: 24 * time.Hour,
	}
}

// HashPassword hashes a plain-text password with bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a plain-text password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateToken creates a signed JWT for a user.
func (c *Config) GenerateToken(username, role string) (string, error) {
	claims := Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(c.TokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(c.JWTSecret)
}

// ValidateToken parses and validates a JWT, returning the claims.
func (c *Config) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrUnauthorized
		}
		return c.JWTSecret, nil
	})
	if err != nil {
		return nil, ErrUnauthorized
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrUnauthorized
	}
	return claims, nil
}

// RoleAtLeast checks if a role has sufficient permissions.
// Hierarchy: admin > editor > viewer
func RoleAtLeast(userRole, requiredRole string) bool {
	levels := map[string]int{
		"viewer": 1,
		"editor": 2,
		"admin":  3,
	}
	return levels[userRole] >= levels[requiredRole]
}

// GenerateWebhookSecret creates a random hex string for HMAC signing.
func GenerateWebhookSecret() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
