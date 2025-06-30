#!/usr/bin/env bash

pkgs=(components services)

for p in "${pkgs[@]}"; do
  pushd $p > /dev/null
  relevantDirs=$(grep -rl 'resource\.RegisterAPI' * | cut -d/ -f1)
  for d in $relevantDirs; do
    expectedImport="_ \"go.viam.com/rdk/$p/$d\""
    if ! grep -q "\\s\\+$expectedImport" register_apis/*; then
      echo "Missing expected import '$expectedImport' in 'rdk/$p/register_apis'"
      exit 1
    fi
  done
  popd > /dev/null
done
