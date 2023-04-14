#!/bin/ash

echo '>> echo ${MH_PASS} | skopeo login --tls-verify=false ${MH_URL} --password-stdin -u ${MH_USER}'
echo ${MH_PASS} | skopeo login --tls-verify=false ${MH_URL} --password-stdin -u ${MH_USER}
echo '>> ./misc/bin/pull-push.sh alpine:3.16.2 alpine:lugano'
./misc/bin/pull-push.sh alpine:3.16.2 alpine:lugano
read
echo '>> echo ${MH_PASS} | skopeo login --tls-verify=false ${MH_URL} --password-stdin -u ${MH_USER}/cpu#zen3'
echo ${MH_PASS} | skopeo login --tls-verify=false ${MH_URL} --password-stdin -u ${MH_USER}/cpu#zen3
echo '>> ./misc/bin/pull-push.sh alpine:3.17.1 alpine:lugano'
./misc/bin/pull-push.sh alpine:3.17.1 alpine:lugano
