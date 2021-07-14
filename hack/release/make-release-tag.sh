#! /usr/bin/env bash

# make-release-tag.sh: This script assumes that you are on a branch and have
# otherwise prepared the release. It rewrites the image tag in the operator's
# deployment manifest, then creates a tag with a message containing the shortlog
# from the previous version. Note: The Envoy image is not managed by this script.

readonly PROGNAME=$(basename "$0")
readonly OLDVERS="$1"
readonly NEWVERS="$2"
readonly CONTOUR_IMG="docker.io/projectcontour/contour:$NEWVERS"
readonly OPERATOR_IMG="docker.io/projectcontour/contour-operator:$NEWVERS"
readonly OPERATOR_EXAMPLE="https://raw.githubusercontent.com/projectcontour/contour-operator/$NEWVERS/examples/operator/operator.yaml"
readonly CONTOUR_EXAMPLE="https://raw.githubusercontent.com/projectcontour/contour-operator/$NEWVERS/examples/contour/contour.yaml"
readonly TEST_EXAMPLE="https://projectcontour.io/docs/$NEWVERS/deploy-options/#test-with-ingress"
readonly CONDUCT_EXAMPLE="https://github.com/projectcontour/contour/blob/$NEWVERS/CODE_OF_CONDUCT.md"

if [ -z "$OLDVERS" ] || [ -z "$NEWVERS" ]; then
    printf "Usage: %s OLDVERS NEWVERS\n" "$PROGNAME"
    exit 1
fi

set -o errexit
set -o nounset
set -o pipefail

# If you are running this script, there's a good chance you switched
# branches, do ensure the vendor cache is current.
go mod vendor

if [ -n "$(git tag --list "$NEWVERS")" ]; then
    printf "%s: tag '%s' already exists\n" "$PROGNAME" "$NEWVERS"
    exit 1
fi

# Wrap sed to deal with GNU and BSD sed flags.
run::sed() {
    local -r vers="$(sed --version < /dev/null 2>&1 | grep -q GNU && echo gnu || echo bsd)"
    case "$vers" in
        gnu) sed -i "$@" ;;
        *) sed -i '' "$@" ;;
    esac
}

# Update the Docker image tags for the operator in the deployment manifests.
for file in config/manager/manager.yaml examples/operator/operator.yaml ; do
  # The version might be main or OLDVERS depending on whether we are
  # tagging from the release branch or from main.
  run::sed \
    "-es|docker.io/projectcontour/contour-operator:main|$OPERATOR_IMG|" \
    "-es|docker.io/projectcontour/contour-operator:$OLDVERS|$OPERATOR_IMG|" \
    "$file"
done

# Update the Docker image tags for Contour in the operator's config.
# The version might be main or OLDVERS depending on whether we are
# tagging from the release branch or from main.
run::sed \
  "-es|docker.io/projectcontour/contour:main|$CONTOUR_IMG|" \
  "-es|docker.io/projectcontour/contour:$OLDVERS|$CONTOUR_IMG|" \
  "internal/config/config.go"

# Update the operator's image pull policy. Set the pull policy with kustomize when
# https://github.com/kubernetes-sigs/kustomize/issues/1493 is fixed.
for file in config/manager/manager.yaml examples/operator/operator.yaml ; do
  echo "setting \"imagePullPolicy: IfNotPresent\" for $file"
  run::sed \
    "-es|imagePullPolicy: Always|imagePullPolicy: IfNotPresent|" \
    "$file"
done

# Update the docs with the release version.
run::sed \
  "-es|https://raw.githubusercontent.com/projectcontour/contour-operator/main/examples/operator/operator.yaml|$OPERATOR_EXAMPLE|" \
  "-es|https://raw.githubusercontent.com/projectcontour/contour-operator/$OLDVERS/examples/operator/operator.yaml|$OPERATOR_EXAMPLE|" \
  "-es|https://raw.githubusercontent.com/projectcontour/contour-operator/main/examples/contour/contour.yaml|$CONTOUR_EXAMPLE|" \
  "-es|https://raw.githubusercontent.com/projectcontour/contour-operator/$OLDVERS/examples/contour/contour.yaml|$CONTOUR_EXAMPLE|" \
  "-es|https://projectcontour.io/docs/main/deploy-options/#test-with-ingress|$TEST_EXAMPLE|" \
  "-es|https://projectcontour.io/docs/$OLDVERS/deploy-options/#test-with-ingress|$TEST_EXAMPLE|" \
  "-es|https://github.com/projectcontour/contour/blob/main/CODE_OF_CONDUCT.md|$CONDUCT_EXAMPLE|" \
  "-es|https://github.com/projectcontour/contour/blob/$OLDVERS/CODE_OF_CONDUCT.md|$CONDUCT_EXAMPLE|" \
  "README.md"

# If pushing the tag failed, then we might have already committed the
# YAML updates. The "git commit" will fail if there are no changes, so
# make sure that there are changes to commit before we do it.
cfg_changed=$(git status -s config/manager 2>&1 | grep -E -q '^\s+[MADRCU]')
example_changed=$(git status -s examples/operator 2>&1 | grep -E -q '^\s+[MADRCU]')
if $cfg_changed  || $example_changed ; then
  git commit -s -m "Update Contour Docker image to $NEWVERS." \
    config/manager/manager.yaml \
    examples/operator/operator.yaml \
    internal/config/config.go \
    README.md
fi

git tag -F - "$NEWVERS" <<EOF
Tag $NEWVERS release.

$(git shortlog "$OLDVERS..HEAD")
EOF

printf "Created tag '%s'\n" "$NEWVERS"

# People set up their remotes in different ways, so we need to check
# which one to dry run against. Choose a remote name that pushes to the
# projectcontour org repository (i.e. not the user's Github fork).
readonly REMOTE=$(git remote -v | awk '$2~/projectcontour\/contour-operator/ && $3=="(push)" {print $1}' | head -n 1)
if [ -z "$REMOTE" ]; then
    printf "%s: unable to determine remote for %s\n" "$PROGNAME" "projectcontour/contour-operator"
    exit 1
fi

readonly BRANCH=$(git branch --show-current)

printf "Testing whether commit can be pushed\n"
git push --dry-run "$REMOTE" "$BRANCH"

printf "Testing whether tag '%s' can be pushed\n" "$NEWVERS"
git push --dry-run "$REMOTE" "$NEWVERS"

printf "Run 'git push %s %s' to push the commit and then 'git push %s %s' to push the tag if you are happy\n" "$REMOTE" "$BRANCH" "$REMOTE" "$NEWVERS"
