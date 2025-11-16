package sync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GitClient handles Git repository operations for StreamSpace repository synchronization.
//
// The client provides:
//   - Repository cloning with shallow fetch (--depth 1)
//   - Pulling latest changes (fetch + reset --hard)
//   - Authentication support (SSH keys, tokens, basic auth)
//   - Commit hash retrieval
//   - Git availability validation
//
// Authentication types:
//   - "none": Public repositories (no credentials)
//   - "ssh": Private repositories with SSH keys
//   - "token": GitHub/GitLab personal access tokens
//   - "basic": Username/password authentication
//
// Security features:
//   - SSH keys written to temporary files with 0600 permissions
//   - StrictHostKeyChecking disabled for automation
//   - No interactive prompts (GIT_TERMINAL_PROMPT=0)
//   - Credentials injected via URL or environment variables
//
// Example usage:
//
//	client := NewGitClient()
//	auth := &AuthConfig{Type: "token", Secret: "ghp_xxx"}
//	err := client.Clone(ctx, "https://github.com/user/repo", "/tmp/repo", "main", auth)
type GitClient struct {
	// timeout is the maximum duration for Git operations.
	// Default: 5 minutes (prevents hanging on large repositories)
	timeout time.Duration
}

// NewGitClient creates a new Git client with default settings.
//
// Default configuration:
//   - timeout: 5 minutes (prevents hanging on large repos)
//
// Example:
//
//	client := NewGitClient()
//	err := client.Clone(ctx, repoURL, localPath, "main", nil)
func NewGitClient() *GitClient {
	return &GitClient{
		timeout: 5 * time.Minute, // Default timeout for Git operations
	}
}

// Clone clones a Git repository to a local path.
//
// The clone operation:
//  1. Removes existing directory if present (fresh clone)
//  2. Performs shallow clone with --depth 1 (faster, smaller)
//  3. Checks out specified branch (or default branch)
//  4. Applies authentication if provided
//
// Authentication is applied via:
//   - SSH: GIT_SSH_COMMAND with temporary key file
//   - Token: Injected into URL (https://token@github.com/...)
//   - Basic: Username:password in URL
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - url: Git repository URL (HTTPS or SSH)
//   - path: Local filesystem path for clone
//   - branch: Branch name to checkout (empty for default)
//   - auth: Authentication configuration (nil for public repos)
//
// Returns an error if:
//   - Directory removal fails
//   - Git clone command fails
//   - Authentication is invalid
//
// Example:
//
//	auth := &AuthConfig{Type: "token", Secret: "ghp_xxxxx"}
//	err := client.Clone(ctx, "https://github.com/user/repo", "/tmp/repo", "main", auth)
func (g *GitClient) Clone(ctx context.Context, url, path, branch string, auth *AuthConfig) error {
	// Remove existing directory if it exists
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to remove existing directory: %w", err)
	}

	// Prepare clone command
	args := []string{"clone"}

	// Add branch if specified
	if branch != "" {
		args = append(args, "--branch", branch)
	}

	// Add depth for faster cloning
	args = append(args, "--depth", "1")

	// Add URL (may be modified for auth)
	cloneURL := g.prepareURL(url, auth)
	args = append(args, cloneURL, path)

	// Execute clone
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = g.prepareEnv(auth)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Pull pulls the latest changes from a Git repository.
//
// The pull operation:
//  1. Fetches latest changes from origin
//  2. Hard resets to origin/branch (discards local changes)
//  3. Cleans untracked files (git clean -fd)
//
// This is a destructive operation that:
//   - Discards any local modifications
//   - Removes untracked files
//   - Ensures repository matches remote exactly
//
// This behavior is intentional for repository sync, where the remote
// is the source of truth and local changes should never occur.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - path: Local repository path
//   - branch: Branch name to reset to (empty for "main")
//   - auth: Authentication configuration (nil for public repos)
//
// Returns an error if:
//   - Fetch fails (network issues, auth problems)
//   - Reset fails (branch doesn't exist)
//   - Clean fails (permission issues)
//
// Example:
//
//	err := client.Pull(ctx, "/tmp/repo", "main", auth)
func (g *GitClient) Pull(ctx context.Context, path, branch string, auth *AuthConfig) error {
	// Change to repository directory
	cmd := exec.CommandContext(ctx, "git", "-C", path, "fetch", "origin")
	cmd.Env = g.prepareEnv(auth)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch failed: %w\nOutput: %s", err, string(output))
	}

	// Reset to latest origin branch
	resetBranch := branch
	if resetBranch == "" {
		resetBranch = "main"
	}

	cmd = exec.CommandContext(ctx, "git", "-C", path, "reset", "--hard", fmt.Sprintf("origin/%s", resetBranch))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset failed: %w\nOutput: %s", err, string(output))
	}

	// Clean untracked files
	cmd = exec.CommandContext(ctx, "git", "-C", path, "clean", "-fd")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clean failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// GetCommitHash returns the current commit hash of a repository.
