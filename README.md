# contour-operator
Welcome to the Contour Operator project. Contour Operator deploys and manages Contour using an
[operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/). 

## Get Started

### Prerequisites

* A [Kubernetes](https://kubernetes.io/) cluster
* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) installed

Install the Contour Operator & Contour CRDs:
```
$ kubectl apply -f https://raw.githubusercontent.com/projectcontour/contour-operator/main/examples/operator/operator.yaml
```

Verify the deployment is available:
```
$ kubectl get deploy -n contour-operator
NAME               READY   UP-TO-DATE   AVAILABLE   AGE
contour-operator   1/1     1            1           1m
```

Install an instance of the `Contour` custom resource:
```
$ kubectl apply -f https://raw.githubusercontent.com/projectcontour/contour-operator/main/examples/contour/contour.yaml
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

[Test with Ingress](https://projectcontour.io/docs/main/deploy-options/#test-with-ingress):
```
$ kubectl apply -f https://projectcontour.io/examples/kuard.yaml
```

Verify the example app deployment is available:
```
$ kubectl get deploy/kuard
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
kuard   3/3     3            3           1m50s
```

Test the example app:
```
$ curl -o /dev/null -s -w "%{http_code}\n" http://local.projectcontour.io/
200
```

**Note:** A public DNS record exists for "local.projectcontour.io" which is
configured to resolve to `127.0.0.1`. This allows you to use a real domain name
when testing in a [kind](https://kind.sigs.k8s.io/) cluster. If testing on a
standard Kubernetes cluster, replace "local.projectcontour.io" with the
hostname of `kubectl get deploy/kuard`.

## Contributing

Thanks for taking the time to join our community and start contributing!

- Please familiarize yourself with the
[Code of Conduct](https://github.com/projectcontour/contour/blob/main/CODE_OF_CONDUCT.md) before contributing.
- See the [contributing guide](docs/CONTRIBUTING.md) for information about setting up your environment, the expected
workflow and instructions on the developer certificate of origin that is required.
- Check out the [open issues](https://github.com/projectcontour/contour-operator/issues).
- Join the Contour Slack channel: [#contour](https://kubernetes.slack.com/messages/contour/)
- Join the **Contour Community Meetings** - [details can be found here](https://projectcontour.io/community)