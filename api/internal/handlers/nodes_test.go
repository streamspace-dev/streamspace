package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupNodesTest creates a test handler with mocked database
func setupNodesTest(t *testing.T) (*NodeHandler, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewNodeHandler(database, nil, nil, "kubernetes")

	cleanup := func() {
		mockDB.Close()
	}

	return handler, mock, cleanup
}

// TestNewNodeHandler tests handler initialization
func TestNewNodeHandler(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	database := db.NewDatabaseForTesting(mockDB)
	handler := NewNodeHandler(database, nil, nil, "kubernetes")

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
	assert.Equal(t, "kubernetes", handler.platform)
}

// verifyDeprecationResponse checks if response is a proper deprecation message
func verifyDeprecationResponse(t *testing.T, w *httptest.ResponseRecorder) {
	assert.Equal(t, http.StatusGone, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Node management has been moved to a plugin", response["error"])
	assert.Contains(t, response["message"], "streamspace-node-manager")
	assert.Equal(t, "deprecated", response["status"])
	assert.Equal(t, "v2.0.0", response["removed_in"])

	// Verify migration info
	migration := response["migration"].(map[string]interface{})
	assert.Contains(t, migration["install"], "streamspace-node-manager")
	assert.Equal(t, "/api/plugins/streamspace-node-manager", migration["api_base"])
	assert.Contains(t, migration["documentation"], "docs.streamspace.io")

	// Verify benefits listed
	benefits := response["benefits"].([]interface{})
	assert.NotEmpty(t, benefits)
	assert.Contains(t, benefits, "Enhanced auto-scaling capabilities")
}

// TestListNodes_Deprecated tests listing nodes returns deprecation
func TestListNodes_Deprecated(t *testing.T) {
	handler, _, cleanup := setupNodesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/nodes", nil)

	handler.ListNodes(c)

	verifyDeprecationResponse(t, w)
}

// TestGetClusterStats_Deprecated tests cluster stats returns deprecation
func TestGetClusterStats_Deprecated(t *testing.T) {
	handler, _, cleanup := setupNodesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/nodes/stats", nil)

	handler.GetClusterStats(c)

	verifyDeprecationResponse(t, w)
}

// TestGetNode_Deprecated tests getting node returns deprecation
func TestGetNode_Deprecated(t *testing.T) {
	handler, _, cleanup := setupNodesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/nodes/node-1", nil)
	c.Params = []gin.Param{{Key: "name", Value: "node-1"}}

	handler.GetNode(c)

	verifyDeprecationResponse(t, w)
}

// TestAddNodeLabel_Deprecated tests adding node label returns deprecation
func TestAddNodeLabel_Deprecated(t *testing.T) {
	handler, _, cleanup := setupNodesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/nodes/node-1/labels", nil)
	c.Params = []gin.Param{{Key: "name", Value: "node-1"}}

	handler.AddNodeLabel(c)

	verifyDeprecationResponse(t, w)
}

// TestRemoveNodeLabel_Deprecated tests removing node label returns deprecation
func TestRemoveNodeLabel_Deprecated(t *testing.T) {
	handler, _, cleanup := setupNodesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/nodes/node-1/labels/key", nil)
	c.Params = []gin.Param{
		{Key: "name", Value: "node-1"},
		{Key: "key", Value: "key"},
	}

	handler.RemoveNodeLabel(c)

	verifyDeprecationResponse(t, w)
}

// TestAddNodeTaint_Deprecated tests adding node taint returns deprecation
func TestAddNodeTaint_Deprecated(t *testing.T) {
	handler, _, cleanup := setupNodesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/nodes/node-1/taints", nil)
	c.Params = []gin.Param{{Key: "name", Value: "node-1"}}

	handler.AddNodeTaint(c)

	verifyDeprecationResponse(t, w)
}

// TestRemoveNodeTaint_Deprecated tests removing node taint returns deprecation
func TestRemoveNodeTaint_Deprecated(t *testing.T) {
	handler, _, cleanup := setupNodesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/nodes/node-1/taints/key", nil)
	c.Params = []gin.Param{
		{Key: "name", Value: "node-1"},
		{Key: "key", Value: "key"},
	}

	handler.RemoveNodeTaint(c)

	verifyDeprecationResponse(t, w)
}

// TestCordonNode_Deprecated tests cordoning node returns deprecation
func TestCordonNode_Deprecated(t *testing.T) {
	handler, _, cleanup := setupNodesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/nodes/node-1/cordon", nil)
	c.Params = []gin.Param{{Key: "name", Value: "node-1"}}

	handler.CordonNode(c)

	verifyDeprecationResponse(t, w)
}

// TestUncordonNode_Deprecated tests uncordoning node returns deprecation
func TestUncordonNode_Deprecated(t *testing.T) {
	handler, _, cleanup := setupNodesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/nodes/node-1/uncordon", nil)
	c.Params = []gin.Param{{Key: "name", Value: "node-1"}}

	handler.UncordonNode(c)

	verifyDeprecationResponse(t, w)
}

// TestDrainNode_Deprecated tests draining node returns deprecation
func TestDrainNode_Deprecated(t *testing.T) {
	handler, _, cleanup := setupNodesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/nodes/node-1/drain", nil)
	c.Params = []gin.Param{{Key: "name", Value: "node-1"}}

	handler.DrainNode(c)

	verifyDeprecationResponse(t, w)
}

// TestDeprecationMessage_Format tests the deprecation message format
func TestDeprecationMessage_Format(t *testing.T) {
	handler, _, cleanup := setupNodesTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/nodes", nil)

	handler.ListNodes(c)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify all expected fields exist
	assert.Contains(t, response, "error")
	assert.Contains(t, response, "message")
	assert.Contains(t, response, "migration")
	assert.Contains(t, response, "benefits")
	assert.Contains(t, response, "status")
	assert.Contains(t, response, "removed_in")

	// Verify migration object structure
	migration := response["migration"].(map[string]interface{})
	assert.Contains(t, migration, "install")
	assert.Contains(t, migration, "api_base")
	assert.Contains(t, migration, "documentation")

	// Verify benefits is an array
	benefits, ok := response["benefits"].([]interface{})
	assert.True(t, ok)
	assert.GreaterOrEqual(t, len(benefits), 3)
}
