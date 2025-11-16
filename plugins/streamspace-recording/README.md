# StreamSpace Session Recording & Playback Plugin

Record and replay sessions with multiple formats, retention policies, and compliance-driven recording.

## Features
- Multiple formats (webm, mp4, vnc)
- Automatic compliance recording
- Retention policies with auto-cleanup
- Encrypted storage
- Playback controls
- Download capability

## Installation
Admin → Plugins → "Session Recording & Playback" → Install

## Configuration
```json
{
  "enabled": true,
  "defaultFormat": "webm",
  "defaultRetentionDays": 365,
  "autoRecordForCompliance": false,
  "encryptRecordings": true
}
```

## License
MIT
