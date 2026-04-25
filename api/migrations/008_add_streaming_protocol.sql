-- Migration 008: Add streaming protocol support
-- Purpose: Support multiple streaming protocols (VNC, Selkies, Guacamole, etc.)
--
-- Adds fields to track streaming protocol type and port for each session.
-- This enables StreamSpace to support various streaming technologies beyond VNC.

-- Add streaming_protocol to sessions table
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS streaming_protocol VARCHAR(50) DEFAULT 'vnc';

-- Add streaming_port to sessions table
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS streaming_port INTEGER DEFAULT 5900;

-- Add streaming_path to sessions table (for URL-based protocols like Selkies)
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS streaming_path VARCHAR(255);

-- Add index for faster protocol-based queries
CREATE INDEX IF NOT EXISTS idx_sessions_streaming_protocol
ON sessions(streaming_protocol);

-- Add comments
COMMENT ON COLUMN sessions.streaming_protocol IS
'Streaming protocol type: vnc, selkies, guacamole, x2go, rdp, etc.';

COMMENT ON COLUMN sessions.streaming_port IS
'Port number for streaming service (VNC: 5900, Selkies: 3000/8082, etc.)';

COMMENT ON COLUMN sessions.streaming_path IS
'URL path for HTTP-based streaming protocols (e.g., /websockify for Selkies)';

-- Update existing sessions to have explicit VNC protocol
UPDATE sessions
SET streaming_protocol = 'vnc',
    streaming_port = 5900
WHERE streaming_protocol IS NULL;
