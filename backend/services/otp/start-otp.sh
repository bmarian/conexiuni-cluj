#!/bin/bash
# Start the local OpenTripPlanner server.
# This script ensures OTP is built and running before the application starts.
# Usage: ./start-otp.sh
#
# OTP will listen on port 8080 by default.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OTP_JAR="$SCRIPT_DIR/otp-shaded-2.9.0.jar"
DATA_DIR="$SCRIPT_DIR/cluj"

if [ ! -f "$OTP_JAR" ]; then
  echo "ERROR: OTP jar not found at $OTP_JAR"
  echo "Place otp-shaded-2.9.0.jar in $(dirname "$OTP_JAR")"
  exit 1
fi

if [ ! -d "$DATA_DIR" ]; then
  echo "ERROR: OTP data directory not found at $DATA_DIR"
  echo "Create it and place gtfs.zip and .pbf files inside."
  exit 1
fi

echo "Starting OpenTripPlanner..."
echo "  JAR:  $OTP_JAR"
echo "  Data: $DATA_DIR"
echo ""

exec java -Xmx2G -jar "$OTP_JAR" --build --serve "$DATA_DIR"
