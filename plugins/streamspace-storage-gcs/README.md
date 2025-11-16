# StreamSpace Google Cloud Storage Plugin

Google Cloud Storage backend for session recordings, snapshots, and file storage.

## Features

- **Google Cloud Storage**: Full support for GCS
- **Service Account Authentication**: Secure authentication with service accounts
- **Storage Classes**: Support for Standard, Nearline, Coldline, Archive
- **Multi-Region**: Support for multi-region buckets
- **Multi-Path Storage**: Separate paths for recordings, snapshots, uploads

## Installation

Admin → Plugins → "Google Cloud Storage" → Install

## Configuration

### Create Service Account

1. Go to **IAM & Admin → Service Accounts** in Google Cloud Console
2. Create a new service account with **Storage Object Admin** role
3. Create and download JSON key
4. Paste JSON content into plugin configuration

### Configure Plugin

```json
{
  "enabled": true,
  "projectID": "your-gcp-project",
  "bucketName": "streamspace-storage",
  "credentialsJSON": "{ \"type\": \"service_account\", ... }",
  "storagePaths": {
    "recordings": "recordings/",
    "snapshots": "snapshots/",
    "uploads": "uploads/"
  }
}
```

## License

MIT
