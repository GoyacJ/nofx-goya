#!/bin/sh
set -e

# Railway sets PORT
export PORT=${PORT:-8080}
echo "🚀 Starting NOFX on port ${PORT}..."

# Generate required keys when missing
if [ -z "$JWT_SECRET" ]; then
    export JWT_SECRET=$(openssl rand -base64 32)
fi
if [ -z "$RSA_PRIVATE_KEY" ]; then
    export RSA_PRIVATE_KEY=$(openssl genrsa 2048 2>/dev/null)
fi
if [ -z "$DATA_ENCRYPTION_KEY" ]; then
    export DATA_ENCRYPTION_KEY=$(openssl rand -base64 32)
fi

# Single process: serve API + embedded web on the same port
exec env API_SERVER_PORT="$PORT" /app/nofx
