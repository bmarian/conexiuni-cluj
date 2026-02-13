#!/bin/bash

# Build script for Conexiuni Cluj

set -e  # Exit on error

echo "Building Conexiuni Cluj..."

# Build frontend
echo "📦 Building Vue frontend..."
cd frontend
npm run build
cd ..

# Build backend
echo "🔨 Building Go backend..."
cd backend
go build -o conexiuni-cluj
cd ..

echo "✅ Build complete!"
echo ""
echo "To run the server:"
echo "  cd backend && ./conexiuni-cluj"
echo ""
echo "The server will be available at http://localhost:6698"
