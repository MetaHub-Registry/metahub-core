#!/bin/sh

if [ -z "$MH_URL" ]; then
    echo "MH_URL is not set"
    exit 1
fi
if [ -z "$MH_USER" ]; then
    echo "MH_USER is not set"
    exit 1
fi

if [ -z "$1" ]; then
    echo "Usage: $0 <src_image> [<dst_image>]"
    echo "  If <dst_image> is not specified, it will be set to <src_image>"
    echo "  the image will be pushed to MH_URL/MH_USER/<dst_image>"
    exit 1
fi

if [ -z "$2" ]; then
    echo "  <dst_image> is not specified, it will be set to $1"
    export DST=$1
else
    export DST=$2
fi
set -x
skopeo copy --dest-tls-verify=false docker://docker.io/${1} docker://$MH_URL/${MH_USER}/${DST}
