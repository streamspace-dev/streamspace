package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/streamspace/streamspace/api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateGroup_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	groupDB := NewGroupDB(db)
	ctx := context.Background()

	req := &models.CreateGroupRequest{
		Name:        "engineering",
		DisplayName: "Engineering",
		Description: "Engineering Department",
		Type:        "department",
	}

	mock.ExpectExec("INSERT INTO groups").
		WithArgs(
			sqlmock.AnyArg(), // id
			req.Name,
			req.DisplayName,
			req.Description,
			req.Type,
			nil,              // parent_id
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	group, err := groupDB.CreateGroup(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, group)
	assert.Equal(t, req.Name, group.Name)
	assert.NotEmpty(t, group.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetGroup_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	groupDB := NewGroupDB(db)
	ctx := context.Background()

	groupID := "group-123"
	expectedGroup := &models.Group{
		ID:          groupID,
		Name:        "engineering",
		DisplayName: "Engineering",
		Description: "Engineering Dept",
		Type:        "department",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		MemberCount: 5,
	}

	rows := sqlmock.NewRows([]string{
		"id", "name", "display_name", "description", "type", "parent_id",
		"created_at", "updated_at", "member_count",
	}).AddRow(
		expectedGroup.ID, expectedGroup.Name, expectedGroup.DisplayName,
		expectedGroup.Description, expectedGroup.Type, nil,
		expectedGroup.CreatedAt, expectedGroup.UpdatedAt, expectedGroup.MemberCount,
	)

	mock.ExpectQuery("SELECT (.+) FROM groups").
		WithArgs(groupID).
		WillReturnRows(rows)

	group, err := groupDB.GetGroup(ctx, groupID)

	assert.NoError(t, err)
	assert.NotNil(t, group)
	assert.Equal(t, expectedGroup.ID, group.ID)
	assert.Equal(t, expectedGroup.Name, group.Name)
	assert.Equal(t, expectedGroup.MemberCount, group.MemberCount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetGroup_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	groupDB := NewGroupDB(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT (.+) FROM groups").
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	group, err := groupDB.GetGroup(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListGroups_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	groupDB := NewGroupDB(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{
		"id", "name", "display_name", "description", "type", "parent_id",
		"created_at", "updated_at", "member_count",
	}).
		AddRow("g1", "eng", "Engineering", "Desc", "dept", nil, time.Now(), time.Now(), 10).
		AddRow("g2", "sales", "Sales", "Desc", "dept", nil, time.Now(), time.Now(), 5)

	// Expect query without filters
	mock.ExpectQuery("SELECT (.+) FROM groups").
		WillReturnRows(rows)

	groups, err := groupDB.ListGroups(ctx, "", nil)

	assert.NoError(t, err)
	assert.Len(t, groups, 2)
	assert.Equal(t, "eng", groups[0].Name)
	assert.Equal(t, "sales", groups[1].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListGroups_WithFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	groupDB := NewGroupDB(db)
	ctx := context.Background()

	groupType := "team"
	parentID := "parent-123"

	rows := sqlmock.NewRows([]string{
		"id", "name", "display_name", "description", "type", "parent_id",
		"created_at", "updated_at", "member_count",
	}).AddRow("g3", "backend", "Backend", "Desc", "team", parentID, time.Now(), time.Now(), 3)

	// Expect query with type and parent_id filters
	mock.ExpectQuery("SELECT (.+) FROM groups").
		WithArgs(groupType, parentID).
		WillReturnRows(rows)

	groups, err := groupDB.ListGroups(ctx, groupType, &parentID)

	assert.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "backend", groups[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAddGroupMember_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	groupDB := NewGroupDB(db)
	ctx := context.Background()

	groupID := "group-123"
	req := &models.AddGroupMemberRequest{
		UserID: "user-456",
		Role:   "admin",
	}

	mock.ExpectExec("INSERT INTO group_memberships").
		WithArgs(
			sqlmock.AnyArg(), // id
			req.UserID,
			groupID,
			req.Role,
			sqlmock.AnyArg(), // created_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = groupDB.AddGroupMember(ctx, groupID, req)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetGroupMembers_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	groupDB := NewGroupDB(db)
	ctx := context.Background()

	groupID := "group-123"
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "group_id", "role", "created_at",
	}).
		AddRow("m1", "user1", groupID, "owner", time.Now()).
		AddRow("m2", "user2", groupID, "member", time.Now())

	mock.ExpectQuery("SELECT (.+) FROM group_memberships").
		WithArgs(groupID).
		WillReturnRows(rows)

	members, err := groupDB.GetGroupMembers(ctx, groupID)

	assert.NoError(t, err)
	assert.Len(t, members, 2)
	assert.Equal(t, "user1", members[0].UserID)
	assert.Equal(t, "owner", members[0].Role)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteGroup_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	groupDB := NewGroupDB(db)
	ctx := context.Background()

	groupID := "group-123"

	// Expect deletion of memberships first
	mock.ExpectExec("DELETE FROM group_memberships").
		WithArgs(groupID).
		WillReturnResult(sqlmock.NewResult(0, 5))

	// Expect deletion of quotas
	mock.ExpectExec("DELETE FROM group_quotas").
		WithArgs(groupID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect deletion of group
	mock.ExpectExec("DELETE FROM groups").
		WithArgs(groupID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = groupDB.DeleteGroup(ctx, groupID)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetGroupQuota_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	groupDB := NewGroupDB(db)
	ctx := context.Background()

	groupID := "group-123"
	maxSessions := 20
	req := &models.SetQuotaRequest{
		MaxSessions: &maxSessions,
	}

	// Expect check for existing quota (returns error/no rows)
	mock.ExpectQuery("SELECT (.+) FROM group_quotas").
		WithArgs(groupID).
		WillReturnError(sql.ErrNoRows)

	// Expect insert
	mock.ExpectExec("INSERT INTO group_quotas").
		WithArgs(
			groupID,
			maxSessions,
			sqlmock.AnyArg(), // default cpu
			sqlmock.AnyArg(), // default memory
			sqlmock.AnyArg(), // default storage
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = groupDB.SetGroupQuota(ctx, groupID, req)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetGroupQuota_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	groupDB := NewGroupDB(db)
	ctx := context.Background()

	groupID := "group-123"
	rows := sqlmock.NewRows([]string{
		"group_id", "max_sessions", "max_cpu", "max_memory", "max_storage",
		"used_sessions", "used_cpu", "used_memory", "used_storage",
		"created_at", "updated_at",
	}).AddRow(
		groupID, 10, "8000m", "32Gi", "500Gi",
		2, "2000m", "8Gi", "100Gi",
		time.Now(), time.Now(),
	)

	mock.ExpectQuery("SELECT (.+) FROM group_quotas").
		WithArgs(groupID).
		WillReturnRows(rows)

	quota, err := groupDB.GetGroupQuota(ctx, groupID)

	assert.NoError(t, err)
	assert.NotNil(t, quota)
	assert.Equal(t, 10, quota.MaxSessions)
	assert.Equal(t, 2, quota.UsedSessions)
	assert.NoError(t, mock.ExpectationsWereMet())
}
