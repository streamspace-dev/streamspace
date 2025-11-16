// Package cache provides Redis-based caching for StreamSpace API.
//
// This file defines standardized cache key naming conventions and patterns.
//
// Purpose:
// - Provide consistent cache key naming across all cache operations
// - Enable efficient cache invalidation via pattern matching
// - Organize cache keys by resource type
// - Support multi-level cache hierarchies
//
// Features:
// - Resource-specific key prefixes (session, user, template, quota)
// - Hierarchical key structure (prefix:resource:identifier)
// - Pattern-based invalidation (session:*, user:123:*)
// - User-scoped keys for isolation
// - List and collection keys
//
// Key Naming Convention:
//   - Format: {prefix}:{resource}:{identifier}
//   - Example: session:user1-firefox
//   - Example: user:username:alice
//   - Example: quota:user:user123
//
// Key Patterns for Invalidation:
//   - session:* - All sessions
//   - *:user:123* - All user-specific caches
//   - template:category:browsers - Templates in category
//
// Implementation Details:
// - Keys use colon (:) as separator for Redis best practices
// - Prefixes prevent key collisions across resource types
// - Patterns use wildcards (*) for bulk invalidation
//
// Example Usage:
//
//	// Generate cache key for session
//	key := cache.SessionKey("user1-firefox")
//	// Result: "session:user1-firefox"
//
//	// Generate key for user by username
//	key := cache.UserByUsernameKey("alice")
//	// Result: "user:username:alice"
//
//	// Invalidate all user-related caches
//	pattern := cache.UserPattern("user123")
//	err := cache.DeletePattern(ctx, pattern)
//	// Deletes: user:user123, session:user:user123:list, quota:user:user123, etc.
package cache

import "fmt"

// Key prefixes for different resource types
const (
	PrefixSession    = "session"
	PrefixUser       = "user"
	PrefixTemplate   = "template"
	PrefixQuota      = "quota"
	PrefixConfig     = "config"
	PrefixRepository = "repository"
	PrefixShare      = "share"
	PrefixStats      = "stats"
)

// Session cache keys
func SessionKey(sessionID string) string {
	return fmt.Sprintf("%s:%s", PrefixSession, sessionID)
}

func UserSessionsKey(userID string) string {
	return fmt.Sprintf("%s:user:%s:list", PrefixSession, userID)
}

func AllSessionsKey() string {
	return fmt.Sprintf("%s:all", PrefixSession)
}

func SessionStatsKey(sessionID string) string {
	return fmt.Sprintf("%s:%s:stats", PrefixSession, sessionID)
}

// User cache keys
func UserKey(userID string) string {
	return fmt.Sprintf("%s:%s", PrefixUser, userID)
}

func UserByUsernameKey(username string) string {
	return fmt.Sprintf("%s:username:%s", PrefixUser, username)
}

func UserByEmailKey(email string) string {
	return fmt.Sprintf("%s:email:%s", PrefixUser, email)
}

func AllUsersKey() string {
	return fmt.Sprintf("%s:all", PrefixUser)
}

// Template cache keys
func TemplateKey(templateName string) string {
	return fmt.Sprintf("%s:%s", PrefixTemplate, templateName)
}

func TemplateByCategoryKey(category string) string {
	return fmt.Sprintf("%s:category:%s", PrefixTemplate, category)
}

func AllTemplatesKey() string {
	return fmt.Sprintf("%s:all", PrefixTemplate)
}

func FeaturedTemplatesKey() string {
	return fmt.Sprintf("%s:featured", PrefixTemplate)
}

// Quota cache keys
func UserQuotaKey(userID string) string {
	return fmt.Sprintf("%s:user:%s", PrefixQuota, userID)
}

func AllQuotasKey() string {
	return fmt.Sprintf("%s:all", PrefixQuota)
}

// Configuration cache keys
func ConfigKey(key string) string {
	return fmt.Sprintf("%s:%s", PrefixConfig, key)
}

func AllConfigKey() string {
	return fmt.Sprintf("%s:all", PrefixConfig)
}

// Repository cache keys
func RepositoryKey(repoID string) string {
	return fmt.Sprintf("%s:%s", PrefixRepository, repoID)
}

func AllRepositoriesKey() string {
	return fmt.Sprintf("%s:all", PrefixRepository)
}

// Share cache keys
func SessionSharesKey(sessionID string) string {
	return fmt.Sprintf("%s:session:%s", PrefixShare, sessionID)
}

func UserSharedSessionsKey(userID string) string {
	return fmt.Sprintf("%s:user:%s", PrefixShare, userID)
}

func ShareInvitationKey(token string) string {
	return fmt.Sprintf("%s:invitation:%s", PrefixShare, token)
}

func SessionCollaboratorsKey(sessionID string) string {
	return fmt.Sprintf("%s:session:%s:collaborators", PrefixShare, sessionID)
}

// Stats cache keys
func UserStatsKey(userID string) string {
	return fmt.Sprintf("%s:user:%s", PrefixStats, userID)
}

func GlobalStatsKey() string {
	return fmt.Sprintf("%s:global", PrefixStats)
}

func TemplateStatsKey(templateName string) string {
	return fmt.Sprintf("%s:template:%s", PrefixStats, templateName)
}

// Cache invalidation patterns
func SessionPattern() string {
	return fmt.Sprintf("%s:*", PrefixSession)
}

func UserPattern(userID string) string {
	return fmt.Sprintf("*:user:%s*", userID)
}

func TemplatePattern() string {
	return fmt.Sprintf("%s:*", PrefixTemplate)
}

func QuotaPattern() string {
	return fmt.Sprintf("%s:*", PrefixQuota)
}

// User favorites invalidation pattern (invalidates all user favorite caches)
func UserFavoritesPattern() string {
	return fmt.Sprintf("%s:favorites:*", PrefixTemplate)
}

// User-specific favorites key
func UserFavoritesKey(userID string) string {
	return fmt.Sprintf("%s:favorites:user:%s", PrefixTemplate, userID)
}
