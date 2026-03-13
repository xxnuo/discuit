#!/bin/bash
set -e

# Start Redis
echo "Starting Redis..."
service redis-server start

# Run migrations
/app/discuit migrate run

# Build the UI
echo "Building the UI..."
cd /app/ui
npm run build
cd ..

# Start the Discuit server
echo "Starting Discuit..."
exec "$@"
