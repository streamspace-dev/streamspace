package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLocalRepositoryPath(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "streamspace-templates")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("failed to create local repository dir: %v", err)
	}

	want, err := filepath.Abs(repoDir)
	if err != nil {
		t.Fatalf("failed to resolve expected absolute path: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantDir string
	}{
		{
			name:    "absolute path",
			input:   repoDir,
			wantOK:  true,
			wantDir: want,
		},
		{
			name:    "file url",
			input:   "file://" + repoDir,
			wantOK:  true,
			wantDir: want,
		},
		{
			name:   "remote url",
			input:  "https://github.com/streamspace-dev/streamspace-templates",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveLocalRepositoryPath(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%t, got %t", tt.wantOK, ok)
			}
			if got != tt.wantDir {
				t.Fatalf("expected path %q, got %q", tt.wantDir, got)
			}
		})
	}
}
