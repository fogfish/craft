#!/bin/sh
set -eu

##
## Required ENV
##   CRAFT_SOURCE      (e.g. s3://craft/github.com/fogfish/app)   
##   CRAFT_TARGET      (e.g. github.com/fogfish/app)
##   CRAFT_CDK_CONTEXT (e.g. demo.cdk.context.json)
##

mkdir -p /go/src/$CRAFT_TARGET

cd /go/src/$CRAFT_TARGET

aws s3 cp $CRAFT_SOURCE . --recursive
cp $CRAFT_CDK_CONTEXT cdk.context.json

cdk deploy
