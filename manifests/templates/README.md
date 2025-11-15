# StreamSpace Templates - Minimal Defaults

This directory contains **minimal bundled templates** for offline/air-gapped deployments or quick testing.

## Full Template Catalog

For the **complete template catalog** (200+ applications), see:

🔗 **[streamspace-templates](https://github.com/JoshuaAFerguson/streamspace-templates)**

The external repository includes:
- **Browsers** (4): Firefox, Chromium, Brave, LibreWolf
- **Development** (3): VS Code, GitHub Desktop, GitQlient
- **Productivity** (2): LibreOffice, Calligra
- **Design** (6): GIMP, Krita, Inkscape, Blender, FreeCAD, KiCad
- **Media** (2): Audacity, Kdenlive
- **Gaming** (2): DuckStation, Dolphin
- **Webtop** (3): Ubuntu XFCE, Ubuntu KDE, Alpine i3

## Automatic Syncing

StreamSpace automatically syncs templates from the external repository when configured:

```yaml
# In chart/values.yaml
repositories:
  templates:
    enabled: true
    url: https://github.com/JoshuaAFerguson/streamspace-templates
    branch: main
    syncInterval: 1h
```

## Manual Installation

Deploy all templates from external repository:

```bash
kubectl apply -f https://raw.githubusercontent.com/JoshuaAFerguson/streamspace-templates/main/catalog.yaml
```

Or specific category:

```bash
kubectl apply -f https://raw.githubusercontent.com/JoshuaAFerguson/streamspace-templates/main/browsers/
```

## Adding Custom Templates

To add your own templates, see the [Template Development Guide](https://github.com/JoshuaAFerguson/streamspace-templates/blob/main/CONTRIBUTING.md).
