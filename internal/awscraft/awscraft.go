//
// Copyright (C) 2024 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/craft
//

package awscraft

import (
	"os"
	"path/filepath"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsbatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecrassets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/jsii-runtime-go"
	"github.com/fogfish/craft/internal/events"
	"github.com/fogfish/scud"
	"github.com/fogfish/swarm/broker/events3"
	"github.com/fogfish/tagver"
)

type CraftProps struct {
	*awscdk.StackProps
	Version          tagver.Version
	SourceCodeBucket string
	MaxvCpus         *float64
	Spot             *bool
}

type Craft struct {
	awscdk.Stack
	vpc        awsec2.Vpc
	compute    awsbatch.FargateComputeEnvironment
	queue      awsbatch.IJobQueue
	sourceCode awss3.IBucket
	broker     *events3.Broker
	role       awsiam.Role
	jobDeploy  awsbatch.EcsJobDefinition
}

func New(app awscdk.App, props *CraftProps) *Craft {
	stack := awscdk.NewStack(app,
		jsii.String(props.Version.Tag("craft")),
		props.StackProps,
	)

	c := &Craft{Stack: stack}
	c.createNetworking(props)
	c.createCompute(props)
	c.createQueue(props)
	c.createBroker(props)
	c.createRole(props)
	c.createJobDeploy(props)
	c.createGateway(props)

	return c
}

func (c *Craft) createNetworking(props *CraftProps) {
	c.vpc = awsec2.NewVpc(c.Stack, jsii.String("VPC"),
		&awsec2.VpcProps{
			VpcName: awscdk.Aws_STACK_NAME(),
			SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
				{
					Name:       jsii.String("public"),
					SubnetType: awsec2.SubnetType_PUBLIC,
				},
			},
		},
	)
}

func (c *Craft) createCompute(props *CraftProps) {
	c.compute = awsbatch.NewFargateComputeEnvironment(c.Stack, jsii.String("Compute"),
		&awsbatch.FargateComputeEnvironmentProps{
			ComputeEnvironmentName: awscdk.Aws_STACK_NAME(),
			Vpc:                    c.vpc,
			MaxvCpus:               props.MaxvCpus,
			Spot:                   props.Spot,
			VpcSubnets: &awsec2.SubnetSelection{
				SubnetGroupName: jsii.String("public"),
			},
		},
	)
}

func (c *Craft) createQueue(props *CraftProps) {
	c.queue = awsbatch.NewJobQueue(c.Stack, jsii.String("Queue"),
		&awsbatch.JobQueueProps{
			JobQueueName: awscdk.Aws_STACK_NAME(),
			ComputeEnvironments: &[]*awsbatch.OrderedComputeEnvironment{
				{Order: jsii.Number(1.0), ComputeEnvironment: c.compute},
			},
		},
	)
}

func (c *Craft) createBroker(props *CraftProps) {
	c.broker = events3.NewBroker(c.Stack, jsii.String("Broker"), nil)

	c.sourceCode = c.broker.NewBucket(
		&awss3.BucketProps{
			BucketName: jsii.String(props.Version.Tag(props.SourceCodeBucket)),
		},
	)
}

func (c *Craft) createRole(props *CraftProps) {
	c.role = awsiam.NewRole(c.Stack, jsii.String("Role"),
		&awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(jsii.String("ecs-tasks.amazonaws.com"), nil),
			ManagedPolicies: &[]awsiam.IManagedPolicy{
				awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AmazonEC2ContainerServiceforEC2Role")),
			},
			InlinePolicies: &map[string]awsiam.PolicyDocument{
				"awscdk": awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
					Statements: &[]awsiam.PolicyStatement{
						awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
							Actions:   jsii.Strings("sts:AssumeRole"),
							Resources: jsii.Strings("arn:aws:iam::*:role/cdk-*"),
						}),
					},
				}),
			},
		},
	)

	c.sourceCode.GrantRead(c.role, nil)
}

func (c *Craft) createJobDeploy(props *CraftProps) {
	sourceCode := os.Getenv("GITHUB_WORKSPACE")
	if sourceCode == "" {
		sourceCode = filepath.Join(os.Getenv("GOPATH"), "src/github.com/fogfish/craft")
	}
	sourceCode = filepath.Join(sourceCode, "internal/cmd/job/deploy")

	asset := awsecrassets.NewDockerImageAsset(c.Stack, jsii.String("Image"),
		&awsecrassets.DockerImageAssetProps{
			Directory: jsii.String(sourceCode),
			Platform:  awsecrassets.Platform_LINUX_AMD64(),
		},
	)

	container := awsbatch.NewEcsFargateContainerDefinition(c.Stack, jsii.String("Container"),
		&awsbatch.EcsFargateContainerDefinitionProps{
			Cpu:                    jsii.Number(1.0),
			Memory:                 awscdk.Size_Gibibytes(jsii.Number(4.0)),
			Image:                  awsecs.ContainerImage_FromDockerImageAsset(asset),
			AssignPublicIp:         jsii.Bool(true),
			JobRole:                c.role,
			FargateCpuArchitecture: awsecs.CpuArchitecture_X86_64(),
		},
	)

	c.jobDeploy = awsbatch.NewEcsJobDefinition(c.Stack, jsii.String("Builder"),
		&awsbatch.EcsJobDefinitionProps{
			JobDefinitionName: jsii.String(props.Version.Tag("craft-job-deploy")),
			Container:         container,
		},
	)
}

func (c *Craft) createGateway(props *CraftProps) {
	sink := c.broker.NewSink(
		&events3.SinkProps{
			// Note: the default property of EventSource captures OBJECT_CREATED and OBJECT_REMOVED events
			EventSource: &awslambdaeventsources.S3EventSourceProps{
				Events: &[]awss3.EventType{
					awss3.EventType_OBJECT_CREATED,
				},
				Filters: &[]*awss3.NotificationKeyFilter{
					{Suffix: jsii.String(events.EVENT_CRAFT)},
				},
			},
			Lambda: &scud.FunctionGoProps{
				SourceCodeModule: "github.com/fogfish/craft",
				SourceCodeLambda: "internal/cmd/lambda/gateway",
				FunctionProps: &awslambda.FunctionProps{
					FunctionName: awscdk.Aws_STACK_NAME(),
					Timeout:      awscdk.Duration_Seconds(jsii.Number(5.0)),
					Environment: &map[string]*string{
						"CONFIG_VSN":             jsii.String(string(props.Version)),
						"CONFIG_S3":              c.sourceCode.BucketName(),
						"CONFIG_BATCH_QUEUE":     c.queue.JobQueueName(),
						"CONFIG_BATCH_JOB_CRAFT": c.jobDeploy.JobDefinitionArn(),
					},
				},
			},
		},
	)

	c.jobDeploy.GrantSubmitJob(sink.Handler, c.queue)
	c.broker.Bucket.GrantRead(sink.Handler, nil)
}
