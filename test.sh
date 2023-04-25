#!/bin/bash
set -ex

go test -timeout 60s ./... -coverprofile coverage.txt
gocover-cobertura < coverage.txt > coverage.xml
gocoverstats -v -f coverage.txt --percent
