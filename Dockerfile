# Build the manager binary
FROM golang:1.15 as builder

WORKDIR /
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/contour-operator.go contour-operator.go
COPY api/ api/
COPY controller/ controller/
COPY util/ util/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o contour-operator contour-operator.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /contour-operator .
USER nonroot:nonroot

ENTRYPOINT ["/contour-operator"]
