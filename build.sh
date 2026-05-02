#!/bin/bash

set -e

echo "Building Conexiuni Cluj..."

echo "📦 Building Vue frontend..."
cd frontend
npm run build
cd ..

echo "🔨 Building Go backend..."
cd backend
go build -o conexiuni-cluj
cd ..

echo "📄 Copying env files next to the binary..."
sed -i 's/^ENV=.*/ENV=production/' .env
cp .env backend/.env
if [ -f keys.env ]; then
  cp keys.env backend/keys.env
fi

echo "✅ Build complete!"
echo ""
echo "To run the server:"
echo "  cd backend && ./conexiuni-cluj"
echo ""
echo "The server will be available at http://localhost:6698"
