#!/bin/bash
set -e

echo "Starting Redis..."
service redis-server start

/app/discuit migrate run

echo "Starting Discuit..."
exec "$@"
