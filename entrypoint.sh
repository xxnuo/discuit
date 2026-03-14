#!/bin/bash
set -e

echo "Starting Redis..."
service redis-server start

/app/discuit migrate run

echo "Building the UI..."
cd /app/ui
pnpm build
cd ..

echo "Starting Discuit..."
exec "$@"
