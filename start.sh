#!/bin/sh

set -e

echo "Running db migrations"
/app/migrate -path /app/migrations -database "$POSTGRES_URL" -verbose up

echo "Starting the app"
exec "$@"