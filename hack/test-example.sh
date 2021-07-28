#! /usr/bin/env bash

# test-example.sh: An e2e test using resources from the examples directory.

readonly KUBECTL=${KUBECTL:-kubectl}
readonly CURL=${CURL:-curl}
RESP=false

kubectl::do() {
    ${KUBECTL} "$@"
}

kubectl::apply() {
    kubectl::do apply "$@"
}

kubectl::delete() {
    kubectl::do delete "$@"
}

waitForHttpResponse() {
    local -r url="$1"
    local delay=$2
    local retries=$3
    local attempts=1

    while [ $attempts -le $retries ]
    do
      echo "Sending http request to $url (attempt #$attempts)"
      resp=$(curl -w %"{http_code}" -s -o /dev/null "$url")
      if [ "$resp" = "200" ] ; then
        echo "Received http response from $url"
        RESP=true
        break
      fi
      echo "attempt #$attempts failed, sleeping"
      sleep "$delay"
      attempts=$(( $attempts + 1 ))
    done
}

# Test Contour
kubectl::apply -f examples/operator/operator.yaml
kubectl::apply -f examples/contour/contour-nodeport.yaml
kubectl::apply -f https://projectcontour.io/examples/kuard.yaml
waitForHttpResponse http://local.projectcontour.io 1 100
kubectl::delete -f https://projectcontour.io/examples/kuard.yaml
kubectl::delete -f examples/contour/contour-nodeport.yaml

# Test Gateway
kubectl::apply -f examples/gateway/gateway-nodeport.yaml
kubectl::apply -f examples/gateway/kuard/kuard.yaml
waitForHttpResponse http://local.projectcontour.io 1 100
kubectl::delete -f examples/gateway/kuard/kuard.yaml
# Delete resources in the desired order to avoid race conditions
# with finalizers.
kubectl::delete gateways --all-namespaces --all
kubectl::delete gatewayclasses --all-namespaces --all
# this line will report an error since the gateway and gatewayclass
# have already been deleted, this is not a problem.
kubectl::delete -f examples/gateway/gateway-nodeport.yaml
kubectl::delete -f examples/operator/operator.yaml

if ${RESP} == false ; then
  echo "examples test passed"
  exit 0
else
  echo "examples test failed"
  exit 1
fi
