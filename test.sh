#!/bin/bash
set -x

export CGO_ENABLED=1 
export CGO_CFLAGS="-Wno-gnu-folding-constant"

# -p 1: run packages SERIALLY. Each package's TestMain spins its own neo4j +
# minio + jaeger testcontainers; the default `go test ./...` runs GOMAXPROCS
# packages in parallel, so the CI agent ends up with many heavy neo4j/minio
# containers at once and starves — whichever package's neo4j gets starved
# returns wrong results / times out, so a different package flakes each run.
# Serializing keeps one container set alive at a time (reliable; a bit slower).
go test -p 1 -timeout 600s ./... -coverprofile coverage.txt $@
TEST_EXIT=$?
gocover-cobertura < coverage.txt > coverage.xml
gocoverstats -v -f coverage.txt --percent
exit $TEST_EXIT
