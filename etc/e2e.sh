#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"

cd $ROOT_DIR

FLAG_OPEN="open"
FLAG_RUN="run"

helpFunction()
{
   echo ""
   echo "Usage: $0 -o <$FLAG_OPEN|$FLAG_RUN> [-k]"
   echo -e "\t-o Whether or not to open the cypress UI or just run the tests"
   echo -e "\t-k Does *not* try to pkill the child test-e2e process. Helpful to avoid pkill in CI."
   exit 1 # Exit script after printing help
}

# TODO(APP-2044): Flip default to true
KILL_CHILD=false

while getopts "o:k" opt
do
   case "$opt" in
      o ) OPEN="$OPTARG" ;;
      k ) KILL_CHILD=false ;;
      ? ) helpFunction ;; 
   esac
done

if [ -z "$OPEN" ] ; then
    helpFunction
elif [ $OPEN != $FLAG_OPEN ] && [ $OPEN != $FLAG_RUN ] ; then
    helpFunction
fi

# Run server with test config
nohup ./bin/test-e2e/server --config web/frontend/cypress/data/test_robot_config.json &

# Wait for interface to be live before running tests
until nc -vz 127.0.0.1 8080; do sleep 2; done

# Run tests (giving a long timeout as it can run slow in CI)
cd web/frontend

if [ $OPEN == "open" ] ; then
    npm run cypress -- -c defaultCommandTimeout=30000
elif [ $OPEN == "run" ] ; then
    npm run cypress:ci -- -c defaultCommandTimeout=30000
fi

if [ $KILL_CHILD == true ] ; then
    pkill -f test-e2e
fi
