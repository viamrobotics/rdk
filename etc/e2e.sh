#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"

cd $ROOT_DIR/web

# Run server with test config

nohup go run cmd/server/main.go frontend/cypress/data/test_robot_config.json &

# Remote Control Tests

cd frontend
npx cypress run
