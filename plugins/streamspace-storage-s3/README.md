# StreamSpace S3 Object Storage Plugin

AWS S3 and S3-compatible object storage backend for session recordings, snapshots, and file storage. Supports AWS S3, MinIO, DigitalOcean Spaces, Wasabi, and other S3-compatible providers.

## Features

- **AWS S3 Native**: Full support for Amazon S3
- **S3-Compatible**: Works with MinIO, DigitalOcean Spaces, Wasabi, Backblaze B2
- **Server-Side Encryption**: AES256 or AWS KMS encryption
- **Custom Endpoints**: Support for private S3 deployments
- **Path-Style URLs**: MinIO and custom S3 compatibility
- **Multi-Path Storage**: Separate paths for recordings, snapshots, uploads

## Installation

Admin → Plugins → "S3 Object Storage" → Install

## Configuration

### AWS S3

```json
{
  "enabled": true,
  "provider": "aws-s3",
  "region": "us-east-1",
  "bucket": "streamspace-storage",
  "accessKeyID": "AKIAIOSFODNN7EXAMPLE",
  "secretAccessKey": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
  "useSSL": true,
  "encryption": {
    "enabled": true,
    "algorithm": "AES256"
  }
}
```

### MinIO

```json
{
  "enabled": true,
  "provider": "minio",
  "endpoint": "https://minio.example.com",
  "region": "us-east-1",
  "bucket": "streamspace",
  "accessKeyID": "minioadmin",
  "secretAccessKey": "minioadmin",
  "useSSL": true,
  "pathStyle": true
}
```

### DigitalOcean Spaces

```json
{
  "enabled": true,
  "provider": "digitalocean-spaces",
  "endpoint": "https://nyc3.digitaloceanspaces.com",
  "region": "nyc3",
  "bucket": "streamspace",
  "accessKeyID": "your-spaces-key",
  "secretAccessKey": "your-spaces-secret",
  "useSSL": true
}
```

## License

MIT
