FROM golang:1.22-alpine AS builder

RUN apk add --no-cache curl bash
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /api ./main.go

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates curl bash tar && \
    ARCH=$(uname -m) && [ "$ARCH" = "arm64" ] && ARCH="aarch64"; \
    curl -fsSL "https://download.docker.com/linux/static/stable/${ARCH}/docker-27.5.1.tgz" | tar -xz --strip-components=1 -C /usr/local/bin docker/docker && \
    rm -rf /var/lib/apt/lists/*

# Install railpack
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "aarch64" ]; then RAILPACK_ARCH="arm64"; else RAILPACK_ARCH="x86_64"; fi && \
    VERSION=$(curl -sL https://api.github.com/repos/railwayapp/railpack/releases/latest | grep '"tag_name":' | cut -d'"' -f4) && \
    curl -fsSL "https://github.com/railwayapp/railpack/releases/download/${VERSION}/railpack-${VERSION}-${RAILPACK_ARCH}-unknown-linux-musl.tar.gz" \
    -o railpack.tar.gz && \
    tar -xzf railpack.tar.gz -C /usr/local/bin && \
    rm railpack.tar.gz && \
    railpack --version

COPY --from=builder /api /api
EXPOSE 8080
ENTRYPOINT ["/api"]