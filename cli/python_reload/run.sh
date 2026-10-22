#!/bin/sh
cd `dirname $0`
exec "$VIAM_MODULE_DATA/reload-venv/bin/python" src/main.py "$@"
