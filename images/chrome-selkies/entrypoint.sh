#!/bin/bash
# StreamSpace Chrome Selkies Entrypoint
#
# This script starts the Selkies-GStreamer WebRTC server with Chrome

set -e

# Configure display resolution if provided
if [ -n "$DISPLAY_WIDTH" ] && [ -n "$DISPLAY_HEIGHT" ]; then
    export DISPLAY_SIZEW=$DISPLAY_WIDTH
    export DISPLAY_SIZEH=$DISPLAY_HEIGHT
fi

# Configure encoder based on available hardware
configure_encoder() {
    # Check for NVIDIA GPU
    if [ -e /dev/nvidia0 ]; then
        echo "NVIDIA GPU detected, using NVENC encoder"
        export SELKIES_ENCODER=nvh264enc
        export SELKIES_ENABLE_NVFBC=true
        return
    fi

    # Check for Intel VA-API
    if [ -e /dev/dri/renderD128 ]; then
        echo "Intel/AMD GPU detected, using VA-API encoder"
        export SELKIES_ENCODER=vah264enc
        return
    fi

    # Fallback to software encoding
    echo "No GPU detected, using software x264 encoder"
    export SELKIES_ENCODER=x264enc
}

configure_encoder

# Print configuration
echo "================================================"
echo "StreamSpace Chrome Selkies Container"
echo "================================================"
echo "Display: ${DISPLAY_SIZEW}x${DISPLAY_SIZEH}@${DISPLAY_REFRESH}Hz"
echo "Encoder: ${SELKIES_ENCODER}"
echo "Audio: ${SELKIES_ENABLE_AUDIO}"
echo "Port: ${WEBRTC_PORT}"
echo "================================================"

# Start Selkies-GStreamer
exec selkies-gstreamer \
    --enable_audio=${SELKIES_ENABLE_AUDIO} \
    --enable_basic_auth=${SELKIES_ENABLE_BASIC_AUTH:-false} \
    --encoder=${SELKIES_ENCODER} \
    --port=${WEBRTC_PORT} \
    "$@"
