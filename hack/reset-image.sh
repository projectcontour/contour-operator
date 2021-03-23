#! /usr/bin/env bash

readonly HERE=$(cd "$(dirname "$0")" && pwd)
readonly REPO=$(cd "${HERE}/.." && pwd)
readonly EXAMPLE_FILE="examples/operator/operator.yaml"
readonly MANAGER_FILE="config/manager/manager.yaml"
readonly PULL_POLICY="Always"
readonly PROGNAME=$(basename "$0")
readonly IMAGE="$1"
readonly VERSION="$2"
readonly OLD_VERSION="$3"

if [ -z "$IMAGE" ] || [ -z "$VERSION" ] || [ -z "$OLD_VERSION" ]; then
    printf "Usage: %s IMAGE VERSION OLD_VERSION\n" "$PROGNAME"
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
  if grep -q "image: ${IMAGE}:${OLD_VERSION}" $file; then
    echo "$file contains image: ${IMAGE}:${OLD_VERSION}"
  else
    echo "resetting image to \"${IMAGE}:${OLD_VERSION}\" for $file"
    run::sed \
    "-es|image: ${IMAGE}:${VERSION}|image: ${IMAGE}:${OLD_VERSION}|" \
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