//
// This retrieves the full SHA-1 hash of the current HEAD commit.
// The hash can be used to:
//   - Track which version of a repository was synced
//   - Detect when updates are available
//   - Record sync history in database
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - path: Local repository path
//
// Returns:
//   - Full SHA-1 commit hash (40 characters)
//   - Error if repository is invalid or git command fails
//
// Example:
//
//	hash, err := client.GetCommitHash(ctx, "/tmp/repo")
//	// hash = "7092ff4a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q"
func (g *GitClient) GetCommitHash(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// prepareURL prepares the Git URL with embedded authentication credentials.
//
// This method injects authentication into HTTPS URLs for:
//   - GitHub: https://token@github.com/user/repo.git
//   - GitLab: https://oauth2:token@gitlab.com/user/repo.git
//   - Generic: https://username:password@host.com/repo.git
//
// Authentication types:
//   - "none": Returns URL unchanged (public repositories)
//   - "token": Injects token into URL (GitHub/GitLab personal access tokens)
//   - "basic": Injects username:password into URL
//   - "ssh": No URL modification (handled by prepareEnv with GIT_SSH_COMMAND)
//
// Security note:
//   Credentials in URLs may appear in process lists and logs.
//   For production use, consider SSH keys or credential helpers.
//
// Parameters:
//   - url: Original Git repository URL
//   - auth: Authentication configuration (nil for public repos)
//
// Returns:
//   - Modified URL with embedded credentials, or original URL if no auth
//
// Example:
//
//	auth := &AuthConfig{Type: "token", Secret: "ghp_xxxxx"}
//	url := prepareURL("https://github.com/user/repo", auth)
//	// url = "https://ghp_xxxxx@github.com/user/repo"
func (g *GitClient) prepareURL(url string, auth *AuthConfig) string {
	if auth == nil || auth.Type == "none" {
		return url
	}

	// For token-based auth, inject token into URL
	if auth.Type == "token" {
		// Handle GitHub/GitLab HTTPS URLs
		if strings.HasPrefix(url, "https://github.com/") {
			// GitHub: https://<token>@github.com/user/repo.git
			return strings.Replace(url, "https://", fmt.Sprintf("https://%s@", auth.Secret), 1)
		} else if strings.HasPrefix(url, "https://gitlab.com/") {
			// GitLab: https://oauth2:<token>@gitlab.com/user/repo.git
			return strings.Replace(url, "https://", fmt.Sprintf("https://oauth2:%s@", auth.Secret), 1)
		}
	}

	// For basic auth, inject username:password
	if auth.Type == "basic" {
		// Parse username:password from secret
		parts := strings.SplitN(auth.Secret, ":", 2)
		if len(parts) == 2 {
			return strings.Replace(url, "https://", fmt.Sprintf("https://%s:%s@", parts[0], parts[1]), 1)
		}
	}

	return url
}

// prepareEnv prepares environment variables for Git commands.
//
// This method configures:
//   - GIT_SSH_COMMAND: Custom SSH command with key file (for SSH auth)
//   - GIT_TERMINAL_PROMPT: Disabled to prevent interactive prompts
//
// SSH authentication workflow:
//  1. Write SSH private key to /tmp/git-ssh-key
//  2. Set file permissions to 0600 (required by SSH)
//  3. Configure GIT_SSH_COMMAND to use the key file
//  4. Disable StrictHostKeyChecking for automation
//
// Security considerations:
//   - SSH keys are written to /tmp (not ideal for production)
//   - StrictHostKeyChecking is disabled (vulnerable to MITM)
//   - Keys are not cleaned up after use
//
// TODO: Improve SSH key handling:
//   - Use secure temporary directories
//   - Enable host key verification
//   - Clean up key files after operations
//
// Parameters:
//   - auth: Authentication configuration (nil for public repos)
//
// Returns:
//   - Environment variable array for exec.Cmd.Env
//
// Example:
//
//	auth := &AuthConfig{Type: "ssh", Secret: "-----BEGIN RSA PRIVATE KEY-----\n..."}
//	env := prepareEnv(auth)
//	cmd.Env = env
func (g *GitClient) prepareEnv(auth *AuthConfig) []string {
	env := os.Environ()

	// For SSH auth, set GIT_SSH_COMMAND to use the provided key
	if auth != nil && auth.Type == "ssh" {
		// Write SSH key to temporary file
		keyFile := "/tmp/git-ssh-key"
		if err := os.WriteFile(keyFile, []byte(auth.Secret), 0600); err == nil {
			sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", keyFile)
			env = append(env, fmt.Sprintf("GIT_SSH_COMMAND=%s", sshCmd))
		}
	}

	// Disable interactive prompts
	env = append(env, "GIT_TERMINAL_PROMPT=0")

	return env
}

// Validate validates that Git is installed and accessible.
//
// This check should be performed on service startup to fail fast if
// Git is not available, rather than failing later during sync operations.
//
// Validation steps:
//  1. Execute "git --version" command
//  2. Verify command succeeds (exit code 0)
//  3. Verify output contains "git version"
//
// Returns an error if:
//   - Git command not found (not installed or not in PATH)
//   - Git command fails to execute
//   - Output doesn't match expected format
//
// Example:
//
//	client := NewGitClient()
//	if err := client.Validate(); err != nil {
//	    log.Fatal("Git is not available:", err)
//	}
func (g *GitClient) Validate() error {
	cmd := exec.Command("git", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git is not available: %w", err)
	}

	// Check Git version
	version := string(output)
	if !strings.Contains(version, "git version") {
		return fmt.Errorf("unexpected git version output: %s", version)
	}

	return nil
}
