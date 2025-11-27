#!/bin/bash

echo "🔧 Setting up test environment for RillNet..."

# Create test directories
mkdir -p test-data/streams
mkdir -p test-data/peers
mkdir -p test-logs

# Set test environment variables
export RILLNET_TEST_MODE=true
export RILLNET_LOG_LEVEL=info
export RILLNET_WEBRTC_MOCK=true
export RILLNET_TEST_DATA_DIR="./test-data"

echo "✅ Test environment setup complete"
echo "   - Test mode: enabled"
echo "   - Log level: info" 
echo "   - WebRTC mock: enabled"
echo "   - Data directory: ./test-data"