#! /usr/bin/env bash

readonly HERE=$(cd "$(dirname "$0")" && pwd)
readonly REPO=$(cd "${HERE}/.." && pwd)
readonly IMAGE="docker.io/projectcontour/contour-operator"
readonly CONFIG_FILE="config/manager/kustomization.yaml"
readonly EXAMPLE_FILE="examples/operator/operator.yaml"
readonly MANAGER_FILE="config/manager/manager.yaml"
readonly PULL_POLICY="Always"
readonly PROGNAME=$(basename "$0")
readonly NEW_VERSION="$1"

if [ -z "$NEW_VERSION" ]; then
    printf "Usage: %s NEW_VERSION\n" "$PROGNAME"
    exit 1
fi

set -o errexit
set -o nounset
set -o pipefail

if grep -q "${IMAGE}" "${CONFIG_FILE}" && grep -q "${NEW_VERSION}" "${CONFIG_FILE}"; then
  echo "${CONFIG_FILE} contains ${IMAGE}:${NEW_VERSION}"
else
  echo "error: ${CONFIG_FILE} is missing ${IMAGE}:${NEW_VERSION}"
  echo "use \"make reset-image\" to reset image references"
  exit 1
fi

if grep -q "${IMAGE}:${NEW_VERSION}" "${EXAMPLE_FILE}"; then
  echo "${EXAMPLE_FILE} contains ${IMAGE}:${NEW_VERSION}"
else
  echo "error: ${EXAMPLE_FILE} is missing ${IMAGE}:${NEW_VERSION}"
  echo "use \"make reset-image\" to reset image references"
  exit 1
fi

for file in ${EXAMPLE_FILE} ${MANAGER_FILE} ; do
  if grep -q "imagePullPolicy: ${PULL_POLICY}" $file; then
    echo "$file contains imagePullPolicy: ${PULL_POLICY}"
  else
    echo "error: $file is missing imagePullPolicy: ${PULL_POLICY}"
    echo "use \"make reset-image\" to reset the image pull policy"
    exit 1
  fi
done
