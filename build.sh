#!/bin/bash

mkdir -p metalib 
rsync -aP \
    --exclude='.git' \
    --exclude='*.tar' \
    --exclude='.scannerwork' \
    --exclude='misc' \
    --exclude='metalib' \
    --exclude='.bumpversion.cfg' \
    --exclude='coverage*' \
    --exclude='sonar-project.properties' \
    . metalib/.
tar cf metalib.tar metalib
rm -rf metalib