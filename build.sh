#!/usr/bin/env bash

rm -f deploy.zip || true
GOOS=linux GOARCH=amd64 go build -v
zip deploy.zip aws-notifier