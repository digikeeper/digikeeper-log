# --- Build stage ---
FROM golang:1.26-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOEXPERIMENT=jsonv2 go build -o /out/server ./cmd/server

# --- Runtime stage ---
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system app && useradd --system --gid app app

COPY --from=build /out/server /usr/local/bin/server
COPY docker-entrypoint.sh /usr/local/bin/

USER app

EXPOSE 9000

ENTRYPOINT ["docker-entrypoint.sh"]
