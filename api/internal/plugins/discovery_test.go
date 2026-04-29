package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPluginFileUsesManifestEntrypointWhenBundleUsesCustomSharedObjectName(t *testing.T) {
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, "streamspace-example")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	manifest := `{
  "name": "streamspace-example",
  "entrypoints": {
    "main": "bundle/plugin-runtime.so"
  }
}`
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	bundleDir := filepath.Join(pluginDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle dir: %v", err)
	}

	expectedPath := filepath.Join(bundleDir, "plugin-runtime.so")
	if err := os.WriteFile(expectedPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write shared object placeholder: %v", err)
	}

	discovery := NewPluginDiscovery(tempDir)
	if actual := discovery.findPluginFile("streamspace-example"); actual != expectedPath {
		t.Fatalf("expected %s, got %s", expectedPath, actual)
	}
}
