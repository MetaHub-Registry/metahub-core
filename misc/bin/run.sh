#!/bin/ash

echo '>> echo ${MH_PASS} | docker login  ${MH_URL} --password-stdin -u ${MH_USER}'
echo ${MH_PASS} | docker login  ${MH_URL} --password-stdin -u ${MH_USER}
echo '>> docker run -ti --rm --pull=always ${MH_URL}/qnib/alpine:lugano cat /etc/os-release|grep VERSION_ID'
docker run -ti --rm --pull=always ${MH_URL}/qnib/alpine:lugano cat /etc/os-release|grep VERSION_ID
read
echo '>> echo ${MH_PASS} | docker login  ${MH_URL} --password-stdin -u ${MH_USER}/cpu#zen3'
echo ${MH_PASS} | docker login  ${MH_URL} --password-stdin -u ${MH_USER}/cpu#zen3
echo '>> docker run -ti --rm --pull=always ${MH_URL}/qnib/alpine:lugano cat /etc/os-release|grep VERSION_ID'
docker run -ti --rm --pull=always ${MH_URL}/qnib/alpine:lugano cat /etc/os-release|grep VERSION_ID
