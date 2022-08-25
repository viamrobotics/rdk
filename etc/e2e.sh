#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"

cd $ROOT_DIR/web

# Run server with test config

sudo nohup go run cmd/server/main.go -config frontend/cypress/data/test_robot_config.json &

# Wait for interface to be live before running tests
until nc -vz 127.0.0.1 8080; do sleep 2; done

# Run tests

cd frontend
npm run cypress:ci -- -c defaultCommandTimeout=10000

# Teardown

kill -9 $(lsof -t -i:8080 2> /dev/null)
