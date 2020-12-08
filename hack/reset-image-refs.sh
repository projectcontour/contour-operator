#! /usr/bin/env bash

readonly HERE=$(cd "$(dirname "$0")" && pwd)
readonly REPO=$(cd "${HERE}/.." && pwd)
readonly IMAGE="docker.io/projectcontour/contour-operator"
readonly CONFIG_FILE="config/manager/kustomization.yaml"
readonly EXAMPLE_FILE="examples/operator/operator.yaml"
readonly PROGNAME=$(basename "$0")
readonly NEW_VERSION="$1"

if [ -z "$NEW_VERSION" ]; then
    printf "Usage: %s NEW_VERSION\n" "$PROGNAME"
    exit 1
fi

set -o errexit
set -o nounset
set -o pipefail

cd "${REPO}"

if grep -q "${IMAGE}" "${CONFIG_FILE}" && grep -q "${NEW_VERSION}" "${CONFIG_FILE}"; then
  echo "${CONFIG_FILE} contains ${IMAGE}:${NEW_VERSION}"
else
  echo "error: ${CONFIG_FILE} is missing ${IMAGE}:${NEW_VERSION}"
  echo "regenerating ${CONFIG_FILE} using kustomize..."
  cd config/manager && kustomize edit set image contour-operator="${IMAGE}:${NEW_VERSION}"
  cd "${REPO}"
fi

if grep -q "${IMAGE}:${NEW_VERSION}" "${EXAMPLE_FILE}"; then
  echo "${EXAMPLE_FILE} contains ${IMAGE}:${NEW_VERSION}"
else
  echo "error: ${EXAMPLE_FILE} is missing ${IMAGE}:${NEW_VERSION}"
  echo "regenerating ${EXAMPLE_FILE} using kustomize..."
  cd "${REPO}"
  kustomize build config/default > "${EXAMPLE_FILE}"
fi
