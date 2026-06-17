package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDefaultRepositoryURLUsesEnvOverride(t *testing.T) {
	t.Setenv("DEFAULT_TEMPLATE_REPOSITORY_URL", "https://example.com/custom/templates.git")

	got, explicit := resolveDefaultRepositoryURL(
		"DEFAULT_TEMPLATE_REPOSITORY_URL",
		"streamspace-templates",
		officialTemplatesRepoURL,
	)
	if !explicit {
		t.Fatal("expected env override to be marked explicit")
	}
	if got != "https://example.com/custom/templates.git" {
		t.Fatalf("expected env override URL, got %q", got)
	}
}

func TestDiscoverSiblingRepositoryDirFindsWorkspaceSibling(t *testing.T) {
	root := t.TempDir()
	apiDir := filepath.Join(root, "streamspace", "api")
	siblingRepo := filepath.Join(root, "streamspace-templates")

	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}
	if err := os.MkdirAll(siblingRepo, 0o755); err != nil {
		t.Fatalf("failed to create sibling repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(siblingRepo, "catalog.yaml"), []byte("apiVersion: v1\n"), 0o644); err != nil {
		t.Fatalf("failed to create catalog marker: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(apiDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	got, ok := discoverSiblingRepositoryDir("streamspace-templates")
	if !ok {
		t.Fatal("expected sibling repository to be discovered")
	}

	want, err := filepath.Abs(siblingRepo)
	if err != nil {
		t.Fatalf("failed to resolve expected absolute path: %v", err)
	}
	want, err = filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("failed to normalize expected path: %v", err)
	}
	got, err = filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("failed to normalize discovered path: %v", err)
	}
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestIsManagedDefaultRepositoryURLRecognizesLegacyAndLocalDefaults(t *testing.T) {
	repo := defaultRepositoryConfig{
		url:            officialTemplatesRepoURL,
		siblingDirName: "streamspace-templates",
		managedURLs:    []string{officialTemplatesRepoURL, legacyTemplatesRepoURL},
	}

	if !isManagedDefaultRepositoryURL(legacyTemplatesRepoURL, repo) {
		t.Fatal("expected legacy official URL to be treated as managed")
	}
	if !isManagedDefaultRepositoryURL("/tmp/streamspace-templates", repo) {
		t.Fatal("expected local sibling path to be treated as managed")
	}
	if isManagedDefaultRepositoryURL("https://example.com/custom/templates.git", repo) {
		t.Fatal("did not expect arbitrary custom URL to be treated as managed")
	}
}
