# examples

This directory contains example manifests for running Contour using Contour
Operator. The following sections describe the purpose of each subdirectory.

## `contour`

An example instance of the `Contour` custom resource. **Note:** You must first
run Contour Operator using the manifest from the `operator` directory.

## `operator`

A single manifest rendered from individual `config` manifests suitable for
`kubectl apply`ing via a URL.
