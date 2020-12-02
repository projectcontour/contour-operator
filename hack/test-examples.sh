#! /usr/bin/env bash

# test-examples.sh: An e2e test using the examples from
# https://projectcontour.io/

readonly KUBECTL=${KUBECTL:-kubectl}
readonly CURL=${CURL:-curl}

kubectl::do() {
    ${KUBECTL} "$@"
}

kubectl::apply() {
    kubectl::do apply -f "$@"
}

kubectl::delete() {
    kubectl::do delete -f "$@"
}

waitForHttpResponse() {
    local -r url="$1"
    local delay=$2
    local timeout=$3
    local n=0

    while [ $n -le "$timeout" ]
    do
      echo "Sending http request to $url"
      resp=$(curl -w %"{http_code}" -s -o /dev/null "$url")
      if [ "$resp" = "200" ] ; then
        echo "Received http response from $url"
        break
      fi
      sleep "$delay"
      n=($n + 1)
    done
}

kubectl::apply https://projectcontour.io/quickstart/operator.yaml
kubectl::apply https://projectcontour.io/quickstart/contour-custom-resource.yaml
kubectl::apply https://projectcontour.io/examples/kuard.yaml
waitForHttpResponse http://local.projectcontour.io 1 300
kubectl::delete https://projectcontour.io/examples/kuard.yaml
kubectl::delete https://projectcontour.io/quickstart/contour-custom-resource.yaml
kubectl::delete https://projectcontour.io/quickstart/operator.yaml
