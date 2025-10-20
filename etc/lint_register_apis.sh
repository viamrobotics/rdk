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
    # Check if the path is a restricted CGO path
    is_cgo_path=false
    for cp in "${cgo_paths[@]}"; do
      if [[ "$cp" == "$p/$d" ]]; then
        is_cgo_path=true
        break
      fi
    done
        if $is_cgo_path; then
      if grep -Fq "$expectedImport" register_apis/* 2>/dev/null; then
        echo "Detected restricted cgo import '$expectedImport' in 'rdk/$p/register_apis'"
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
