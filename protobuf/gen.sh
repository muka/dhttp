#!/usr/bin/env bash

PROTOSRC="./*.proto"

BASEDIR=$(dirname "$0")

protoc_bin=${BASEDIR}/../tmp/protoc/bin/protoc
protoc_include=${BASEDIR}/../tmp/protoc/include
googleapis=${BASEDIR}/../tmp/googleapis

# generate the gRPC code
${protoc_bin} \
    -I. \
    -I${protoc_include} \
    -I${googleapis} \
    --go_out=${BASEDIR} \
    $PROTOSRC
