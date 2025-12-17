#!/bin/bash
set -ex

go test -timeout 240s ./... -coverprofile coverage.txt $@
gocover-cobertura < coverage.txt > coverage.xml
gocoverstats -v -f coverage.txt --percent
