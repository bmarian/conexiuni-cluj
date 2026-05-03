#!/bin/bash

set -e

UPDATE_PBF=false

# Parse arguments
for arg in "$@"; do
  if [ "$arg" == "--update-pbf" ]; then
    UPDATE_PBF=true
  fi
done

echo "🔄 Pulling latest changes..."
git pull

echo "📦 Installing frontend dependencies..."
cd frontend
npm i
cd ..

echo "🏗️ Building Vue frontend..."
cd frontend
npm run build
cd ..

echo "🔨 Building Go backend..."
cd backend
go build -o conexiuni-cluj
cd ..

echo "📄 Updating production environment..."
sed -i 's/^ENV=.*/ENV=production/' .env
cp .env backend/.env
if [ -f keys.env ]; then
  cp keys.env backend/keys.env
fi

echo "🚌 Setting up OpenTripPlanner..."
cd backend/services/otp

# Download otp.jar if missing or incomplete
if [ ! -f otp.jar ] || [ ! -s otp.jar ]; then
    echo "📥 Downloading otp.jar..."
    wget --tries=10 --waitretry=5 --retry-connrefused --retry-on-http-error=502,503,504 -O otp.jar.tmp https://github.com/opentripplanner/OpenTripPlanner/releases/download/v2.9.0/otp-shaded-2.9.0.jar
    mv otp.jar.tmp otp.jar
fi

# Download osmosis if missing or incomplete
if [ ! -f osmosis/bin/osmosis ]; then
    echo "📥 Downloading Osmosis..."
    wget --tries=10 --waitretry=5 --retry-connrefused --retry-on-http-error=502,503,504 -O osmosis-0.49.2.tar https://github.com/openstreetmap/osmosis/releases/download/0.49.2/osmosis-0.49.2.tar
    mkdir -p osmosis
    tar -xf osmosis-0.49.2.tar -C osmosis
    rm osmosis-0.49.2.tar
    chmod +x osmosis/bin/osmosis
fi

if [ "$UPDATE_PBF" = true ]; then
    echo "🗺️ Updating PBF data..."
    mkdir -p romania
    echo "📥 Downloading Romania PBF..."
    wget --tries=10 --waitretry=5 --retry-connrefused --retry-on-http-error=502,503,504 -O romania/romania-latest.osm.pbf.tmp https://download.geofabrik.de/europe/romania-latest.osm.pbf
    mv romania/romania-latest.osm.pbf.tmp romania/romania-latest.osm.pbf
    
    echo "🗑️ Deleting old Cluj PBF..."
    rm -f cluj/cluj.pbf
    
    echo "✂️ Cropping PBF with Osmosis..."
    ./osmosis/bin/osmosis ./romania/romania-latest.osm.pbf -b=46.38,22.75,47.50,24.27 --complete-ways -o=cluj/cluj.pbf
fi

cd ../../../

echo "⚙️ Reloading systemd and restarting service..."
sudo systemctl daemon-reload
sudo systemctl restart conexiuni-cluj

echo "🚀 Update complete!"
