#! /usr/bin/env bash

readonly HERE=$(cd "$(dirname "$0")" && pwd)
readonly REPO=$(cd "${HERE}/.." && pwd)
readonly EXAMPLE_FILE="config/samples/operator/operator.yaml"
readonly MANAGER_FILE="config/manager/manager.yaml"
readonly PULL_POLICY="Always"
readonly PROGNAME=$(basename "$0")
readonly IMAGE="$1"
readonly VERSION="$2"

if [ -z "$IMAGE" ] || [ -z "$VERSION" ]; then
    printf "Usage: %s IMAGE VERSION\n" "$PROGNAME"
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

for file in ${EXAMPLE_FILE} ${MANAGER_FILE} ; do
  if grep -q "image: ${IMAGE}:${VERSION}" $file; then
    echo "$file contains image: ${IMAGE}:${VERSION}"
  else
    echo "resetting image to \"${IMAGE}:${VERSION}\" for $file"
    run::sed \
    "-es|image: ${IMAGE}:.*$|image: ${IMAGE}:${VERSION}|" \
      "$file"
  fi
done

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
