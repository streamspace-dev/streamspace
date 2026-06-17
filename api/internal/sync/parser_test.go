package sync

import (
	"encoding/json"
	"testing"
)

func TestParseTemplateFromStringSupportsCurrentTemplateShape(t *testing.T) {
	parser := NewTemplateParser()

	templateYAML := `apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: chrome-selkies
  annotations:
    streamspace.dev/image-status: published
spec:
  displayName: Google Chrome
  description: Chrome browser streamed via Selkies-GStreamer (WebRTC).
  category: Web Browsers
  icon: https://example.com/chrome.svg
  baseImage: ghcr.io/streamspace-dev/chrome-selkies:latest
  streamingProtocol: selkies
  defaultResources:
    requests:
      memory: 2Gi
      cpu: 1000m
    limits:
      memory: 2Gi
      cpu: 2000m
  ports:
    - name: selkies
      containerPort: 8080
      protocol: TCP
  tags:
    - browser
    - web
`

	parsed, err := parser.ParseTemplateFromString(templateYAML)
	if err != nil {
		t.Fatalf("expected template to parse, got error: %v", err)
	}

	if parsed.Name != "chrome-selkies" {
		t.Fatalf("expected name chrome-selkies, got %q", parsed.Name)
	}
	if parsed.DisplayName != "Google Chrome" {
		t.Fatalf("expected display name to be preserved, got %q", parsed.DisplayName)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal([]byte(parsed.Manifest), &manifest); err != nil {
		t.Fatalf("expected stored manifest to be valid JSON: %v", err)
	}

	metadata, ok := manifest["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metadata object in stored manifest")
	}
	annotations, ok := metadata["annotations"].(map[string]interface{})
	if !ok || annotations["streamspace.dev/image-status"] != "published" {
		t.Fatal("expected metadata.annotations to be preserved in stored manifest")
	}

	spec, ok := manifest["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("expected spec object in stored manifest")
	}
	if spec["streamingProtocol"] != "selkies" {
		t.Fatalf("expected streamingProtocol to be preserved, got %#v", spec["streamingProtocol"])
	}

	defaultResources, ok := spec["defaultResources"].(map[string]interface{})
	if !ok {
		t.Fatal("expected nested defaultResources object in stored manifest")
	}
	requests, ok := defaultResources["requests"].(map[string]interface{})
	if !ok || requests["memory"] != "2Gi" {
		t.Fatal("expected nested defaultResources.requests to be preserved")
	}
}
