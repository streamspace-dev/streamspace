package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/streamspace-dev/streamspace/api/internal/auth"
)

// createTestToken creates a JWT token for testing
func createTestToken(t *testing.T, jwtManager *auth.JWTManager, claims *auth.Claims) string {
	// Create token using the manager's config
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte("test-secret-key-at-least-32-bytes"))
	require.NoError(t, err)
	return tokenString
}

func TestOrgContextMiddleware_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:     "test-secret-key-at-least-32-bytes",
		Issuer:        "streamspace-test",
		TokenDuration: 24 * time.Hour,
	})

	// Create a token with org context
	token, err := jwtManager.GenerateTokenWithOrg(
		nil,
		"user123",
		"testuser",
		"test@example.com",
		"user",
		[]string{"team1"},
		&auth.OrgInfo{
			OrgID:        "org123",
			OrgName:      "Test Org",
			K8sNamespace: "streamspace-test",
			OrgRole:      "user",
		},
		"127.0.0.1",
		"TestAgent",
	)
	require.NoError(t, err)

	router := gin.New()
	router.Use(OrgContextMiddleware(jwtManager))
	router.GET("/test", func(c *gin.Context) {
		orgID, err := GetOrgID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"org_id": orgID})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "org123")
}

func TestOrgContextMiddleware_MissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:     "test-secret-key-at-least-32-bytes",
		Issuer:        "streamspace-test",
		TokenDuration: 24 * time.Hour,
	})

	router := gin.New()
	router.Use(OrgContextMiddleware(jwtManager))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization header required")
}

func TestOrgContextMiddleware_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:     "test-secret-key-at-least-32-bytes",
		Issuer:        "streamspace-test",
		TokenDuration: 24 * time.Hour,
	})

	router := gin.New()
	router.Use(OrgContextMiddleware(jwtManager))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid or expired token")
}

func TestOrgContextMiddleware_TokenMissingOrgID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		SecretKey:     "test-secret-key-at-least-32-bytes",
		Issuer:        "streamspace-test",
		TokenDuration: 24 * time.Hour,
	})

	// Create token WITHOUT org_id using the deprecated method
	// This simulates old tokens that don't have org context
	claims := &auth.Claims{
		UserID:   "user123",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
		OrgID:    "", // Empty org_id
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "streamspace-test",
			Subject:   "user123",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret-key-at-least-32-bytes"))
	require.NoError(t, err)

	router := gin.New()
	router.Use(OrgContextMiddleware(jwtManager))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Token missing organization context")
}

func TestGetOrgID_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(ContextKeyOrgID, "org123")

	orgID, err := GetOrgID(c)

	assert.NoError(t, err)
	assert.Equal(t, "org123", orgID)
}

func TestGetOrgID_Missing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// No org_id set

	orgID, err := GetOrgID(c)

	assert.Error(t, err)
	assert.Empty(t, orgID)
	assert.Equal(t, ErrMissingOrgContext, err)
}

func TestGetK8sNamespace_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(ContextKeyK8sNamespace, "streamspace-acme")

	ns, err := GetK8sNamespace(c)

	assert.NoError(t, err)
	assert.Equal(t, "streamspace-acme", ns)
}

func TestGetK8sNamespace_Default(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(ContextKeyK8sNamespace, "")

	ns, err := GetK8sNamespace(c)

	assert.NoError(t, err)
	assert.Equal(t, "streamspace", ns) // Default namespace
}

func TestRequireOrgRole_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ContextKeyOrgRole, "org_admin")
		c.Next()
	})
	router.Use(RequireOrgRole("org_admin", "maintainer"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireOrgRole_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ContextKeyOrgRole, "viewer")
		c.Next()
	})
	router.Use(RequireOrgRole("org_admin", "maintainer"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Insufficient permissions")
}
