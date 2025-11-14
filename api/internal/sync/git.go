package sync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GitClient handles Git operations
type GitClient struct {
	timeout time.Duration
}

// NewGitClient creates a new Git client
func NewGitClient() *GitClient {
	return &GitClient{
		timeout: 5 * time.Minute, // Default timeout for Git operations
	}
}

// Clone clones a Git repository
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

// Pull pulls the latest changes from a Git repository
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

// GetCommitHash returns the current commit hash
func (g *GitClient) GetCommitHash(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// prepareURL prepares the Git URL with authentication if needed
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

// prepareEnv prepares environment variables for Git commands
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

// Validate validates that Git is available
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
