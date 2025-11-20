// Package models defines the core data structures for the StreamSpace API.
//
// This file contains models for installed applications and access control.
//
// Features:
//   - Installed application representation
//   - Group access control models
//   - Request/response types for application API
package models

import (
	"time"
)

// InstalledApplication represents an installed application instance.
//
// Applications are installed from catalog templates and can be:
//   - Customized with display names
//   - Configured with application-specific settings
//   - Enabled/disabled
//   - Granted to specific groups
//
// Example:
//
//	{
//	  "id": "550e8400-e29b-41d4-a716-446655440000",
//	  "catalogTemplateId": 1,
//	  "name": "firefox-abc12345",
//	  "displayName": "Firefox Browser",
//	  "folderPath": "apps/firefox-abc12345",
//	  "enabled": true,
//	  "configuration": {"homepage": "https://example.com"}
//	}
type InstalledApplication struct {
	// ID is a unique identifier for this installed application (UUID v4).
	ID string `json:"id" db:"id"`

	// CatalogTemplateID is the ID of the source catalog template.
	CatalogTemplateID int `json:"catalogTemplateId" db:"catalog_template_id"`

	// Name is the internal name with GUID suffix for uniqueness.
	// Example: "firefox-abc12345"
	Name string `json:"name" db:"name"`

	// DisplayName is the custom name shown on user dashboards.
	// Can be changed by admins to customize the user experience.
	// Example: "Firefox Browser", "Development Firefox"
	DisplayName string `json:"displayName" db:"display_name"`

	// FolderPath is the path to the application configuration folder.
	// Example: "apps/firefox-abc12345"
	FolderPath string `json:"folderPath" db:"folder_path"`

	// Enabled indicates whether the application is active.
	// Disabled applications are not shown to users.
	Enabled bool `json:"enabled" db:"enabled"`

	// Configuration contains application-specific settings as JSONB.
	// Schema depends on the template's configurable options.
	Configuration map[string]interface{} `json:"configuration,omitempty"`

	// CreatedBy is the user ID who installed the application.
	CreatedBy string `json:"createdBy" db:"created_by"`

	// CreatedAt is when the application was installed.
	CreatedAt time.Time `json:"createdAt" db:"created_at"`

	// UpdatedAt is when the application was last modified.
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`

	// Template information (stored in installed_applications for persistence)
	TemplateName        string `json:"templateName,omitempty"`
	TemplateDisplayName string `json:"templateDisplayName,omitempty"`
	Description         string `json:"description,omitempty" db:"description"`
	Category            string `json:"category,omitempty" db:"category"`
	AppType             string `json:"appType,omitempty"`
	IconURL             string `json:"icon,omitempty" db:"icon_url"`
	IconData            []byte `json:"-" db:"icon_data"`           // Binary icon data (not sent in JSON)
	IconMediaType       string `json:"-" db:"icon_media_type"`     // MIME type of icon
	Manifest            string `json:"manifest,omitempty" db:"manifest"`

	// InstallStatus tracks the installation state (pending, creating, installed, failed)
	InstallStatus string `json:"installStatus,omitempty" db:"install_status"`

	// InstallMessage provides additional context about the installation status
	InstallMessage string `json:"installMessage,omitempty" db:"install_message"`

	// Groups with access to this application (populated separately)
	Groups []*ApplicationGroupAccess `json:"groups,omitempty"`
}

// ApplicationGroupAccess represents a group's access to an application.
//
// Access levels:
//   - "view": Can see the application in the catalog
//   - "launch": Can launch sessions with this application
//   - "admin": Can modify application settings
type ApplicationGroupAccess struct {
	// ID is a unique identifier for this access record.
	ID string `json:"id" db:"id"`

	// ApplicationID is the installed application.
	ApplicationID string `json:"applicationId" db:"application_id"`

	// GroupID is the group with access.
	GroupID string `json:"groupId" db:"group_id"`

	// AccessLevel is the permission level.
	// Valid values: "view", "launch", "admin"
	AccessLevel string `json:"accessLevel" db:"access_level"`

	// CreatedAt is when access was granted.
	CreatedAt time.Time `json:"createdAt" db:"created_at"`

	// Group information (populated from JOIN)
	GroupName        string `json:"groupName,omitempty"`
	GroupDisplayName string `json:"groupDisplayName,omitempty"`
}

// InstallApplicationRequest is the request to install a new application.
type InstallApplicationRequest struct {
	// CatalogTemplateID is the source template to install from.
	CatalogTemplateID int `json:"catalogTemplateId" binding:"required"`

	// DisplayName is the custom name for this installation (optional).
	// If not provided, uses the template's default display name.
	DisplayName string `json:"displayName"`

	// Platform specifies which platform to install on (optional).
	// Valid values: kubernetes, docker, hyperv, vcenter
	// If not provided, defaults to the template's platform or 'kubernetes'.
	Platform string `json:"platform"`

	// Configuration is the initial application settings (optional).
	Configuration map[string]interface{} `json:"configuration"`

	// GroupIDs is the list of groups to grant access (optional).
	// If not provided, no groups will have access initially.
	GroupIDs []string `json:"groupIds"`
}

// UpdateApplicationRequest is the request to update an installed application.
type UpdateApplicationRequest struct {
	// DisplayName updates the custom display name.
	DisplayName *string `json:"displayName,omitempty"`

	// Enabled updates the active status.
	Enabled *bool `json:"enabled,omitempty"`

	// Configuration updates the application settings.
	Configuration map[string]interface{} `json:"configuration,omitempty"`
}

// AddGroupAccessRequest is the request to grant group access to an application.
type AddGroupAccessRequest struct {
	// GroupID is the group to grant access.
	GroupID string `json:"groupId" binding:"required"`

	// AccessLevel is the permission level.
	// Valid values: "view", "launch", "admin"
	// Default: "launch"
	AccessLevel string `json:"accessLevel"`
}

// UpdateGroupAccessRequest is the request to update a group's access level.
type UpdateGroupAccessRequest struct {
	// AccessLevel is the new permission level.
	// Valid values: "view", "launch", "admin"
	AccessLevel string `json:"accessLevel" binding:"required"`
}

// ApplicationListResponse is the response for listing applications.
type ApplicationListResponse struct {
	Applications []*InstalledApplication `json:"applications"`
	Total        int                     `json:"total"`
}

// ApplicationWithGroups is an application with its group access list.
type ApplicationWithGroups struct {
	*InstalledApplication
	Groups []*ApplicationGroupAccess `json:"groups"`
}
