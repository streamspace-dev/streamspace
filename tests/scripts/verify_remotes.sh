#!/bin/bash
cd ~/streamspace
echo "=== Git Remotes Updated to streamspace-dev ==="
echo ""
for repo in streamspace streamspace-plugins streamspace-templates streamspace-saas streamspace-validator streamspace-builder streamspace-scribe streamspace.wiki; do
  if [ -d "$repo/.git" ]; then
    printf "%-25s " "$repo:"
    cd "$repo"
    git remote get-url origin | sed 's|https://github.com/||'
    cd ~/streamspace
  fi
done
