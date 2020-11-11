# Contributing

Thanks for taking the time to join our community and start contributing.
These guidelines will help you get started with the Contour Operator project.
Please note that we require [DCO sign off](#dco-sign-off).  

For more insight into the Contour Operator development workflow, reference the
[how-we-work](https://projectcontour.io/resources/how-we-work/) page.

## Building from source

This section describes how to build Contour Operator from source.

### Prerequisites

1. *Install Go*

    Contour Operator requires [Go 1.15][1] or later. We also assume that you're familiar with
    Go's [`GOPATH` workspace][3] convention and have the appropriate environment variables set.

### Fetch the source

Contour Operator uses [`go modules`][2] for dependency management.

```
go get github.com/projectcontour/contour-operator
```

The remainder of this document assumes your terminal's working directory is
`$GOPATH/src/github.com/projectcontour/contour-operator`.

### Building

To build Contour Operator, run:

```
make manager
```

This produces a `contour-operator` binary in your `$GOPATH/bin` directory and runs go fmt and go vet against the code.

### Running the unit tests

You can run all the unit tests for the project:

```
make test
```

To run the tests for a single package, change to package directory and run:

```
go test .
```

__Note:__ Unit tests must pass for your PR to get merged.

## Contribution workflow

This section describes the process for contributing a bug fix or new feature.

### Before you submit a pull request

This project operates according to the _talk, then code_ rule. If you plan to
submit a pull request for anything more than a typo or obvious bug fix,
first you _should_ [raise an issue][6] to discuss your proposal, before submitting any code.

Depending on the size of the feature you may be expected to first write a design proposal.
Follow the [Proposal Process](https://github.com/projectcontour/community/blob/main/GOVERNANCE.md#proposal-process)
documented in Contour's Governance.

### Commit message and PR guidelines

- Have a short subject on the first line and a body. The body can be empty.
- Use the imperative mood (ie "If applied, this commit will (subject)" should make sense).
- There must be a DCO line ("Signed-off-by: John Doe <jdoe@example.com>"), see [DCO Sign Off](#dco-sign-off) below.
- Put a summary of the main area affected by the commit at the start, with a colon as delimiter.
For example 'docs:', 'internal/(packagename):', 'design:' or something similar.
- Do not merge commits that don't relate to the affected issue (e.g. "Updating from PR comments", etc).
Should the need to cherry-pick a commit or rollback arise, the purpose of the commit should be clear.
- If main has moved on, you'll need to rebase before we can merge, so merging upstream main or rebasing
from upstream before opening your PR will probably save you some time.

Pull requests *must* include a `Fixes #NNNN` or `Updates #NNNN` comment. Remember that `Fixes` will close
the associated issue, and `Updates` will link the PR to it.

#### Commit message template

```
<packagename>: <imperative mood short description>

<longer change description/justification>

Updates #NNNN
Fixes #MMMM

Signed-off-by: Your Name <you@youremail.com>
```

### Merging commits

Maintainers should prefer to merge pull requests with the
[Squash and merge](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/about-pull-request-merges#squash-and-merge-your-pull-request-commits) option.
This option is preferred for a number of reasons. First, it causes GitHub to insert the pull request number in the
commit subject which makes it easier to track which PR changes landed in. Second, it gives maintainers an opportunity to
edit the commit message to conform to the project's standards and general [good practice](https://chris.beams.io/posts/git-commit/).
Finally, a one-to-one correspondence between pull requests and commits makes it easier to manage reverting changes and
increases the reliability of bisecting the tree (since CI runs at a pull request granularity).

At a maintainer's discretion, pull requests with multiple commits can be merged with the
[Create a merge commit](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/about-pull-request-merges) option.
Merging pull requests with multiple commits can make sense in cases where a change involves code generation or
mechanical changes that can be cleanly separated from semantic changes. The maintainer should review commit messages for
each commit and make sure that each commit builds and passes tests.

### Import Aliases

Naming is one of the most difficult things in software engineering. Contour Operator uses the following pattern to
name imports when referencing packages from other packages.

> thingversion: The name+package path of the thing and then the version

Example:

```
appsv1 "k8s.io/api/apps/v1"
```   
 
### Pre commit CI

Before submitting a change it should pass all the pre commit CI jobs. If there are unrelated test failures
the change can be merged so long as a reference to an issue that tracks the test failures is provided.

Once a change lands in main it will be built and available at `docker.io/projectcontour/contour-operator:main`.
The Contour Operator image follows Contour's [tagging][7] policy.

### Build an image

To build a Contour Operator container image that includes your changes, run these commands
(replacing <MY_DOCKER_USERNAME> with your own Docker Hub username):

```
IMAGE=docker.io/<MY_DOCKER_USERNAME>/contour-operator make container
```

The image can now be pushed to your repository:
```
IMAGE=docker.io/<MY_DOCKER_USERNAME>/contour-operator make push
```

### Verify your changes

#### Prerequisites

1. *[Deploy](https://projectcontour.io/docs/v1.9.0/deploy-options/#kind) a [kind](https://kind.sigs.k8s.io/) cluster.*

Verify your changes by deploying the image you built to your kind cluster. First install the Contour and
Contour Operator CRDs.

```
make install
```

Then deploy the operator to your kind cluster:

```
IMAGE=docker.io/<MY_DOCKER_USERNAME>/contour-operator make deploy
```

### Run the Operator Locally

The easiest way to test your changes is to run the operator locally. __Note:__ This will run in the foreground,
so switch to a new terminal if you want to leave it running:

```
make run
```

Before submitting your changes, follow the image-based deployment instructions to ensure the operator works as
expected within a Kubernetes cluster.

## DCO Sign off

All authors to the project retain copyright to their work. However, to ensure that they are only submitting work that
they have rights to, we are requiring everyone to acknowledge this by signing their work.

Since this signature indicates your rights to the contribution and certifies the statements below, it must contain
your real name and email address. Various forms of noreply email address must not be used.

Any copyright notices in this repository should specify the authors as "The project authors".

To sign your work, just add a line like this at the end of your commit message:

```
Signed-off-by: John Doe <jdoe@example.com>
```

This can easily be done with the `--signoff` option to `git commit`.

By doing so you can certify the following (from [https://developercertificate.org/][5]):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
1 Letterman Drive
Suite D4700
San Francisco, CA, 94129

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

[1]: https://golang.org/dl/
[2]: https://github.com/golang/go/wiki/Modules
[3]: https://golang.org/doc/code.html
[4]: https://golang.org/pkg/testing/
[5]: https://developercertificate.org/
[6]: https://github.com/projectcontour/contour-operator/issues/new/choose
[6]: https://projectcontour.io/resources/tagging/
[7]: https://projectcontour.io/docs/main/deploy-options/