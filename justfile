bin := "./bin"
cmd := "./cmd"
export GOEXPERIMENT := "jsonv2"

build:
    go build -o {{ bin }}/server {{ cmd }}/server

run:
    go run {{ cmd }}/server

lint:
    golangci-lint run ./...

fmt:
    golangci-lint run --fix ./...

test *args:
    go test {{ args }}

unit *args:
    go test {{ args }} ./...

