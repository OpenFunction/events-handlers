FROM golang:1.15 as builder
WORKDIR /workspace
COPY . /workspace
RUN go mod download

# Build
RUN GOPROXY="https://goproxy.cn" CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o handler main.go

# Use distroless as minimal base image to package the handler binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM openfunction/distroless-static:nonroot
WORKDIR /
COPY --from=builder /workspace/handler .
USER nonroot:nonroot

ENTRYPOINT ["/handler"]