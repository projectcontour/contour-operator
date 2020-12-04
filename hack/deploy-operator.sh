#! /usr/bin/env bash

readonly HERE=$(cd "$(dirname "$0")" && pwd)
readonly REPO=$(cd "${HERE}/.." && pwd)
readonly DEFAULT_IMAGE="docker.io/projectcontour/contour-operator"
readonly PROGNAME=$(basename "$0")
readonly IMAGE="$1"
VERSION="$2"

if [ -z "$IMAGE" ] || [ -z "$VERSION" ]; then
    printf "Usage: %s IMAGE VERSION\n" "$PROGNAME"
    exit 1
fi

set -o errexit
set -o nounset
set -o pipefail

cd "${REPO}/config/manager"

if [[ ${IMAGE} = ${DEFAULT_IMAGE} ]]; then
  echo "Using tag \"main\" since image is ${IMAGE}"
  kustomize edit set image "contour-operator=${IMAGE}:main"
else
  kustomize edit set image contour-operator="${IMAGE}:${VERSION}"
fi

kustomize build "${REPO}/config/default" | kubectl apply -f -
