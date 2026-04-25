package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/streamspace-dev/streamspace/api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallApplication_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	appDB := NewApplicationDB(db)
	ctx := context.Background()

	req := &models.InstallApplicationRequest{
		CatalogTemplateID: 1,
		DisplayName:       "Firefox",
		Configuration:     map[string]interface{}{"theme": "dark"},
	}
	userID := "user-123"

	// Mock template lookup
	mock.ExpectQuery("SELECT name, display_name, COALESCE").
		WithArgs(req.CatalogTemplateID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "display_name", "description", "category", "icon_url", "manifest"}).
			AddRow("firefox", "Firefox Browser", "Web browser", "browsers", "http://icon.url", "{}"))

	// Mock insert
	mock.ExpectExec("INSERT INTO installed_applications").
		WithArgs(
			sqlmock.AnyArg(), // id
			req.CatalogTemplateID,
			sqlmock.AnyArg(), // name
			req.DisplayName,
			"Web browser", // description
			"browsers",    // category
			"http://icon.url",
			sqlmock.AnyArg(), // icon_data
			sqlmock.AnyArg(), // icon_media_type
			"{}",             // manifest
			sqlmock.AnyArg(), // folder_path
			true,             // enabled
			sqlmock.AnyArg(), // configuration
			userID,
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	app, err := appDB.InstallApplication(ctx, req, userID)

	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.Equal(t, req.DisplayName, app.DisplayName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetApplication_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	appDB := NewApplicationDB(db)
	ctx := context.Background()

	appID := "app-123"
	config := map[string]interface{}{"theme": "dark"}
	configJSON, _ := json.Marshal(config)

	rows := sqlmock.NewRows([]string{
		"id", "catalog_template_id", "name", "display_name", "folder_path",
		"enabled", "configuration", "created_by", "created_at", "updated_at",
		"template_name", "template_display_name", "description", "category",
		"app_type", "icon_url", "manifest", "install_status", "install_message",
	}).AddRow(
		appID, 1, "firefox-guid", "Firefox", "apps/firefox",
		true, string(configJSON), "user-123", time.Now(), time.Now(),
		"firefox", "Firefox Browser", "Desc", "browsers",
		"desktop", "http://icon.url", "{}", "installed", "",
	)

	mock.ExpectQuery("SELECT (.+) FROM installed_applications").
		WithArgs(appID).
		WillReturnRows(rows)

	app, err := appDB.GetApplication(ctx, appID)

	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.Equal(t, appID, app.ID)
	assert.Equal(t, "Firefox", app.DisplayName)
	assert.Equal(t, "dark", app.Configuration["theme"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListApplications_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	appDB := NewApplicationDB(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{
		"id", "catalog_template_id", "name", "display_name", "folder_path",
		"enabled", "configuration", "created_by", "created_at", "updated_at",
		"template_name", "template_display_name", "description", "category",
		"app_type", "icon_url", "install_status", "install_message",
	}).AddRow(
		"app-1", 1, "firefox", "Firefox", "path",
		true, "{}", "user1", time.Now(), time.Now(),
		"firefox", "Firefox", "Desc", "cat",
		"desktop", "url", "installed", "",
	)

	mock.ExpectQuery("SELECT (.+) FROM installed_applications").
		WillReturnRows(rows)

	apps, err := appDB.ListApplications(ctx, false)

	assert.NoError(t, err)
	assert.Len(t, apps, 1)
	assert.Equal(t, "Firefox", apps[0].DisplayName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateApplication_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	appDB := NewApplicationDB(db)
	ctx := context.Background()

	appID := "app-123"
	displayName := "New Name"
	req := &models.UpdateApplicationRequest{
		DisplayName: &displayName,
	}

	mock.ExpectExec("UPDATE installed_applications").
		WithArgs(
			displayName,
			sqlmock.AnyArg(), // updated_at
			appID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = appDB.UpdateApplication(ctx, appID, req)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteApplication_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	appDB := NewApplicationDB(db)
	ctx := context.Background()

	appID := "app-123"

	// Expect delete group access
	mock.ExpectExec("DELETE FROM application_group_access").
		WithArgs(appID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect delete application
	mock.ExpectExec("DELETE FROM installed_applications").
		WithArgs(appID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = appDB.DeleteApplication(ctx, appID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAddGroupAccess_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	appDB := NewApplicationDB(db)
	ctx := context.Background()

	appID := "app-123"
	groupID := "group-456"
	accessLevel := "launch"

	mock.ExpectExec("INSERT INTO application_group_access").
		WithArgs(
			sqlmock.AnyArg(), // id
			appID,
			groupID,
			accessLevel,
			sqlmock.AnyArg(), // created_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = appDB.AddGroupAccess(ctx, appID, groupID, accessLevel)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetApplicationEnabled_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	appDB := NewApplicationDB(db)
	ctx := context.Background()

	appID := "app-123"
	enabled := false

	mock.ExpectExec("UPDATE installed_applications").
		WithArgs(
			enabled,
			sqlmock.AnyArg(), // updated_at
			appID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = appDB.SetApplicationEnabled(ctx, appID, enabled)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
