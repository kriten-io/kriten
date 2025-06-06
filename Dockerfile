FROM golang:1.23 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY . .

RUN GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD) 
# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o kriten -ldflags "-X main.GitBranch=$GIT_BRANCH"

# Use distroless as minimal base image to package the kriten binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/kriten .
COPY --from=builder /workspace/.env .
COPY --from=builder /workspace/spec.json .
USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/kriten"]
