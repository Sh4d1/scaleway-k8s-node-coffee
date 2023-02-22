FROM golang:1.20 as builder

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -a -o scaleway-k8s-node-coffee ./cmd/

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/scaleway-k8s-node-coffee .
USER nonroot:nonroot

ENTRYPOINT ["/scaleway-k8s-node-coffee"]
