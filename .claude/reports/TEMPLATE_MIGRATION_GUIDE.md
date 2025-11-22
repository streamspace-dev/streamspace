# StreamSpace Template Migration Guide

## Overview

This guide covers migrating StreamSpace templates from the main repository to the dedicated template repository at https://github.com/JoshuaAFerguson/streamspace-templates.

## Current State

### Template Locations
- **Main Templates**: `manifests/templates/` (22 curated templates)
- **Generated Templates**: `manifests/templates-generated/` (30 auto-generated templates)
- **Sample Templates**: `controller/config/samples/` (6 sample templates)

### Template Categories
1. **Browsers** (4 templates): Brave, Chromium, Firefox, Librewolf
2. **Design** (6 templates): Blender, FreeCAD, GIMP, Inkscape, KiCAD, Krita
3. **Development** (3 templates): Code Server, GitHub Desktop, GitQlient
4. **Gaming** (2 templates): Dolphin, DuckStation
5. **Media** (2 templates): Audacity, Kdenlive
6. **Productivity** (2 templates): Calligra, LibreOffice
7. **Webtop** (3 templates): Alpine i3, Ubuntu KDE, Ubuntu XFCE

## Target Repository Structure

```
streamspace-templates/
├── README.md
├── LICENSE
├── .github/
│   └── workflows/
│       └── validate-templates.yml
├── templates/
│   ├── browsers/
│   │   ├── brave.yaml
│   │   ├── chromium.yaml
│   │   ├── firefox.yaml
│   │   └── librewolf.yaml
│   ├── design/
│   │   ├── blender.yaml
│   │   ├── freecad.yaml
│   │   ├── gimp.yaml
│   │   ├── inkscape.yaml
│   │   ├── kicad.yaml
│   │   └── krita.yaml
│   ├── development/
│   │   ├── code-server.yaml
│   │   ├── github-desktop.yaml
│   │   └── gitqlient.yaml
│   ├── gaming/
│   │   ├── dolphin.yaml
│   │   └── duckstation.yaml
│   ├── media/
│   │   ├── audacity.yaml
│   │   └── kdenlive.yaml
│   ├── productivity/
│   │   ├── calligra.yaml
│   │   └── libreoffice.yaml
│   └── webtop/
│       ├── webtop-alpine-i3.yaml
│       ├── webtop-ubuntu-kde.yaml
│       └── webtop-ubuntu-xfce.yaml
├── generated/
│   └── [auto-generated templates from LinuxServer.io]
├── icons/
│   └── [custom icons if not using external URLs]
└── scripts/
    ├── generate-templates.py
    └── validate-templates.sh
```

## Migration Steps

### Phase 1: Repository Setup

1. **Initialize Repository**
   ```bash
   cd /path/to/streamspace-templates
   git init
   git remote add origin https://github.com/JoshuaAFerguson/streamspace-templates.git
   ```

2. **Create Directory Structure**
   ```bash
   mkdir -p templates/{browsers,design,development,gaming,media,productivity,webtop}
   mkdir -p generated icons scripts
   ```

