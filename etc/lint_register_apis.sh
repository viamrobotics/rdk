#!/usr/bin/env bash
# Ensures that all component and service APIs have a corresponding import line
# in 'rdk/[services|components]/register_apis/all.go'.

# The directories in the `pkgs` array contain packages that register API models
# with go init functions. This script will verify that all such packages have
# underscore imports in the corresponding `register_apis` subpackage, which
# provides a convenient way for modules and other client code to register all
# known API models with fewer imports.
pkgs=(components services)

# `cgo_paths` contains a list of packages that register API models but depend
# on cgo. For these packages we invert the usual logic: cgo packages should
# _not_ be included in the `register_apis` package so that the `register_apis`
# package can be used by modules and other client code without imposing a
# dependency on cgo, further requiring installed libraries and properly
# configured CGO_... environment variables for Go compilation to succeed.
cgo_paths=(services/motion services/vision components/camera)

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
