package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubjectConstants(t *testing.T) {
	// Verify named subject constants exist
	subjects := map[string]string{
		"SessionCreate":    SubjectSessionCreate,
		"SessionDelete":    SubjectSessionDelete,
		"SessionHibernate": SubjectSessionHibernate,
		"SessionWake":      SubjectSessionWake,
		"AppInstall":       SubjectAppInstall,
		"AppUninstall":     SubjectAppUninstall,
		"TemplateCreate":   SubjectTemplateCreate,
		"TemplateDelete":   SubjectTemplateDelete,
		"NodeCordon":       SubjectNodeCordon,
		"NodeUncordon":     SubjectNodeUncordon,
		"NodeDrain":        SubjectNodeDrain,
	}

	for name, subject := range subjects {
		assert.NotEmpty(t, subject, "Subject %s should not be empty", name)
		assert.Contains(t, subject, "streamspace", "Subject %s should contain 'streamspace'", name)
	}
}

func TestSubjectWithPlatform(t *testing.T) {
	tests := []struct {
		name     string
		subject  string
		platform string
		expected string
	}{
		{
			name:     "Kubernetes platform",
			subject:  "streamspace.session.create",
			platform: PlatformKubernetes,
			expected: "streamspace.session.create.kubernetes",
		},
		{
			name:     "Docker platform",
			subject:  "streamspace.app.install",
			platform: PlatformDocker,
			expected: "streamspace.app.install.docker",
		},
		{
			name:     "HyperV platform",
			subject:  "streamspace.template.create",
			platform: PlatformHyperV,
			expected: "streamspace.template.create.hyperv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubjectWithPlatform(tt.subject, tt.platform)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubjectParsing(t *testing.T) {
	// Verify that subjects follow naming convention
	t.Run("Session subjects", func(t *testing.T) {
		assert.Contains(t, SubjectSessionCreate, ".session.")
		assert.Contains(t, SubjectSessionDelete, ".session.")
		assert.Contains(t, SubjectSessionHibernate, ".session.")
	})

	t.Run("App subjects", func(t *testing.T) {
		assert.Contains(t, SubjectAppInstall, ".app.")
		assert.Contains(t, SubjectAppUninstall, ".app.")
	})

	t.Run("Template subjects", func(t *testing.T) {
		assert.Contains(t, SubjectTemplateCreate, ".template.")
		assert.Contains(t, SubjectTemplateDelete, ".template.")
	})

	t.Run("Node subjects", func(t *testing.T) {
		assert.Contains(t, SubjectNodeCordon, ".node.")
		assert.Contains(t, SubjectNodeUncordon, ".node.")
		assert.Contains(t, SubjectNodeDrain, ".node.")
	})
}
