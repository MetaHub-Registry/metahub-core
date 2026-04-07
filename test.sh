#!/bin/bash
set -ex

export CGO_ENABLED=1 

go test -timeout 300s ./... -coverprofile coverage.txt $@
gocover-cobertura < coverage.txt > coverage.xml
gocoverstats -v -f coverage.txt --percent
