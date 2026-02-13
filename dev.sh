#!/bin/bash

# Development script for Conexiuni Cluj
# Runs frontend and backend in development mode

set -e

echo "Starting Conexiuni Cluj in development mode..."
echo ""
echo "Frontend will be available at http://localhost:5173"
echo "Backend API will be available at http://localhost:6698"
echo ""

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
