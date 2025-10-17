#!/usr/bin/env bash
# Ensures that all component and service APIs have a corresponding import line in 'rdk/[services|components]/register_apis/all.go'.

pkgs=(components services)
cgo_paths=(services/motion services/vision components/camera components/audioinput)

for p in "${pkgs[@]}"; do
  pushd $p > /dev/null
  relevantDirs=$(grep -rl 'resource\.RegisterAPI' * | cut -d/ -f1)
  for d in $relevantDirs; do
    expectedImport="_ \"go.viam.com/rdk/$p/$d\""
    regexp="\\s\\+$expectedImport"
    if [[ ${cgo_paths[@]} == "$p/$d" ]]; then
      if grep -q "$regexp" register_apis/*; then
        echo "Detected restricted cgo import in 'rdk/$p/register_apis'"
        exit 1
      fi
      continue
    fi
    if ! grep -q "$regexp" register_apis/*; then
      echo "Missing expected import '$expectedImport' in 'rdk/$p/register_apis'"
      exit 1
    fi
  done
  popd > /dev/null
done
