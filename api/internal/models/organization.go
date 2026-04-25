// Package models defines the core data structures for the StreamSpace API.
// This file implements Organization models for multi-tenancy support.
//
// SECURITY: Organizations provide tenant isolation - all resources MUST be
// scoped to an organization to prevent cross-tenant data access.
package models

import (
	"time"
)

// Organization represents a tenant in StreamSpace.
//
// Organizations enable multi-tenancy by providing:
//   - Isolation: All resources are scoped to an org_id
//   - Namespace mapping: Each org maps to a K8s namespace
//   - RBAC: Org-level roles (OrgAdmin, Maintainer, User, Viewer)
//   - Quotas: Org-wide resource limits
//
// SECURITY: All API handlers MUST filter queries by org_id from the
// authenticated user's JWT claims to prevent cross-tenant access.
//
// Example:
//
//	{
//	  "id": "org-acme",
//	  "name": "acme",
//	  "displayName": "ACME Corporation",
//	  "description": "ACME Corp StreamSpace tenant",
//	  "k8sNamespace": "streamspace-acme",
//	  "status": "active"
//	}
type Organization struct {
	// ID is a unique identifier for this organization (UUID or slug).
	// Format: "org-{name}" or UUID
	ID string `json:"id" db:"id"`

	// Name is a unique machine-readable identifier.
	// Requirements: lowercase, alphanumeric, hyphens only
	// Example: "acme", "engineering-team", "research-lab"
	Name string `json:"name" db:"name"`

	// DisplayName is the human-readable organization name.
	// Example: "ACME Corporation", "Engineering Team"
	DisplayName string `json:"displayName" db:"display_name"`

	// Description explains the purpose of this organization.
	Description string `json:"description" db:"description"`

	// K8sNamespace is the Kubernetes namespace for this org's resources.
	// Sessions and pods are created in this namespace.
	// Default: "streamspace" (single-tenant) or "streamspace-{orgName}" (multi-tenant)
	K8sNamespace string `json:"k8sNamespace" db:"k8s_namespace"`

	// Status indicates the organization's state.
	//
	// Valid statuses:
	//   - "active": Normal operation
	//   - "suspended": Temporarily disabled (billing, policy)
	//   - "deleted": Soft-deleted, pending cleanup
	//
	// Default: "active"
	Status string `json:"status" db:"status"`

	// CreatedAt is when this organization was created.
	CreatedAt time.Time `json:"createdAt" db:"created_at"`

	// UpdatedAt is when this organization was last modified.
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}

// CreateOrganizationRequest represents a request to create a new organization.
//
// Validation rules:
//   - Name: required, lowercase, alphanumeric + hyphens
//   - DisplayName: required
//
// Example:
//
//	{
//	  "name": "acme",
//	  "displayName": "ACME Corporation",
//	  "description": "ACME Corp StreamSpace tenant"
//	}
type CreateOrganizationRequest struct {
	Name         string `json:"name" binding:"required" validate:"required,min=3,max=50,lowercase,alphanum|contains=-"`
	DisplayName  string `json:"displayName" binding:"required" validate:"required,min=3,max=100"`
	Description  string `json:"description" validate:"omitempty,max=500"`
	K8sNamespace string `json:"k8sNamespace" validate:"omitempty,min=3,max=63,lowercase"`
}

// UpdateOrganizationRequest represents a request to update an organization.
//
// All fields are optional (pointer types) - only provided fields are updated.
type UpdateOrganizationRequest struct {
	DisplayName  *string `json:"displayName,omitempty" validate:"omitempty,min=3,max=100"`
	Description  *string `json:"description,omitempty" validate:"omitempty,max=500"`
	K8sNamespace *string `json:"k8sNamespace,omitempty" validate:"omitempty,min=3,max=63,lowercase"`
	Status       *string `json:"status,omitempty" validate:"omitempty,oneof=active suspended deleted"`
}

// OrgRole defines the user's role within an organization.
// This is separate from the system-wide role (admin/operator/user).
type OrgRole string

const (
	// OrgRoleAdmin can manage users/roles, templates, org settings, webhooks.
	// Full access within the organization.
	OrgRoleAdmin OrgRole = "org_admin"

	// OrgRoleMaintainer can manage templates, start/stop/hibernate sessions.
	// No user/role administration.
	OrgRoleMaintainer OrgRole = "maintainer"

	// OrgRoleUser can manage own sessions and list org templates.
	// Standard user access.
	OrgRoleUser OrgRole = "user"

	// OrgRoleViewer has read-only access to lists/metrics.
	// No session lifecycle permissions.
	OrgRoleViewer OrgRole = "viewer"
)

// ValidOrgRoles returns all valid organization roles.
func ValidOrgRoles() []OrgRole {
	return []OrgRole{OrgRoleAdmin, OrgRoleMaintainer, OrgRoleUser, OrgRoleViewer}
}

// IsValidOrgRole checks if the given role is valid.
func IsValidOrgRole(role string) bool {
	for _, r := range ValidOrgRoles() {
		if string(r) == role {
			return true
		}
	}
	return false
}
