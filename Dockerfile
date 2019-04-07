# Build the manager binary
FROM golang:1.12 as builder

# Copy in the go src
WORKDIR /go/src/github.com/kkohtaka/namingway
COPY cmd/    cmd/
COPY vendor/ vendor/
COPY pkg/    pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/kkohtaka/namingway/cmd/manager

# Copy the controller-manager into a thin image
FROM scratch
WORKDIR /
COPY --from=builder /go/src/github.com/kkohtaka/namingway/manager .
ENTRYPOINT ["/manager"]
