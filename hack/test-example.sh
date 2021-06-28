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
    local attempts=0

    while [ $attempts -le $retries ]
    do
      echo "Sending http request to $url"
      resp=$(curl -w %"{http_code}" -s -o /dev/null "$url")
      if [ "$resp" = "200" ] ; then
        echo "Received http response from $url"
        RESP=true
        break
      fi
      sleep "$delay"
      attempts=$(( $attempts + 1 ))
    done
}

# Test Contour
kubectl::apply -f config/samples/operator/operator.yaml
kubectl::apply -f config/samples/contour/contour-nodeport.yaml
kubectl::apply -f https://projectcontour.io/examples/kuard.yaml
waitForHttpResponse http://local.projectcontour.io 1 100
kubectl::delete -f https://projectcontour.io/examples/kuard.yaml
kubectl::delete -f config/samples/contour/contour-nodeport.yaml
# Test Gateway
kubectl::apply -f config/samples/gateway/gateway-nodeport.yaml
kubectl::apply -f config/samples/gateway/kuard/kuard.yaml
waitForHttpResponse http://local.projectcontour.io 1 100
kubectl::delete -f config/samples/gateway/kuard/kuard.yaml
kubectl::delete -f config/samples/gateway/gateway-nodeport.yaml
kubectl::delete -f config/samples/operator/operator.yaml

if ${RESP} == false ; then
  echo "examples test passed"
  exit 0
else
  echo "examples test failed"
  exit 1
fi
