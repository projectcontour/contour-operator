#! /usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

readonly KUSTOMIZE_VERS="v3.5.4"
readonly KUBECTL_VERS="v1.19.2"
readonly KIND_VERS="v0.9.0"
readonly KUBEBUILDER_VERS="2.3.1"

readonly PROGNAME=$(basename $0)
readonly CURL=${CURL:-curl}

# Google storage is case sensitive, so we we need to lowercase the OS.
readonly OS=$(uname | tr '[:upper:]' '[:lower:]')

usage() {
  echo "Usage: $PROGNAME INSTALLDIR"
}

download() {
    local -r url="$1"
    local -r target="$2"

    echo Downloading "$target" from "$url"
    ${CURL} --progress-bar --location  --output "$target" "$url"
}

case "$#" in
  "1")
    mkdir -p "$1"
    readonly DESTDIR=$(cd "$1" && pwd)
    ;;
  *)
    usage
    exit 64
    ;;
esac

# TODO: Remove once upstream images are available (https://github.com/projectcontour/contour/issues/3610).
# Install latest version of kind.
go get sigs.k8s.io/kind@master

# Move the $GOPATH/bin/kind binary to local since Github actions
# have their own version installed.
mv /home/runner/go/bin/kind ${DESTDIR}/kind

# Uncomment this once v0.11 of Kind is released.
# download \
#     "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERS}/kind-${OS}-amd64" \
#     "${DESTDIR}/kind"

# chmod +x  "${DESTDIR}/kind"

download \
    "https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERS}/bin/${OS}/amd64/kubectl" \
    "${DESTDIR}/kubectl"

chmod +x "${DESTDIR}/kubectl"

download \
    "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${KUSTOMIZE_VERS}/kustomize_${KUSTOMIZE_VERS}_${OS}_amd64.tar.gz" \
    "${DESTDIR}/kustomize.tgz"

tar -C "${DESTDIR}" -xf "${DESTDIR}/kustomize.tgz" kustomize
rm "${DESTDIR}/kustomize.tgz"

download \
    "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERS}/kubebuilder_${KUBEBUILDER_VERS}_${OS}_amd64.tar.gz" \
    "/tmp/kubebuilder.tgz"
tar -C /tmp/ -xzf /tmp/kubebuilder.tgz
mv /tmp/kubebuilder_${KUBEBUILDER_VERS}_${OS}_amd64 kubebuilder
sudo mv kubebuilder /usr/local/
