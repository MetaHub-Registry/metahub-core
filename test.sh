#!/bin/bash
set -x

export CGO_ENABLED=1 
export CGO_CFLAGS="-Wno-gnu-folding-constant"

go test -timeout 600s ./... -coverprofile coverage.txt $@
TEST_EXIT=$?
gocover-cobertura < coverage.txt > coverage.xml
gocoverstats -v -f coverage.txt --percent
exit $TEST_EXIT
