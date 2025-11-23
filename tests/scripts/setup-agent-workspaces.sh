#!/bin/bash
# Setup Multi-Agent Workspaces for StreamSpace v2.0

set -e

# Get the parent directory of current streamspace repo
PARENT_DIR="$(dirname "$(pwd)")"
REPO_URL="https://github.com/JoshuaAFerguson/streamspace.git"

echo "🏗️  Setting up multi-agent workspaces..."
echo ""

# Current directory becomes Architect workspace
echo "✓ Architect workspace: $(pwd)"
git checkout feature/streamspace-v2-agent-refactor
echo ""

# Create Builder workspace
cd "$PARENT_DIR"
if [ ! -d "streamspace-builder" ]; then
    echo "📦 Cloning Builder workspace..."
    git clone "$REPO_URL" streamspace-builder
    cd streamspace-builder
    git checkout claude/v2-builder
    echo "✓ Builder workspace: $(pwd)"
else
    echo "✓ Builder workspace already exists: $PARENT_DIR/streamspace-builder"
fi
echo ""

# Create Validator workspace
cd "$PARENT_DIR"
if [ ! -d "streamspace-validator" ]; then
    echo "📦 Cloning Validator workspace..."
    git clone "$REPO_URL" streamspace-validator
    cd streamspace-validator
    git checkout claude/v2-validator
    echo "✓ Validator workspace: $(pwd)"
else
    echo "✓ Validator workspace already exists: $PARENT_DIR/streamspace-validator"
fi
echo ""

# Create Scribe workspace
cd "$PARENT_DIR"
if [ ! -d "streamspace-scribe" ]; then
    echo "📦 Cloning Scribe workspace..."
    git clone "$REPO_URL" streamspace-scribe
    cd streamspace-scribe
    git checkout claude/v2-scribe
    echo "✓ Scribe workspace: $(pwd)"
else
    echo "✓ Scribe workspace already exists: $PARENT_DIR/streamspace-scribe"
fi
echo ""

echo "🎉 Multi-agent workspace setup complete!"
echo ""
echo "Agent Workspaces:"
echo "  Architect:  $PARENT_DIR/streamspace"
echo "  Builder:    $PARENT_DIR/streamspace-builder"
echo "  Validator:  $PARENT_DIR/streamspace-validator"
echo "  Scribe:     $PARENT_DIR/streamspace-scribe"
echo ""
echo "Usage:"
echo "  cd $PARENT_DIR/streamspace-builder    # Work as Builder"
echo "  cd $PARENT_DIR/streamspace-validator  # Work as Validator"
echo "  cd $PARENT_DIR/streamspace-scribe     # Work as Scribe"
echo "  cd $PARENT_DIR/streamspace            # Coordinate as Architect"
