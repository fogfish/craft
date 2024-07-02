#!/bin/sh
set -eu

##
## Required ENV
##   CRAFT_BUCKET
##     S3 bucket where application templates stores
##     (e.g. craft)
##
##   CRAFT_MODULE
##     source code module to build
##     (e.g. github.com/fogfish/app)
##
##   CRAFT_CDK_CONTEXT
##     context for AWS CDK application, inline JSON object
##     (e.g. {"acc": "xxx"})
##

mkdir -p /go/src/$CRAFT_MODULE

cd /go/src/$CRAFT_MODULE

aws s3 cp s3://$CRAFT_BUCKET/$CRAFT_MODULE . --recursive

echo $CRAFT_CDK_CONTEXT > cdk.context.json

cdk deploy
