#!/bin/bash
set -euo pipefail
export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/local/go/bin:$HOME/go/bin:$PATH"
cd "$(dirname "$0")"
finish() {
    status=$?
    if [ "$status" -ne 0 ]; then
        printf '\nCould not start. Read the error above.\n'
    fi
    printf '\nPress Enter to close... '
    read -r _ || true
}
trap finish EXIT
if [ "$(uname -s)" != "Darwin" ]; then
    echo "This launcher is for macOS only."
    exit 1
fi
for dependency in go node npm wails; do
    if ! command -v "$dependency" >/dev/null 2>&1; then
        echo "Missing $dependency. Install the app development tools first."
        exit 1
    fi
done
if [ -x .venv/bin/python ]; then
    export PHOTO_AI_PYTHON="$PWD/.venv/bin/python"
else
    unset PHOTO_AI_PYTHON || true
fi
wails dev
