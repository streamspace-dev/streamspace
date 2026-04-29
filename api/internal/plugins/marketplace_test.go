package plugins

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestFetchCatalogSupportsYAMLCatalogWithManifestHydration(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://plugins.example/catalog.yaml":
			return textResponse(`
spec:
  plugins:
    - name: streamspace-slack
      version: "1.0.0"
      displayName: Slack Integration
      description: Slack notifications
      author: StreamSpace Team
      category: official
      path: streamspace-slack
      tags: [notifications, slack]
`)
		case "https://plugins.example/streamspace-slack/manifest.json":
			return textResponse(`{
  "name": "streamspace-slack",
  "version": "1.0.1",
  "displayName": "Slack Integration",
  "description": "Send notifications to Slack",
  "author": "StreamSpace Team",
  "category": "Integrations",
  "icon": "slack.png",
  "tags": ["notifications", "slack"],
  "entrypoints": {"main": "slack_plugin.go"}
}`)
		default:
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
				Header:     make(http.Header),
			}, nil
		}
	})
	defer func() {
		http.DefaultTransport = oldTransport
	}()

	marketplace := &PluginMarketplace{
		repositoryURL: "https://plugins.example",
	}

	plugins, err := marketplace.fetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("fetchCatalog returned error: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	plugin := plugins[0]
	if plugin.Name != "streamspace-slack" {
		t.Fatalf("expected plugin name streamspace-slack, got %s", plugin.Name)
	}
	if plugin.Version != "1.0.0" {
		t.Fatalf("expected catalog version to be preserved, got %s", plugin.Version)
	}
	if plugin.Manifest.Entrypoints.Main != "slack_plugin.go" {
		t.Fatalf("expected manifest main entrypoint slack_plugin.go, got %s", plugin.Manifest.Entrypoints.Main)
	}
	expectedIconURL := "https://plugins.example/streamspace-slack/slack.png"
	if plugin.IconURL != expectedIconURL {
		t.Fatalf("expected icon URL %s, got %s", expectedIconURL, plugin.IconURL)
	}
}

func TestDownloadPluginFilesRejectsSourceOnlyManifestEntrypoints(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://plugins.example/streamspace-slack/manifest.json":
			return textResponse(`{
  "name": "streamspace-slack",
  "entrypoints": {"main": "slack_plugin.go"}
}`)
		case "https://plugins.example/streamspace-slack/README.md":
			return textResponse("# Slack")
		default:
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
				Header:     make(http.Header),
			}, nil
		}
	})
	defer func() {
		http.DefaultTransport = oldTransport
	}()

	pluginDir := t.TempDir()
	marketplace := &PluginMarketplace{
		repositoryURL: "https://plugins.example",
	}

	err := marketplace.downloadPluginFiles(&MarketplacePlugin{
		Name: "streamspace-slack",
		Path: "streamspace-slack",
	}, pluginDir)
	if err == nil || !strings.Contains(err.Error(), ".so entrypoint") {
		t.Fatalf("expected .so entrypoint error, got %v", err)
	}

	if _, statErr := os.Stat(pluginDir + "/manifest.json"); statErr != nil {
		t.Fatalf("expected manifest.json to be downloaded before validation: %v", statErr)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func textResponse(body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}
