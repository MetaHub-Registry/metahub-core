#!/bin/bash


if [[ "$1" == "local" ]];then
  SONAR_QUBE_URL=${SONAR_QUBE_URL_LOCAL}
  SONAR_QUBE_TOKEN=${SONAR_QUBE_TOKEN_LOCAL}
fi

set -x 

sonar-scanner \
  -Dsonar.branch.name=$(git rev-parse --abbrev-ref HEAD) \
  -Dsonar.projectKey=${SONAR_QUBE_PROJECT} \
  -Dsonar.sources=. \
  -Dsonar.go.exclusions="**/*_test.go,**/test_*.go,**/vendor/**,**/test.go,**/testcases.go,**/*swagger*.go,**/coverage.*,**/examples*" \
  -Dsonar.go.coverage.reportPaths=coverage.txt \
  -Dsonar.go.tests.reportPaths=report.json \
  -Dsonar.host.url=${SONAR_QUBE_URL} \
  -Dsonar.token=${SONAR_QUBE_TOKEN}

