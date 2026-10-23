#!/bin/sh
# Installs dependencies for a reloaded module. uv, its Python, and the venv all live in
# VIAM_MODULE_DATA so they survive reloads and nothing is required on the machine beyond curl/wget.
set -e
cd `dirname $0`

UV_VERSION="0.12.23"
DEFAULT_PYTHON_VERSION="3.13"

DATA="${VIAM_MODULE_DATA:?VIAM_MODULE_DATA is not set}"
UV="$DATA/bin/uv"
VENV="$DATA/reload-venv"
export UV_CACHE_DIR="$DATA/uv-cache"
# The venv's python links into this directory, so it must persist alongside the venv.
export UV_PYTHON_INSTALL_DIR="$DATA/uv-python"

if ! "$UV" --version 2>/dev/null | grep -q "^uv $UV_VERSION"; then
    echo "Installing uv $UV_VERSION..."
    INSTALLER="https://astral.sh/uv/$UV_VERSION/install.sh"
    if command -v curl >/dev/null; then
        curl -LsSf "$INSTALLER" | UV_UNMANAGED_INSTALL="$DATA/bin" sh
    elif command -v wget >/dev/null; then
        wget -qO- "$INSTALLER" | UV_UNMANAGED_INSTALL="$DATA/bin" sh
    else
        echo "Reload needs curl or wget on the machine to install uv." >&2
        exit 1
    fi
fi

# A module's .python-version wins; uv reads it from the working directory when --python is omitted.
if [ -f .python-version ]; then
    PYTHON_REQUEST=$(cat .python-version)
    PYTHON_ARG=""
else
    PYTHON_REQUEST="$DEFAULT_PYTHON_VERSION"
    PYTHON_ARG="--python $DEFAULT_PYTHON_VERSION"
fi

# A pyproject.toml makes this a uv project installed with `uv sync`; otherwise use requirements.txt.
if [ -f pyproject.toml ]; then
    DEPS_FILES="pyproject.toml"
    if [ -f uv.lock ]; then
        DEPS_FILES="$DEPS_FILES uv.lock"
    fi
elif [ -f requirements.txt ]; then
    DEPS_FILES="requirements.txt"
else
    echo "Reload needs a pyproject.toml or requirements.txt to install dependencies." >&2
    exit 1
fi
DEPS_STATE=$(cat $DEPS_FILES | cksum)

install_deps() {
    echo "Installing Python packages..."
    if [ -f pyproject.toml ]; then
        UV_PROJECT_ENVIRONMENT="$1" "$UV" sync --managed-python --no-dev --no-install-project -q
    else
        "$UV" pip install --python "$1/bin/python" -r requirements.txt -U -q
    fi
    printf '%s' "$DEPS_STATE" > "$1/.deps-state"
}

# Build a replacement venv beside the old one and swap only on success, so a failed
# rebuild leaves the previous working venv in place.
if [ ! -x "$VENV/bin/python" ] || [ "$(cat "$VENV/.python-request" 2>/dev/null)" != "$PYTHON_REQUEST" ]; then
    NEW_VENV="$VENV.new"
    rm -rf "$NEW_VENV"
    "$UV" venv --managed-python $PYTHON_ARG "$NEW_VENV"
    install_deps "$NEW_VENV"
    printf '%s' "$PYTHON_REQUEST" > "$NEW_VENV/.python-request"
    rm -rf "$VENV"
    mv "$NEW_VENV" "$VENV"
elif [ "$(cat "$VENV/.deps-state" 2>/dev/null)" != "$DEPS_STATE" ]; then
    install_deps "$VENV"
fi

if [ -f .user_first_run ]; then
    exec "./$(cat .user_first_run)"
fi
