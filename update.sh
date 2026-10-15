#!/bin/bash

set -e

UPDATE_PBF=false
DELETE_DB=false
DELETE_LOGS=false
BUST_CACHE=false

for arg in "$@"; do
  if [ "$arg" == "--update-pbf" ]; then
    UPDATE_PBF=true
  elif [ "$arg" == "--delete-db" ]; then
    DELETE_DB=true
  elif [ "$arg" == "--delete-logs" ]; then
    DELETE_LOGS=true
  elif [ "$arg" == "--bust-cache" ]; then
    BUST_CACHE=true
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
touch keys.env
if [ -n "$(tail -c1 keys.env)" ]; then
  echo >> keys.env
fi
if ! grep -Eq '^VAPID_(PUBLIC|PRIVATE)_KEY=.+' keys.env; then
  echo "🔑 Generating VAPID keys for push notifications..."
  sed -i -E '/^VAPID_(PUBLIC|PRIVATE)_KEY=/d' keys.env
  npx --yes web-push generate-vapid-keys --json \
    | node -e 'const k = JSON.parse(require("fs").readFileSync(0, "utf8")); console.log(`VAPID_PUBLIC_KEY=${k.publicKey}\nVAPID_PRIVATE_KEY=${k.privateKey}`)' \
    >> keys.env
fi
if ! grep -q '^VAPID_SUBJECT=.' keys.env; then
  sed -i '/^VAPID_SUBJECT=/d' keys.env
  echo "VAPID_SUBJECT=https://bus.bmarian.online/" >> keys.env
fi
cp keys.env backend/keys.env
chmod 600 keys.env backend/keys.env

# The planner reads the streets around the stops straight from the country extract.
mkdir -p backend/services/osm
if [ "$UPDATE_PBF" = true ] || [ ! -s backend/services/osm/romania-latest.osm.pbf ]; then
    echo ""
    echo "🗺️ Downloading Romania PBF..."
    wget --tries=10 --waitretry=5 --retry-connrefused --retry-on-http-error=502,503,504 -O backend/services/osm/romania-latest.osm.pbf.tmp https://download.geofabrik.de/europe/romania-latest.osm.pbf
    mv backend/services/osm/romania-latest.osm.pbf.tmp backend/services/osm/romania-latest.osm.pbf
fi

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

if [ "$BUST_CACHE" = true ]; then
  echo ""
  echo "🧹 Busting cache..."
  cd backend
  ./conexiuni-cluj --bust-cache
  cd ..
fi

echo ""
echo "⚙️ Reloading systemd and restarting service..."
sudo systemctl daemon-reload
sudo systemctl restart conexiuni-cluj

echo ""
echo "🚀 Update complete!"
