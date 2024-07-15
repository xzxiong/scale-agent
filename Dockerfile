# Build the manager binary
FROM golang:1.22.3-bookworm as builder
ARG TARGETOS
ARG TARGETARCH
ARG GITHUB_ACCESS_TOKEN
ARG GOPROXY="https://goproxy.cn,direct"

RUN go env -w GOPROXY=${GOPROXY} GOPRIVATE="github.com/matrixone-cloud"

WORKDIR /workspace
# Copy the Go Modules / go sources
COPY . .

# Config for pulling private repro
RUN git config --global url."https://${GITHUB_ACCESS_TOKEN}:@github.com/".insteadOf "https://github.com/"
RUN go mod download

# Build
RUN make build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM ubuntu:22.04
WORKDIR /
RUN apt update && apt -y install locales curl
RUN locale-gen en_US.UTF-8
ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8

WORKDIR /
COPY --from=builder /workspace/bin/scale-agent .

EXPOSE 8080

ENTRYPOINT ["/scale-agent"]
