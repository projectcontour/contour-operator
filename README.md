# contour-operator
Welcome to the Contour Operator project. Contour Operator deploys and manages Contour using an
[operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/). 

## Get Started

### Prerequisites

* A [Kubernetes](https://kubernetes.io/) cluster
* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) installed
* [Kustomize](https://kustomize.io/) installed
* [Make](https://www.gnu.org/software/make/) installed
* Clone [this](https://github.com/projectcontour/contour-operator.git) repo

Install the Contour Operator & Contour CRDs:
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
make deploy
```

Verify the deployment is available (not needed if running the operator locally):
```
$ kubectl get deploy -n contour-operator
NAME                                  READY   UP-TO-DATE   AVAILABLE   AGE
contour-operator-controller-manager   1/1     1            1           1m
```

Install an instance of the `Contour` custom resource:
```
kubectl apply -f config/samples/
```

Verify the Contour and Envoy pods are running/completed:
```
$ kubectl get po -n projectcontour
NAME                       READY   STATUS      RESTARTS   AGE
contour-7649c6f6cc-ct5rz   1/1     Running     0          116s
contour-7649c6f6cc-dmbrc   1/1     Running     0          116s
contour-certgen-rmz86      0/1     Completed   0          116s
envoy-jrhsp                2/2     Running     0          116s
```

[Test with Ingress](https://projectcontour.io/docs/v1.9.0/deploy-options/#test-with-ingress):
```
kubectl apply -f https://projectcontour.io/examples/kuard.yaml
```

[Test with HTTPProxy](https://projectcontour.io/docs/v1.9.0/deploy-options/#test-with-httpproxy):
```
kubectl apply -f https://projectcontour.io/examples/kuard-httpproxy.yaml
```

## Contributing

Thanks for taking the time to join our community and start contributing!

- Please familiarize yourself with the
[Code of Conduct](https://github.com/projectcontour/contour/blob/main/CODE_OF_CONDUCT.md) before contributing.
- See the [contributing guide](docs/CONTRIBUTING.md) for information about setting up your environment, the expected
workflow and instructions on the developer certificate of origin that is required.
- Check out the [open issues](https://github.com/projectcontour/contour-operator/issues).
- Join the Contour Slack channel: [#contour](https://kubernetes.slack.com/messages/contour/)
- Join the **Contour Community Meetings** - [details can be found here](https://projectcontour.io/community)