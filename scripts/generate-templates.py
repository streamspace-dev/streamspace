#!/usr/bin/env python3
"""
Generate StreamSpace Template CRs from LinuxServer.io API

Usage:
    python3 generate-templates.py [--category CATEGORY] [--output-dir DIR]

Examples:
    # Generate all templates
    python3 generate-templates.py

    # Generate only browser templates
    python3 generate-templates.py --category "Web Browsers"

    # Output to specific directory
    python3 generate-templates.py --output-dir /tmp/templates
"""

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Dict, List
import urllib.request
import yaml

# LinuxServer.io API endpoint
API_URL = "https://api.linuxserver.io/api/v1/images"

# Category mapping
CATEGORY_MAP = {
    "Network & DNS": "Networking",
    "Media Servers & Music": "Media",
    "Chat & Social": "Communication",
    "Monitoring": "Monitoring",
    "Audio Processing": "Audio & Video",
    "Family": "Lifestyle",
    "3D Printing": "Design & Graphics",
    "Media Management": "Media",
    "Music": "Media",
    "Finance": "Productivity",
    "3D Modeling": "Design & Graphics",
    "Science": "Science & Education",
    "Content Management": "Productivity",
    "Web Browser": "Web Browsers",
    "Books": "Productivity",
    "Documents": "Productivity",
    "Web Tools & Automation": "Automation",
    "Programming": "Development",
    "FTP": "File Management",
    "Downloaders": "File Management",
    "Storage & Monitoring": "System Utilities",
    "Games": "Gaming",
    "Media Requesters": "Media",
    "Administration & Storage": "System Utilities",
    "Machine Learning": "AI & ML",
    "RSS & Social": "Communication",
    "Remote Desktop & Security": "Remote Access",
    "Remote Desktop & Business": "Remote Access",
    "Home Automation": "Automation",
    "Media Tools": "Audio & Video",
    "Image Editor": "Design & Graphics",
    "Photos": "Design & Graphics",
    "Password Manager": "Security",
    "Video Editor": "Audio & Video",
    "Recipes": "Lifestyle",
    "Administration & Security": "Security",
    "IRC": "Communication",
    "Databases": "Development",
}

# Resource estimates by category
RESOURCE_DEFAULTS = {
    "Web Browsers": {"memory": "2Gi", "cpu": "1000m"},
    "Development": {"memory": "4Gi", "cpu": "2000m"},
    "Design & Graphics": {"memory": "4Gi", "cpu": "2000m"},
    "Audio & Video": {"memory": "3Gi", "cpu": "1500m"},
    "Gaming": {"memory": "4Gi", "cpu": "2000m"},
    "Productivity": {"memory": "3Gi", "cpu": "1500m"},
    "Media": {"memory": "2Gi", "cpu": "1000m"},
    "default": {"memory": "2Gi", "cpu": "1000m"},
}

# Special handling for certain images
SPECIAL_CONFIGS = {
    "webtop": {
        "description": "Full Linux desktop environment accessible via web browser. Available in multiple distributions and desktop environments.",
        "category": "Desktop Environments",
        "resources": {"memory": "4Gi", "cpu": "2000m"},
    },
    "kasm": {
        "description": "Kasm Workspaces platform for streaming containerized apps and desktops to the browser.",
        "skip": True,  # Skip, we're replacing this
    },
}


def fetch_images() -> List[Dict]:
    """Fetch image catalog from LinuxServer.io API"""
    print(f"Fetching image catalog from {API_URL}...")
    try:
        with urllib.request.urlopen(API_URL) as response:
            data = json.loads(response.read().decode())
            return data.get("images", [])
    except Exception as e:
        print(f"Error fetching images: {e}", file=sys.stderr)
        sys.exit(1)


def normalize_category(raw_category: str) -> str:
    """Normalize category name"""
    return CATEGORY_MAP.get(raw_category, raw_category or "Uncategorized")


def get_resources(category: str, image_name: str) -> Dict[str, str]:
    """Get resource defaults for image"""
    if image_name in SPECIAL_CONFIGS:
        return SPECIAL_CONFIGS[image_name].get("resources", RESOURCE_DEFAULTS["default"])
    return RESOURCE_DEFAULTS.get(category, RESOURCE_DEFAULTS["default"])


def should_skip(image_name: str) -> bool:
    """Check if image should be skipped"""
    return SPECIAL_CONFIGS.get(image_name, {}).get("skip", False)


