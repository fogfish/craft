//
// Copyright (C) 2024 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/craft
//

package awscraft_test

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
	"github.com/fogfish/craft/internal/awscraft"
	"github.com/fogfish/tagver"
)

func TestAwsCraft(t *testing.T) {
	app := awscdk.NewApp(nil)

	stack := awscraft.New(app,
		&awscraft.CraftProps{
			StackProps: &awscdk.StackProps{
				Env: &awscdk.Environment{
					Region: jsii.String("us-east-1"),
				},
			},
			Version:          tagver.Version("test"),
			SourceCodeBucket: "test",
		},
	)

	require := map[*string]*float64{
		jsii.String("AWS::EC2::VPC"):                         jsii.Number(1),
		jsii.String("AWS::EC2::Subnet"):                      jsii.Number(2),
		jsii.String("AWS::EC2::RouteTable"):                  jsii.Number(2),
		jsii.String("AWS::EC2::SubnetRouteTableAssociation"): jsii.Number(2),
		jsii.String("AWS::EC2::Route"):                       jsii.Number(2),
		jsii.String("AWS::EC2::InternetGateway"):             jsii.Number(1),
		jsii.String("AWS::EC2::InternetGateway"):             jsii.Number(1),
		jsii.String("AWS::EC2::SecurityGroup"):               jsii.Number(1),
		jsii.String("AWS::Batch::ComputeEnvironment"):        jsii.Number(1),
		jsii.String("AWS::Batch::JobQueue"):                  jsii.Number(1),
		jsii.String("AWS::Batch::JobDefinition"):             jsii.Number(1),
		jsii.String("AWS::S3::Bucket"):                       jsii.Number(1),
		jsii.String("Custom::S3BucketNotifications"):         jsii.Number(1),
		jsii.String("AWS::IAM::Role"):                        jsii.Number(5),
		jsii.String("AWS::Lambda::Function"):                 jsii.Number(3),
		jsii.String("Custom::LogRetention"):                  jsii.Number(1),
	}

	template := assertions.Template_FromStack(stack.Stack, nil)
	for key, val := range require {
		template.ResourceCountIs(key, val)
	}
}
