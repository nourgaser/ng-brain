#!/bin/bash

docker run --rm \
  -v $(pwd):/app \
  -w /app \
  -e CGO_ENABLED=0 \
  golang:alpine \
  sh -c "apk add git && go get gopkg.in/yaml.v3 && go get github.com/fsnotify/fsnotify && go build -o librarian librarian.go"