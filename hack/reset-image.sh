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

cd "${REPO}"

# Wrap sed to deal with GNU and BSD sed flags.
run::sed() {
    local -r vers="$(sed --version < /dev/null 2>&1 | grep -q GNU && echo gnu || echo bsd)"
    case "$vers" in
        gnu) sed -i "$@" ;;
        *) sed -i '' "$@" ;;
    esac
}

if grep -q "${IMAGE}" "${CONFIG_FILE}" && grep -q "${NEW_VERSION}" "${CONFIG_FILE}"; then
  echo "${CONFIG_FILE} contains ${IMAGE}:${NEW_VERSION}"
else
  echo "regenerating ${CONFIG_FILE} using kustomize..."
  cd config/manager && kustomize edit set image contour-operator="${IMAGE}:${NEW_VERSION}"
  cd "${REPO}"
fi

if grep -q "${IMAGE}:${NEW_VERSION}" "${EXAMPLE_FILE}"; then
  echo "${EXAMPLE_FILE} contains ${IMAGE}:${NEW_VERSION}"
else
  echo "regenerating ${EXAMPLE_FILE} using kustomize..."
  cd "${REPO}"
  kustomize build config/default > "${EXAMPLE_FILE}"
fi

for file in ${EXAMPLE_FILE} ${MANAGER_FILE} ; do
  if grep -q "imagePullPolicy: ${PULL_POLICY}" $file; then
    echo "$file contains imagePullPolicy: ${PULL_POLICY}"
  else
    echo "resetting imagePullPolicy to \"Always\" for $file"
    run::sed \
      "-es|imagePullPolicy: IfNotPresent|imagePullPolicy: Always|" \
      "$file"
  fi
done
