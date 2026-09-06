# --- Build stage ---
FROM golang:1.27-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /out/server ./cmd/server

# --- Runtime stage ---
FROM debian:bookworm-slim

RUN groupadd --gid 10001 app \
    && useradd --uid 10001 --gid app --no-create-home --shell /usr/sbin/nologin app \
    && install -d --owner=app --group=app --mode=0750 /var/lib/digikeeper

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*


COPY --from=build /out/server /usr/local/bin/server
COPY --chmod=755 docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

# Matches the default JOURNAL_STORAGE_PATH configured by the application.
VOLUME ["/var/lib/digikeeper"]

USER app:app

EXPOSE 9000

ENTRYPOINT ["docker-entrypoint.sh"]