def generate_template(image: Dict) -> Dict:
    """Generate StreamSpace Template CR from image metadata"""
    name = image.get("name", "").lower().replace("/", "-")
    display_name = image.get("name", "Unknown").replace("linuxserver/", "").title()
    raw_category = image.get("category", "")
    category = normalize_category(raw_category)

    # Check for special config
    special = SPECIAL_CONFIGS.get(name.replace("linuxserver-", ""), {})

    description = special.get("description") or image.get("description", f"{display_name} containerized application")
    resources = get_resources(category, name)

    # Determine if it uses KasmVNC (most linuxserver GUI apps do)
    kasmvnc_enabled = "desktop" in description.lower() or "gui" in description.lower() or category in ["Web Browsers", "Design & Graphics", "Gaming", "Productivity", "Desktop Environments"]

    # Base image URL
    base_image = f"lscr.io/linuxserver/{name.replace('linuxserver-', '')}:latest"

    template = {
        "apiVersion": "stream.streamspace.io/v1alpha1",
        "kind": "Template",
        "metadata": {
            "name": name.replace("linuxserver-", ""),
            "namespace": "streamspace",
        },
        "spec": {
            "displayName": display_name,
            "description": description[:500],  # Truncate if too long
            "category": category,
            "icon": f"https://raw.githubusercontent.com/linuxserver/docker-templates/master/linuxserver.io/img/{name.replace('linuxserver-', '')}-logo.png",
            "baseImage": base_image,
            "defaultResources": resources,
            "ports": [
                {
                    "name": "vnc" if kasmvnc_enabled else "http",
                    "containerPort": 3000 if kasmvnc_enabled else 8080,
                    "protocol": "TCP",
                }
            ],
            "env": [
                {"name": "PUID", "value": "1000"},
                {"name": "PGID", "value": "1000"},
                {"name": "TZ", "value": "America/New_York"},
            ],
            "volumeMounts": [
                {"name": "user-home", "mountPath": "/config"}
            ],
            "kasmvnc": {
                "enabled": kasmvnc_enabled,
                "port": 3000 if kasmvnc_enabled else 8080,
            },
            "capabilities": ["Network", "Clipboard"],
            "tags": [name.replace("linuxserver-", ""), category.lower()],
        },
    }

    return template


def save_template(template: Dict, output_dir: Path):
    """Save template to YAML file"""
    category = template["spec"]["category"]
    name = template["metadata"]["name"]

    # Create category directory
    category_dir = output_dir / category.lower().replace(" & ", "-").replace(" ", "-")
    category_dir.mkdir(parents=True, exist_ok=True)

    # Save YAML file
    file_path = category_dir / f"{name}.yaml"
    with open(file_path, "w") as f:
        yaml.dump(template, f, default_flow_style=False, sort_keys=False)

    return file_path


def main():
    parser = argparse.ArgumentParser(description="Generate StreamSpace Template CRs from LinuxServer.io")
    parser.add_argument(
        "--category",
        help="Filter by category (e.g., 'Web Browsers', 'Development')",
        default=None,
    )
    parser.add_argument(
        "--output-dir",
        help="Output directory for templates",
        default="manifests/templates-generated",
    )
    parser.add_argument(
        "--list-categories",
        action="store_true",
        help="List all available categories and exit",
    )

    args = parser.parse_args()

    # Fetch images
    images = fetch_images()
    print(f"Fetched {len(images)} images")

    if args.list_categories:
        categories = set()
        for img in images:
            raw_cat = img.get("category", "")
            categories.add(normalize_category(raw_cat))
        print("\nAvailable categories:")
        for cat in sorted(categories):
            print(f"  - {cat}")
        sys.exit(0)

    # Filter by category if specified
    if args.category:
        images = [img for img in images if normalize_category(img.get("category", "")) == args.category]
        print(f"Filtered to {len(images)} images in category '{args.category}'")

    # Generate templates
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    generated = 0
    skipped = 0

    for image in images:
        name = image.get("name", "").lower()

        if should_skip(name):
            print(f"Skipping {name} (special config)")
            skipped += 1
            continue

        try:
            template = generate_template(image)
            file_path = save_template(template, output_dir)
            print(f"Generated: {file_path}")
            generated += 1
        except Exception as e:
            print(f"Error generating template for {name}: {e}", file=sys.stderr)
            skipped += 1

    print(f"\nSummary:")
    print(f"  Generated: {generated} templates")
    print(f"  Skipped: {skipped} images")
    print(f"  Output directory: {output_dir.absolute()}")


if __name__ == "__main__":
    main()
