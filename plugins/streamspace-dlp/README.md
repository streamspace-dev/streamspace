# StreamSpace Data Loss Prevention (DLP) Plugin

Prevent data exfiltration with comprehensive controls for clipboard, file transfers, screen capture, printing, USB devices, and network access.

## Features

### Clipboard Controls
- Direction control (disabled, to-session, from-session, bidirectional)
- Size limits
- Content filtering (regex patterns)
- Sensitive data detection (SSN, credit cards, API keys)

### File Transfer Controls
- Upload/download enable/disable
- File size limits
- File type whitelist/blacklist
- Malware scanning integration
- Content inspection

### Screen Capture & Printing
- Screen capture blocking
- Watermarking (user ID, timestamp)
- Print job controls
- Screenshot detection

### USB & Peripherals
- USB device blocking
- Audio input/output controls
- Microphone access control
- Webcam access control

### Network Access Controls
- Domain allowlist/blocklist
- IP range restrictions
- URL filtering
- DNS-based controls

### Session Controls
- Idle timeout enforcement
- Max session duration
- Access reason requirement
- Approval workflows

### Violation Management
- Real-time violation detection
- Automatic blocking
- User/admin notifications
- Audit trail
- Violation analytics

## Installation

Via **Admin → Plugins**, search "Data Loss Prevention", click Install and Enable.

## Configuration

```json
{
  "enabled": true,
  "defaultPolicy": "balanced",
  "clipboardControl": {
    "enabled": true,
    "direction": "bidirectional",
    "maxSize": 1048576
  },
  "fileTransferControl": {
    "enabled": true,
    "uploadEnabled": true,
    "downloadEnabled": true,
    "maxFileSize": 104857600,
    "scanForMalware": true
  },
  "screenCaptureControl": {
    "enabled": false,
    "watermarkEnabled": true,
    "watermarkText": "{{user_id}} - {{timestamp}}"
  },
  "deviceControl": {
    "usbEnabled": false,
    "audioEnabled": true,
    "microphoneEnabled": false,
    "webcamEnabled": false
  },
  "violationActions": {
    "alertOnViolation": true,
    "blockOnViolation": true,
    "notifyUser": true,
    "notifyAdmin": true
  }
}
```

## Policy Examples

### Strict Security Policy
```json
{
  "name": "High Security Environment",
  "clipboardDirection": "disabled",
  "fileTransferEnabled": false,
  "screenCaptureEnabled": false,
  "printingEnabled": false,
  "usbDevicesEnabled": false,
  "blockOnViolation": true
}
```

### Balanced Policy
```json
{
  "name": "Standard Security",
  "clipboardDirection": "bidirectional",
  "clipboardMaxSize": 10240,
  "fileUploadEnabled": true,
  "fileDownloadEnabled": true,
  "fileMaxSize": 10485760,
  "fileTypeBlacklist": [".exe", ".bat", ".sh"],
  "screenCaptureEnabled": true,
  "watermarkEnabled": true
}
```

## Violation Types

- **clipboard_violation** - Clipboard use blocked by policy
- **file_transfer_violation** - File transfer blocked
- **file_size_violation** - File exceeds size limit
- **file_type_violation** - File type not allowed
- **screen_capture_violation** - Screen capture attempted
- **usb_device_violation** - USB device blocked
- **network_access_violation** - Network access blocked
- **idle_timeout_violation** - Session idle timeout exceeded

## API Usage

### Create DLP Policy
```bash
POST /api/plugins/dlp/policies
{
  "name": "Finance Team DLP",
  "priority": 10,
  "applyToTeams": ["finance"],
  "clipboardEnabled": false,
  "fileDownloadEnabled": false,
  "alertOnViolation": true
}
```

### List Violations
```bash
GET /api/plugins/dlp/violations?severity=high&resolved=false
```

## Support

- Docs: https://docs.streamspace.io/plugins/dlp
- GitHub: https://github.com/JoshuaAFerguson/streamspace-plugins/issues

## License

MIT License

## Version History

- **1.0.0** (2025-01-15) - Initial release with complete DLP controls
