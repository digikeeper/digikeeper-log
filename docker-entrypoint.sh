#!/bin/sh
set -e

# Create data directory if it doesn't exist
if [ -n "$LOG_STORAGE_PATH" ]; then
    mkdir -p "$LOG_STORAGE_PATH"
fi

case "${1:-server}" in
    server)
        shift 2>/dev/null || true
        exec server "$@"
        ;;
    sh|bash)
        exec "$@"
        ;;
    *)
        echo "Unknown command: $1" >&2
        echo "Usage: server | sh | bash" >&2
        exit 1
        ;;
esac
