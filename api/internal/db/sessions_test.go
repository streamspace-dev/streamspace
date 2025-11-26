package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSession_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sessionDB := NewSessionDB(db)
	ctx := context.Background()

	session := &Session{
		ID:           "session123",
		UserID:       "user123",
		OrgID:        "org123",
		TemplateName: "ubuntu-22.04",
		State:        "pending",
		AppType:      "desktop",
		CPU:          "1000m",
		Memory:       "2Gi",
		Namespace:    "streamspace",
		Platform:     "kubernetes",
	}

	// Expect INSERT with all session fields (25 parameters including org_id and timestamps)
	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(sqlmock.AnyArg(), session.UserID, session.OrgID, sqlmock.AnyArg(), session.TemplateName, session.State, session.AppType,
			sqlmock.AnyArg(), sqlmock.AnyArg(), session.Namespace, session.Platform, sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = sessionDB.CreateSession(ctx, session)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSession_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sessionDB := NewSessionDB(db)
	ctx := context.Background()

	sessionID := "session123"

	// Match the 25 columns from the actual GetSession query (including org_id, agent_id, cluster_id)
	rows := sqlmock.NewRows([]string{"id", "user_id", "org_id", "team_id", "template_name", "state", "app_type",
		"active_connections", "url", "namespace", "platform", "agent_id", "cluster_id", "pod_name",
		"memory", "cpu", "persistent_home", "idle_timeout", "max_session_duration",
		"tags", "created_at", "updated_at", "last_connection", "last_disconnect", "last_activity"}).
		AddRow("session123", "user123", "org123", "", "ubuntu-22.04", "running", "desktop",
			0, "https://session123.example.com", "streamspace", "kubernetes", "", "", "pod-123",
			"2Gi", "1000m", false, "3600", "28800",
			nil, time.Now(), time.Now(), nil, nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM sessions WHERE id").
		WithArgs(sessionID).
		WillReturnRows(rows)

	session, err := sessionDB.GetSession(ctx, sessionID)

	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, "session123", session.ID)
	assert.Equal(t, "user123", session.UserID)
	assert.Equal(t, "org123", session.OrgID)
	assert.Equal(t, "running", session.State)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSession_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sessionDB := NewSessionDB(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT (.+) FROM sessions WHERE id").
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	session, err := sessionDB.GetSession(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListSessions_ByUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sessionDB := NewSessionDB(db)
	ctx := context.Background()

	userID := "user123"

	// Match the 25 columns from the actual query (including org_id, agent_id, cluster_id, tags)
	rows := sqlmock.NewRows([]string{"id", "user_id", "org_id", "team_id", "template_name", "state", "app_type",
		"active_connections", "url", "namespace", "platform", "agent_id", "cluster_id", "pod_name",
		"memory", "cpu", "persistent_home", "idle_timeout", "max_session_duration",
		"tags", "created_at", "updated_at", "last_connection", "last_disconnect", "last_activity"}).
		AddRow("session1", userID, "org123", "", "ubuntu", "running", "desktop", 0, "", "streamspace", "kubernetes", "", "", "", "2Gi", "1000m", false, "", "", nil, time.Now(), time.Now(), nil, nil, nil).
		AddRow("session2", userID, "org123", "", "debian", "stopped", "desktop", 0, "", "streamspace", "kubernetes", "", "", "", "1Gi", "500m", false, "", "", nil, time.Now(), time.Now(), nil, nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM sessions WHERE user_id").
		WithArgs(userID).
		WillReturnRows(rows)

	sessions, err := sessionDB.ListSessionsByUser(ctx, userID)

	assert.NoError(t, err)
	assert.Len(t, sessions, 2)
	assert.Equal(t, "session1", sessions[0].ID)
	assert.Equal(t, "session2", sessions[1].ID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateSessionStatus_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sessionDB := NewSessionDB(db)
	ctx := context.Background()

	sessionID := "session123"
	newStatus := "stopped"

	mock.ExpectExec("UPDATE sessions SET state").
		WithArgs(newStatus, sqlmock.AnyArg(), sessionID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = sessionDB.UpdateSessionState(ctx, sessionID, newStatus)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSession_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sessionDB := NewSessionDB(db)
	ctx := context.Background()

	sessionID := "session123"

	// DeleteSession uses UPDATE to set state='deleted', not DELETE
	mock.ExpectExec("UPDATE sessions").
		WithArgs(sqlmock.AnyArg(), sessionID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = sessionDB.DeleteSession(ctx, sessionID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSession_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sessionDB := NewSessionDB(db)
	ctx := context.Background()

	// DeleteSession doesn't check rows affected, it just executes
	mock.ExpectExec("UPDATE sessions").
		WithArgs(sqlmock.AnyArg(), "nonexistent").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = sessionDB.DeleteSession(ctx, "nonexistent")

	// DeleteSession doesn't return error for 0 rows
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCountUserSessions_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sessionDB := NewSessionDB(db)
	ctx := context.Background()

	userID := "user123"

	rows := sqlmock.NewRows([]string{"count"}).AddRow(5)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM sessions WHERE user_id").
		WithArgs(userID).
		WillReturnRows(rows)

	count, err := sessionDB.CountSessionsByUser(ctx, userID)

	assert.NoError(t, err)
	assert.Equal(t, 5, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}
