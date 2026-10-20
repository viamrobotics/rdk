#!/bin/sh
cd `dirname $0`

# Cloud reload ships source to run interpreted, skipping the slow PyInstaller freeze.
if [ -f .viam_reload ]; then
    STAGE=dist/reload
    rm -rf $STAGE && mkdir -p $STAGE
    cp -R src requirements.txt $STAGE/
    cp reload/run.sh $STAGE/reload_run.sh
    cp reload/first_run.sh $STAGE/reload_first_run.sh
    if [ -f .python-version ]; then
        cp .python-version $STAGE/
    fi
    USER_FIRST_RUN=$(python3 -c "import json; print(json.load(open('meta.json')).get('first_run', ''))")
    if [ -n "$USER_FIRST_RUN" ] && [ -f "$USER_FIRST_RUN" ]; then
        mkdir -p "$STAGE/$(dirname "$USER_FIRST_RUN")"
        cp "$USER_FIRST_RUN" "$STAGE/$USER_FIRST_RUN"
        echo "$USER_FIRST_RUN" > $STAGE/.user_first_run
    fi
    python3 -c "
import json
meta = json.load(open('meta.json'))
meta['entrypoint'] = 'reload_run.sh'
meta['first_run'] = 'reload_first_run.sh'
json.dump(meta, open('$STAGE/meta.json', 'w'), indent=2)
"
    tar -czvf dist/archive.tar.gz -C $STAGE .
    exit 0
fi

# Create a virtual environment to run our code
VENV_NAME="venv"
PYTHON="$VENV_NAME/bin/python"

if ! $PYTHON -m pip install pyinstaller -Uqq; then
    exit 1
fi

$PYTHON -m PyInstaller --onefile --collect-all viam --hidden-import="googleapiclient" src/main.py

TAR_FILES="meta.json ./dist/main"
FIRST_RUN=$($PYTHON -c "import json; print(json.load(open('meta.json')).get('first_run', ''))" 2>/dev/null)
if [ -n "$FIRST_RUN" ] && [ -f "$FIRST_RUN" ]; then
    TAR_FILES="$TAR_FILES $FIRST_RUN"
fi
tar -czvf dist/archive.tar.gz $TAR_FILES
