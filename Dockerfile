ARG DEBIAN_BASE_SUFFIX=amd64
FROM k8s.gcr.io/debian-base-${DEBIAN_BASE_SUFFIX}:1.0.0 as builder
ARG GO_TARBALL=go1.12.linux-amd64.tar.gz
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH
RUN apt-get update && apt-get install -y \
        curl \
    && rm -rf /var/lib/apt/lists/*
RUN curl https://dl.google.com/go/${GO_TARBALL} | tar zxv -C /usr/local
WORKDIR /go/src/github.com/kkohtaka/namingway
COPY cmd/    cmd/
COPY vendor/ vendor/
COPY pkg/    pkg/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/kkohtaka/namingway/cmd/manager

# Copy the controller-manager into a thin image
FROM scratch
WORKDIR /
COPY --from=builder /go/src/github.com/kkohtaka/namingway/manager .
ENTRYPOINT ["/manager"]
