# StreamSpace Session Snapshots & Restore Plugin

Create, manage, and restore session snapshots with scheduling and sharing.

## Features
- Session state snapshots
- Scheduled snapshots
- Snapshot restore
- Snapshot sharing
- Compression and encryption
- Auto-cleanup

## Installation
Admin → Plugins → "Session Snapshots & Restore" → Install

## Configuration
```json
{
  "enabled": true,
  "maxSnapshotsPerSession": 10,
  "defaultRetentionDays": 90,
  "compressionEnabled": true,
  "encryptSnapshots": true
}
```

## License
MIT
