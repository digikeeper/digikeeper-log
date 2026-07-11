# --- Build stage ---
FROM golang:1.26-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOEXPERIMENT=jsonv2 go build -o /out/server ./cmd/server

# --- Runtime stage ---
FROM debian:bookworm-slim

RUN groupadd --gid 10001 app \
    && useradd --uid 10001 --gid app --no-create-home --shell /usr/sbin/nologin app

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*


COPY --from=build /out/server /usr/local/bin/server
COPY --chmod=755 docker-entrypoint.sh  /usr/local/bin/docker-entrypoint.sh

USER app:app

EXPOSE 9000

ENTRYPOINT ["docker-entrypoint.sh"]
