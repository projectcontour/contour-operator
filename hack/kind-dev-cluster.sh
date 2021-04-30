#! /usr/bin/env bash

# kind-dev-cluster.sh: spin up a dev configuration in Kind
#
# This script starts a cluster in kind. We map
# the envoy listening ports to the host so that host traffic can
# easily be proxied.

readonly KIND=${KIND:-kind}
readonly KUBECTL=${KUBECTL:-kubectl}

# TODO: Use official upstream image once it is available
# See https://github.com/projectcontour/contour/issues/3610
readonly NODEIMAGE=${NODEIMAGE:-"stevesloka/kind-node:v1.21.0"}
readonly CLUSTER=${CLUSTER:-contour}

readonly HERE=$(cd $(dirname $0) && pwd)
readonly REPO=$(cd ${HERE}/.. && pwd)

host::addresses() {
    case $(uname -s) in
    Darwin)
        networksetup -listallhardwareports | \
            awk '/Device/{print $2}' | \
            xargs -n1 ipconfig getifaddr
        ;;
    Linux)
        ip --json addr show up primary scope global primary permanent | \
            jq -r '.[].addr_info | .[] | select(.local) | .local'
        ;;
    *)
        echo 0.0.0.0
        ;;
    esac
}

kind::cluster::list() {
    ${KIND} get clusters
}

# Emit a Kind config that maps the envoy listener ports to the host.
# ContainerPort and hostPort definitions are used for testing Contour
# nodeports.
kind::cluster::config() {
    cat <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
  extraPortMappings:
  - containerPort: 30080
    hostPort: 80
    listenAddress: "0.0.0.0"
  - containerPort: 30443
    hostPort: 443
    listenAddress: "0.0.0.0"
  - containerPort: 30081
    hostPort: 81
    listenAddress: "0.0.0.0"
  - containerPort: 30444
    hostPort: 444
    listenAddress: "0.0.0.0"
EOF
}

kind::cluster::create() {
    ${KIND} create cluster \
        --config <(kind::cluster::config) \
        --image "${NODEIMAGE}" \
        --name ${CLUSTER} \
        --wait 5m
}

kubectl::do() {
    ${KUBECTL} "$@"
}

kubectl::apply() {
    kubectl::do apply -f "$@"
}

kind::cluster::create
kubectl::do get nodes
