#!/bin/bash
# Double-click in Finder. Downloads libraries and Florence; never calls Gemini.
set -euo pipefail
export PATH="/opt/homebrew/bin:/usr/local/bin:/Library/Frameworks/Python.framework/Versions/3.13/bin:/Library/Frameworks/Python.framework/Versions/3.12/bin:/Library/Frameworks/Python.framework/Versions/3.11/bin:$PATH"
cd "$(dirname "$0")"

finish() {
    status=$?
    if [ "$status" -ne 0 ]; then
        printf '\nSetup failed. Read the error above, then run this file again.\n'
    fi
    printf '\nPress Enter to close... '
    read -r _ || true
}
trap finish EXIT

if [ "$(uname -s)" != "Darwin" ]; then
    echo "This installer is for macOS only."
    exit 1
fi
if [ ! -f tools/setup_local_ai.py ]; then
    echo "Keep this file inside the latest project folder."
    exit 1
fi
echo "Preparing local AI. Internet is required for the first download."
echo "Photos, database, API keys and existing AI results will not be changed."

if [ -e .venv ] && [ ! -x .venv/bin/python ]; then
    echo "The existing .venv is incompatible. Rename it before running setup again."
    echo "Do not copy the Windows .venv to this Mac."
    exit 1
fi

if [ ! -x .venv/bin/python ]; then
    python_bin=""
    for candidate in python3.13 python3.12 python3.11 python3; do
        if command -v "$candidate" >/dev/null 2>&1 && \
           "$candidate" -c 'import sys; raise SystemExit(not ((3,11) <= sys.version_info[:2] <= (3,13)))' 2>/dev/null; then
            python_bin="$(command -v "$candidate")"
            break
        fi
    done
    if [ -z "$python_bin" ] && command -v brew >/dev/null 2>&1; then
        echo "Installing Python 3.13 with the existing Homebrew installation..."
        HOMEBREW_NO_ASK=1 brew install python@3.13
        python_bin="$(brew --prefix python@3.13)/bin/python3.13"
    fi
    if [ -z "$python_bin" ]; then
        echo "Install Python 3.13 from python.org first, then double-click this file again."
        open 'https://www.python.org/downloads/macos/'
        exit 1
    fi
    "$python_bin" -m venv .venv
fi

python_bin="$PWD/.venv/bin/python"
"$python_bin" -c 'import sys; assert (3,11) <= sys.version_info[:2] <= (3,13), "Use a Python 3.11-3.13 environment"'
"$python_bin" -m pip install --upgrade pip
"$python_bin" -m pip install torch torchvision 'transformers>=5,<6' Pillow safetensors huggingface_hub
"$python_bin" tools/setup_local_ai.py
echo
echo "AI setup complete. Use Start Mac App.command to open the app."
echo "The app still needs GEMINI_API_KEY in .env for Thai translation."
