#!/bin/bash

set -e

UPDATE_PBF=false
DELETE_DB=false
DELETE_LOGS=false

# Parse arguments
for arg in "$@"; do
  if [ "$arg" == "--update-pbf" ]; then
    UPDATE_PBF=true
  elif [ "$arg" == "--delete-db" ]; then
    DELETE_DB=true
  elif [ "$arg" == "--delete-logs" ]; then
    DELETE_LOGS=true
  fi
done

echo "🔄 Pulling latest changes..."
git pull

echo ""
echo "📦 Installing frontend dependencies..."
cd frontend
npm i
cd ..

echo ""
echo "🏗️ Building Vue frontend..."
cd frontend
npm run build
cd ..

echo ""
echo "🔨 Building Go backend..."
cd backend
go build -o conexiuni-cluj
cd ..

echo ""
echo "📄 Updating production environment..."
sed -i 's/^ENV=.*/ENV=production/' .env
cp .env backend/.env
if [ -f keys.env ]; then
  cp keys.env backend/keys.env
fi

echo ""
echo "🚌 Setting up OpenTripPlanner..."
mkdir -p backend/services/otp
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
    tar -xf osmosis-0.49.2.tar -C osmosis --strip-components=1
    rm osmosis-0.49.2.tar
    chmod +x osmosis/bin/osmosis
fi

if [ "$UPDATE_PBF" = true ]; then
    echo ""
    echo "🗺️ Updating PBF data..."
    mkdir -p romania
    echo "📥 Downloading Romania PBF..."
    wget --tries=10 --waitretry=5 --retry-connrefused --retry-on-http-error=502,503,504 -O romania/romania-latest.osm.pbf.tmp https://download.geofabrik.de/europe/romania-latest.osm.pbf
    mv romania/romania-latest.osm.pbf.tmp romania/romania-latest.osm.pbf
    
    echo ""
    echo "🗑️ Deleting old Cluj PBF..."
    mkdir -p cluj
    rm -f cluj/cluj.pbf
    
    echo ""
    echo "✂️ Cropping PBF with Osmosis..."
    ./osmosis/bin/osmosis \
        --read-pbf file="romania/romania-latest.osm.pbf" \
        --bounding-box bottom=46.38 left=22.75 top=47.50 right=24.27 completeWays=yes \
        --write-pbf file="cluj/cluj.pbf"
fi

cd ../../../

if [ "$DELETE_DB" = true ]; then
  echo ""
  echo "🗑️ Deleting database..."
  rm -f conexiuni-cluj.db
fi

if [ "$DELETE_LOGS" = true ]; then
  echo ""
  echo "🗑️ Deleting logs folder..."
  rm -rf logs
fi

echo ""
echo "⚙️ Reloading systemd and restarting service..."
sudo systemctl daemon-reload
sudo systemctl restart conexiuni-cluj

echo ""
echo "🚀 Update complete!"
