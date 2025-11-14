#!/usr/bin/env python3
"""
Generate StreamSpace Template CRs from curated catalog

Usage:
    python3 generate-from-catalog.py [--output-dir DIR]
"""

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Dict, List
import yaml


def load_catalog(catalog_file: str) -> List[Dict]:
    """Load curated app catalog from JSON file"""
    with open(catalog_file, 'r') as f:
        data = json.load(f)
        return data.get("images", [])


def generate_template(app: Dict) -> Dict:
    """Generate StreamSpace Template CR from app metadata"""
    name = app["name"]
    display_name = app["displayName"]
    description = app["description"]
    category = app["category"]
    resources = app["resources"]
    kasmvnc_enabled = app.get("kasmvnc", True)
    port = app.get("port", 3000 if kasmvnc_enabled else 8080)

    # Base image URL
    base_image = f"lscr.io/linuxserver/{name}:latest"

    # Build environment variables
    env_vars = [
        {"name": "PUID", "value": "1000"},
        {"name": "PGID", "value": "1000"},
        {"name": "TZ", "value": "America/New_York"},
    ]

    # Add custom env vars if specified
    if "env" in app:
        env_vars.extend(app["env"])

    template = {
        "apiVersion": "stream.streamspace.io/v1alpha1",
        "kind": "Template",
        "metadata": {
            "name": name,
            "namespace": "streamspace",
            "labels": {
                "app.kubernetes.io/name": name,
                "app.kubernetes.io/component": "template",
                "streamspace.io/category": category.lower().replace(" & ", "-").replace(" ", "-"),
            }
        },
        "spec": {
            "displayName": display_name,
            "description": description,
            "category": category,
            "icon": f"https://raw.githubusercontent.com/linuxserver/docker-templates/master/linuxserver.io/img/{name}-logo.png",
            "baseImage": base_image,
            "defaultResources": {
                "requests": resources,
                "limits": {
                    "memory": resources["memory"],
                    "cpu": str(int(resources["cpu"].replace("m", "")) * 2) + "m" if "m" in resources["cpu"] else resources["cpu"]
                }
            },
            "ports": [
                {
                    "name": "vnc" if kasmvnc_enabled else "http",
                    "containerPort": port,
                    "protocol": "TCP",
                }
            ],
            "env": env_vars,
            "volumeMounts": [
                {"name": "user-home", "mountPath": "/config"}
            ],
            "kasmvnc": {
                "enabled": kasmvnc_enabled,
                "port": port if kasmvnc_enabled else None,
            },
            "capabilities": ["Network", "Clipboard"] + (["Audio"] if category in ["Audio & Video", "Gaming"] else []),
            "tags": [name, category.lower().replace(" ", "-")],
        },
    }

    return template


def save_template(template: Dict, output_dir: Path) -> Path:
    """Save template to YAML file"""
    category = template["spec"]["category"]
    name = template["metadata"]["name"]

    # Create category directory
    category_slug = category.lower().replace(" & ", "-").replace(" ", "-")
    category_dir = output_dir / category_slug
    category_dir.mkdir(parents=True, exist_ok=True)

    # Save YAML file
    file_path = category_dir / f"{name}.yaml"
    with open(file_path, "w") as f:
        # Add header comment
        f.write(f"# {template['spec']['displayName']} - {template['spec']['description']}\n")
        f.write(f"# Category: {category}\n")
        f.write(f"# Base Image: {template['spec']['baseImage']}\n")
        f.write("---\n")
        yaml.dump(template, f, default_flow_style=False, sort_keys=False)

    return file_path


def main():
    parser = argparse.ArgumentParser(description="Generate StreamSpace Templates from curated catalog")
    parser.add_argument(
        "--catalog",
        help="Path to catalog JSON file",
        default="scripts/popular-apps.json",
    )
    parser.add_argument(
        "--output-dir",
        help="Output directory for templates",
        default="manifests/templates-generated",
    )

    args = parser.parse_args()

    # Load catalog
    try:
        apps = load_catalog(args.catalog)
        print(f"Loaded {len(apps)} applications from catalog")
    except FileNotFoundError:
        print(f"Error: Catalog file not found: {args.catalog}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error loading catalog: {e}", file=sys.stderr)
        sys.exit(1)

    # Generate templates
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    generated = 0
    categories = set()

    for app in apps:
        try:
            template = generate_template(app)
            file_path = save_template(template, output_dir)
            categories.add(template["spec"]["category"])
            print(f"✓ Generated: {file_path}")
            generated += 1
        except Exception as e:
            print(f"✗ Error generating template for {app.get('name', 'unknown')}: {e}", file=sys.stderr)

    print(f"\n{'='*60}")
    print(f"Summary:")
    print(f"  Generated: {generated} templates")
    print(f"  Categories: {len(categories)}")
    print(f"  Output directory: {output_dir.absolute()}")
    print(f"{'='*60}")
    print(f"\nCategories:")
    for cat in sorted(categories):
        count = sum(1 for t in apps if t["category"] == cat)
        print(f"  - {cat}: {count} templates")


if __name__ == "__main__":
    main()
