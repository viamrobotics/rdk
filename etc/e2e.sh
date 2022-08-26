#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"

cd $ROOT_DIR

# Run server with test config

nohup ./bin/test-e2e/server --config web/frontend/cypress/data/test_robot_config.json &

# Wait for interface to be live before running tests
until nc -vz 127.0.0.1 8080; do sleep 2; done

# Run tests (giving a long timeout as it can run slow in CI)

cd web/frontend
npm run cypress:ci -- -c defaultCommandTimeout=30000
