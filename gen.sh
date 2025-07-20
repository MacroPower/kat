#!/bin/bash

go mod tidy
go generate ./...

go build -o kat ./cmd/kat/main.go

mkdir -p ./completion/bash
mkdir -p ./completion/fish
mkdir -p ./completion/zsh
mkdir -p ./man

./kat completion bash > ./completion/bash/kat.bash
./kat completion fish > ./completion/fish/kat.fish
./kat completion zsh > ./completion/zsh/_kat
./kat man > ./man/kat.1
