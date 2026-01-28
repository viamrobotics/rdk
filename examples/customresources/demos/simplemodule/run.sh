#!/bin/sh
cd `dirname $0`

go build ./

if [ "$EUID" -eq 0 ] && [ "$(uname)" == "Darwin" ]; then
  VIAM_SYSTEM_AUDIO_USER="_viamsystemaudio"
  echo "viam-server running as root on MacOS. running module as $VIAM_SYSTEM_AUDIO_USER."
  if ! id "$VIAM_SYSTEM_AUDIO_USER" >/dev/null 2>&1; then
      echo "User $VIAM_SYSTEM_AUDIO_USER does not exist. Creating MacOS role account..."
      # 497 below is an arbitary value, but role account UIDs must be between 450 and 499.
      sudo sysadminctl -addUser "$VIAM_SYSTEM_AUDIO_USER" -fullName "Viam System Audio" -roleAccount -UID 497
      if [ $? -ne 0 ]; then
          echo "Error: Failed to create user $VIAM_SYSTEM_AUDIO_USER."
          exit 1
      fi
  fi
  # The exec here is important as we want the sudo process to take over this script. If
  # you do not use exec, viam-server will be unable to send signals to your module to stop
  # it. The quotations around $@ are also necessary because we want that var-sub to happen
  # before sudo starts and gets new command line arguments.
  exec sudo -u "$VIAM_SYSTEM_AUDIO_USER" ./simplemodule "$@"
else
  exec ./simplemodule $@
fi
