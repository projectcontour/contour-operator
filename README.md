# contour-operator
Deploy and manage Contour using an [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/). 

## Building

To build the operator, run:

```
make manager
```

## Developing

### Prerequisites

* [Deploy](https://projectcontour.io/docs/v1.9.0/deploy-options/#kind) a [kind](https://kind.sigs.k8s.io/) cluster.

Install the Contour CRD:
```
make install
```

Run the operator locally or in the cluster .

To run the operator locally. __Note:__ This will run in the foreground, so switch to a new terminal if you want to leave
it running:
```
make run
```

To run the operator in a cluster:
```
make docker-build docker-push IMG=docker.io/<YOU_GITHUB_ID>/contour-operator:latest
make deploy IMG=docker.io/<YOU_GITHUB_ID>/contour-operator:latest
```

Verify the deployment is available:
```
$ kubectl get deploy -n contour-operator
NAME                                  READY   UP-TO-DATE   AVAILABLE   AGE
contour-operator-controller-manager   1/1     1            1           1m
```

Install an instance of the `Contour` custom resource:
```
kubectl apply -f config/samples/
```