3. **Create README.md**
   ```bash
   cat > README.md << 'EOF'
   # StreamSpace Templates

   Official template repository for StreamSpace - Cloud-native desktop streaming platform.

   ## Overview

   This repository contains application templates for StreamSpace sessions. Each template defines a containerized desktop application that can be streamed via web browser.

   ## Template Categories

   - **Browsers**: Web browsers (Firefox, Chromium, Brave, etc.)
   - **Design**: 3D modeling, graphic design, CAD applications
   - **Development**: IDEs, code editors, git clients
   - **Gaming**: Emulators and gaming applications
   - **Media**: Audio/video editing software
   - **Productivity**: Office suites and productivity tools
   - **Webtop**: Full desktop environments

   ## Template Structure

   Templates are Kubernetes Custom Resources (CRDs) with the following format:

   ```yaml
   apiVersion: stream.space/v1alpha1
   kind: Template
   metadata:
     name: template-name
     namespace: workspaces
   spec:
     displayName: "Display Name"
     description: "Detailed description"
     category: "Category Name"
     icon: "https://..."
     baseImage: "docker.io/image:tag"
     defaultResources:
       memory: 2Gi
       cpu: 1000m
     ports:
       - name: vnc
         containerPort: 3000
         protocol: TCP
     env: []
     volumeMounts: []
     kasmvnc:
       enabled: true
       port: 3000
     capabilities: []
     tags: []
   ```

   ## Usage

   ### Adding to StreamSpace

   1. Navigate to **Repositories** in StreamSpace UI
   2. Click **Add Repository**
   3. Enter repository URL: `https://github.com/JoshuaAFerguson/streamspace-templates`
   4. Select branch: `main`
   5. Click **Add and Sync**

   ### Creating Templates

   See [CONTRIBUTING.md](CONTRIBUTING.md) for template creation guidelines.

   ## License

   MIT License - See [LICENSE](LICENSE) file.
   EOF
   ```

### Phase 2: Copy Templates

1. **Copy Main Templates**
   ```bash
   # From streamspace repository root
   cp manifests/templates/brave.yaml /path/to/streamspace-templates/templates/browsers/
   cp manifests/templates/chromium.yaml /path/to/streamspace-templates/templates/browsers/
   cp manifests/templates/firefox.yaml /path/to/streamspace-templates/templates/browsers/
   cp manifests/templates/librewolf.yaml /path/to/streamspace-templates/templates/browsers/

   cp manifests/templates/blender.yaml /path/to/streamspace-templates/templates/design/
   cp manifests/templates/freecad.yaml /path/to/streamspace-templates/templates/design/
   cp manifests/templates/gimp.yaml /path/to/streamspace-templates/templates/design/
   cp manifests/templates/inkscape.yaml /path/to/streamspace-templates/templates/design/
   cp manifests/templates/kicad.yaml /path/to/streamspace-templates/templates/design/
   cp manifests/templates/krita.yaml /path/to/streamspace-templates/templates/design/

   cp manifests/templates/code-server.yaml /path/to/streamspace-templates/templates/development/
   cp manifests/templates/github-desktop.yaml /path/to/streamspace-templates/templates/development/
   cp manifests/templates/gitqlient.yaml /path/to/streamspace-templates/templates/development/

   cp manifests/templates/dolphin.yaml /path/to/streamspace-templates/templates/gaming/
   cp manifests/templates/duckstation.yaml /path/to/streamspace-templates/templates/gaming/

   cp manifests/templates/audacity.yaml /path/to/streamspace-templates/templates/media/
   cp manifests/templates/kdenlive.yaml /path/to/streamspace-templates/templates/media/

   cp manifests/templates/calligra.yaml /path/to/streamspace-templates/templates/productivity/
   cp manifests/templates/libreoffice.yaml /path/to/streamspace-templates/templates/productivity/

   cp manifests/templates/webtop-alpine-i3.yaml /path/to/streamspace-templates/templates/webtop/
   cp manifests/templates/webtop-ubuntu-kde.yaml /path/to/streamspace-templates/templates/webtop/
   cp manifests/templates/webtop-ubuntu-xfce.yaml /path/to/streamspace-templates/templates/webtop/
   ```

2. **Copy Generated Templates (Optional)**
   ```bash
   cp -r manifests/templates-generated/* /path/to/streamspace-templates/generated/
   ```

3. **Copy Generation Script**
   ```bash
   cp scripts/generate-templates.py /path/to/streamspace-templates/scripts/
   ```

### Phase 3: Template Validation

1. **Create Validation Script**
   ```bash
   cat > /path/to/streamspace-templates/scripts/validate-templates.sh << 'EOF'
   #!/bin/bash
   set -e

   echo "Validating StreamSpace templates..."

   ERRORS=0

   for file in templates/**/*.yaml generated/*.yaml; do
       if [ ! -f "$file" ]; then
           continue
       fi

       echo "Validating $file..."

       # Check for required fields
       if ! grep -q "apiVersion: stream.space/v1alpha1" "$file"; then
           echo "  ERROR: Missing apiVersion in $file"
           ERRORS=$((ERRORS + 1))
       fi

       if ! grep -q "kind: Template" "$file"; then
           echo "  ERROR: Missing kind: Template in $file"
           ERRORS=$((ERRORS + 1))
       fi

       if ! grep -q "displayName:" "$file"; then
           echo "  ERROR: Missing displayName in $file"
           ERRORS=$((ERRORS + 1))
       fi

       if ! grep -q "baseImage:" "$file"; then
           echo "  ERROR: Missing baseImage in $file"
           ERRORS=$((ERRORS + 1))
       fi

       echo "  ✓ $file is valid"
   done

   if [ $ERRORS -gt 0 ]; then
       echo ""
       echo "❌ Validation failed with $ERRORS errors"
       exit 1
   else
       echo ""
       echo "✅ All templates validated successfully"
   fi
   EOF

   chmod +x /path/to/streamspace-templates/scripts/validate-templates.sh
   ```

2. **Run Validation**
   ```bash
   cd /path/to/streamspace-templates
   ./scripts/validate-templates.sh
   ```

### Phase 4: Commit and Push

1. **Initial Commit**
   ```bash
   cd /path/to/streamspace-templates
   git add .
   git commit -m "Initial commit: StreamSpace templates

   - Add 22 curated application templates across 7 categories
   - Add template generation script
   - Add validation script
   - Add comprehensive README and documentation"
   ```

2. **Push to GitHub**
   ```bash
   git branch -M main
   git push -u origin main
   ```

### Phase 5: Configure StreamSpace

1. **Add Repository via API**
   ```bash
   curl -X POST http://api.streamspace.local/api/v1/repositories \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "name": "Official Templates",
       "url": "https://github.com/JoshuaAFerguson/streamspace-templates",
       "branch": "main",
       "authType": "none"
     }'
   ```

2. **Or Add via UI**
   - Navigate to **Repositories** page
   - Click **Add Repository**
   - Fill in details:
     - Name: `Official Templates`
     - URL: `https://github.com/JoshuaAFerguson/streamspace-templates`
     - Branch: `main`
     - Auth Type: `None` (for public repo)
   - Click **Add and Sync**

3. **Verify Sync**
   ```bash
   # Check sync status
   curl http://api.streamspace.local/api/v1/repositories

   # Verify templates are loaded
   curl http://api.streamspace.local/api/v1/catalog/templates
   ```

## Template Manifest Requirements

### Mandatory Fields
- `apiVersion`: Must be `stream.space/v1alpha1`
- `kind`: Must be `Template`
- `metadata.name`: Unique identifier (lowercase, hyphens)
- `spec.displayName`: Human-readable name
- `spec.baseImage`: Docker image reference

### Recommended Fields
- `spec.description`: Detailed description (2-3 sentences)
- `spec.category`: Category for organization
- `spec.icon`: Icon URL (recommended 256x256 PNG)
- `spec.defaultResources`: Resource requests/limits
- `spec.tags`: Array of search tags

### Optional Fields
- `spec.env`: Environment variables
- `spec.volumeMounts`: Volume mount configurations
- `spec.ports`: Port definitions
- `spec.kasmvnc`: KasmVNC configuration for GUI apps
- `spec.capabilities`: Feature capabilities (Network, Audio, Clipboard, USB, Printing)

## Resource Recommendations

| Category | Memory | CPU | Notes |
|----------|--------|-----|-------|
| Browsers | 2Gi | 1000m | Adjust for heavy browsing |
| Design (3D) | 6-8Gi | 3000-4000m | GPU acceleration recommended |
| Design (2D) | 3-4Gi | 2000m | For image editing |
| Development | 4Gi | 2000m | IDE/code editors |
| Gaming | 4-8Gi | 2000-4000m | Emulators vary widely |
| Media | 4-6Gi | 2000-3000m | Video editing requires more |
| Productivity | 2-3Gi | 1000m | Office applications |
| Webtop | 4Gi | 2000m | Full desktops |

## Best Practices

### Template Naming
- Use lowercase with hyphens: `firefox-browser`, `code-server`
- Keep names concise and descriptive
- Avoid version numbers in names

### Icons
- Use 256x256 PNG format
- Use transparent backgrounds
- Host on CDN or in `icons/` directory
- Prefer official application icons

### Descriptions
- First sentence: What the application does
- Second sentence: Key features
- Third sentence: Use cases or ideal for...
- Keep under 200 characters

### Tags
- Include application name
- Include category keywords
- Include use case keywords
- Include alternative names
- Example: `["browser", "web", "privacy", "mozilla", "firefox"]`

### Categories
Use consistent categories:
- Web Browsers
- Design & Graphics
- Development Tools
- Gaming & Emulation
- Media & Audio
- Productivity & Office
- Desktop Environments

### Resource Limits
- Always specify both memory and CPU
- Use millicores for CPU (1000m = 1 CPU)
- Use standard units: Mi, Gi for memory
- Test with minimum resources
- Document GPU requirements in description

### Environment Variables
- Document required vs optional env vars
- Provide sensible defaults
- Use uppercase with underscores
- Common vars: PUID, PGID, TZ

### Version Control
- Tag releases with semantic versioning
- Create branches for major template updates
- Use pull requests for community contributions
- Document breaking changes in commit messages

## Automated Sync

StreamSpace can automatically sync repositories on a schedule:

```go
// Default sync interval: 1 hour
// Configurable via SYNC_INTERVAL environment variable
SYNC_INTERVAL=30m
```

Manual sync triggers:
- Via UI: Repositories page → Sync button
- Via API: `POST /api/v1/repositories/{id}/sync`
- Via CLI: `kubectl annotate repository {name} sync=now`

## Migration Checklist

- [ ] Create streamspace-templates repository on GitHub
- [ ] Set up directory structure
- [ ] Copy all template YAML files
- [ ] Organize into category directories
- [ ] Create README.md
- [ ] Add validation script
- [ ] Run validation tests
- [ ] Commit and push to GitHub
- [ ] Add repository in StreamSpace UI
- [ ] Verify sync completes successfully
- [ ] Test template installation
- [ ] Update main StreamSpace repo to remove local templates
- [ ] Update deployment documentation

## Post-Migration

### In Main StreamSpace Repository

1. **Remove Local Templates** (after confirming external repo works)
   ```bash
   git rm -r manifests/templates/
   git rm -r manifests/templates-generated/
   git commit -m "Remove local templates (moved to external repository)"
   ```

2. **Update Documentation**
   - Update README.md to reference template repository
   - Update deployment guides
   - Update architecture documentation

3. **Update Default Configuration**
   ```yaml
   # In deployment manifests or config
   defaultRepositories:
     - name: "Official Templates"
       url: "https://github.com/JoshuaAFerguson/streamspace-templates"
       branch: "main"
   ```

### In Template Repository

1. **Set up GitHub Actions** (optional)
   ```yaml
   # .github/workflows/validate-templates.yml
   name: Validate Templates
   on: [push, pull_request]
   jobs:
     validate:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v3
         - name: Validate Templates
           run: ./scripts/validate-templates.sh
   ```

2. **Enable Discussions** for community template requests

3. **Create CONTRIBUTING.md** with template submission guidelines

4. **Add Issue Templates** for new template requests

## Troubleshooting

### Templates Not Appearing After Sync

1. Check repository status:
   ```bash
   curl http://api.streamspace.local/api/v1/repositories
   ```

2. Check API logs:
   ```bash
   kubectl logs -n streamspace deployment/streamspace-api
   ```

3. Manual sync:
   ```bash
   curl -X POST http://api.streamspace.local/api/v1/repositories/{id}/sync
   ```

### Template Installation Fails

1. Verify template exists in catalog:
   ```bash
   curl http://api.streamspace.local/api/v1/catalog/templates
   ```

2. Check template manifest syntax

3. Verify base image is accessible

### Sync Failures

Common causes:
- Repository URL incorrect
- Branch name incorrect
- Private repo without auth credentials
- Network connectivity issues
- Invalid YAML syntax in templates

## Support

- **Issues**: https://github.com/JoshuaAFerguson/streamspace-templates/issues
- **Main Project**: https://github.com/JoshuaAFerguson/streamspace
- **Documentation**: See main project README

## License

Templates are provided under MIT License. Individual applications have their own licenses.
