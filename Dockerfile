FROM golang:1.23 AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    libvips-dev && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

ARG TARGETARCH
ENV GOOS=linux
ENV GOARCH=${TARGETARCH}

WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN go build -ldflags "-s -w" -o discuit .

FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    redis-server \
    libvips-dev \
    ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/discuit /app/discuit
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /app

EXPOSE 80
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/app/discuit", "serve"]
