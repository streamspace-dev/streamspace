# StreamSpace Azure Blob Storage Plugin

Microsoft Azure Blob Storage backend for session recordings, snapshots, and file storage.

## Features

- **Azure Blob Storage**: Full support for Microsoft Azure Blob Storage
- **Hot/Cool/Archive Tiers**: Optimize costs with storage tiers
- **Private Endpoints**: Support for private Azure endpoints
- **Multi-Path Storage**: Separate paths for recordings, snapshots, uploads

## Installation

Admin → Plugins → "Azure Blob Storage" → Install

## Configuration

```json
{
  "enabled": true,
  "accountName": "streamspacestorage",
  "accountKey": "your-storage-account-key",
  "containerName": "streamspace",
  "storagePaths": {
    "recordings": "recordings/",
    "snapshots": "snapshots/",
    "uploads": "uploads/"
  }
}
```

## License

MIT
