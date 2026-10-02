#!/bin/sh
set -eu
cd "$(dirname "$0")"

# First run needs network to fetch uv (Astral) and Python deps.
if ! command -v uv >/dev/null 2>&1; then
    export PATH="$HOME/.local/bin:$PATH"
    if ! command -v uv >/dev/null 2>&1; then
        echo "Installing uv..."
        curl -LsSf https://astral.sh/uv/install.sh | sh
        export PATH="$HOME/.local/bin:$PATH"
    fi
fi

VENV_DIR="${VIAM_MODULE_DATA:-$(pwd)}/.venv"
export UV_PROJECT_ENVIRONMENT="$VENV_DIR"

if [ -f pyproject.toml ]; then
    uv sync
elif [ -f requirements.txt ]; then
    uv venv "$VENV_DIR"
    uv pip install -r requirements.txt
fi

exec uv run python src/main.py "$@"
