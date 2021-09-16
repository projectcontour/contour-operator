#! /usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

readonly HERE=$(cd $(dirname $0) && pwd)
readonly REPO=$(cd ${HERE}/../.. && pwd)

echo Current Git Branch
git branch --show-current

if git status -s ${REPO} 2>&1 | grep -E -q '^\s+[MADRCU]'
then
	echo Uncommitted changes in generated sources:
	git status -s ${REPO}
	exit 1
fi
