#!/bin/sh

echo ${MH_PASS} | skopeo login --tls-verify=false ${MH_URL} --password-stdin -u ${MH_USER}
pull-push.sh alpine:3.16.2
echo ${MH_PASS} | skopeo login --tls-verify=false ${MH_URL} --password-stdin -u ${MH_USER}/cpu#zen3
pull-push.sh alpine:3.16.1 alpine:3.16.2