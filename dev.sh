#!/bin/bash

# Development script for Conexiuni Cluj
# Runs OTP, frontend, and backend in development mode

set -e

echo "Starting Conexiuni Cluj in development mode..."
echo ""
echo "OTP will be available at http://localhost:8080"
echo "Frontend will be available at http://localhost:5173"
echo "Backend API will be available at http://localhost:6698"
echo ""

# Start OTP in background
echo "Starting OTP server..."
bash backend/services/otp/start-otp.sh &
OTP_PID=$!

# Wait for OTP to become ready (polls /otp endpoint)
echo "Waiting for OTP to be ready..."
MAX_WAIT=600
ELAPSED=0
while [ $ELAPSED -lt $MAX_WAIT ]; do
  if curl -sf http://localhost:8080/otp > /dev/null 2>&1; then
    echo "OTP is ready!"
    break
  fi
  sleep 5
  ELAPSED=$((ELAPSED + 5))
  echo "  Still waiting for OTP... (${ELAPSED}s)"
done

if [ $ELAPSED -ge $MAX_WAIT ]; then
  echo "WARNING: OTP did not become ready within ${MAX_WAIT}s, starting backend anyway..."
fi

# Start backend in background
echo "Starting backend..."
cd backend
go run . &
BACKEND_PID=$!
cd ..

# Start frontend (this will block)
echo "Starting frontend..."
cd frontend
npm run dev

# Cleanup when frontend exits
kill $BACKEND_PID 2>/dev/null || true
kill $OTP_PID 2>/dev/null || true
