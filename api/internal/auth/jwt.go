package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig holds JWT configuration
type JWTConfig struct {
	SecretKey     string
	Issuer        string
	TokenDuration time.Duration
}

// Claims represents custom JWT claims
type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Role     string   `json:"role"`
	Groups   []string `json:"groups,omitempty"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT token operations
type JWTManager struct {
	config *JWTConfig
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(config *JWTConfig) *JWTManager {
	if config.TokenDuration == 0 {
		config.TokenDuration = 24 * time.Hour // Default 24 hours
	}
	if config.Issuer == "" {
		config.Issuer = "streamspace-api"
	}
	return &JWTManager{
		config: config,
	}
}

// GenerateToken generates a new JWT token for a user
func (m *JWTManager) GenerateToken(userID, username, email, role string, groups []string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
		Groups:   groups,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.config.Issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.config.TokenDuration)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(m.config.SecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.config.SecretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// RefreshToken generates a new token with extended expiration
func (m *JWTManager) RefreshToken(tokenString string) (string, error) {
	claims, err := m.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	// Check if token is not too old (allow refresh within 7 days of expiration)
	if time.Until(claims.ExpiresAt.Time) > 7*24*time.Hour {
		return "", errors.New("token not eligible for refresh yet")
	}

	// Generate new token with same claims but updated timestamps
	return m.GenerateToken(claims.UserID, claims.Username, claims.Email, claims.Role, claims.Groups)
}

// ExtractUserID extracts the user ID from a token without full validation
func (m *JWTManager) ExtractUserID(tokenString string) (string, error) {
	claims, err := m.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}
