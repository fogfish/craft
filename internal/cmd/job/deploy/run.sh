#!/bin/sh
set -eu

##
## Required ENV
##   CRAFT_SOURCE 
##     source code folder to download from s3
##     (e.g. s3://craft/github.com/fogfish/app)
##
##   CRAFT_TARGET
##     source code folder to store code
##     (e.g. github.com/fogfish/app)
##
##   CRAFT_MODULE
##     source code module to build
##     (e.g. app)
##
##   CRAFT_CDK_CONTEXT
##     context for AWS CDK application
##     (e.g. demo.cdk.context.json)
##

mkdir -p /go/src/$CRAFT_TARGET

cd /go/src/$CRAFT_TARGET

aws s3 cp $CRAFT_SOURCE . --recursive

cp $CRAFT_CDK_CONTEXT $CRAFT_MODULE/cdk.context.json

cd $CRAFT_MODULE

cdk deploy
