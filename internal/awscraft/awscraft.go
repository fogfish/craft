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
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/jsii-runtime-go"
	"github.com/fogfish/scud"
	"github.com/fogfish/swarm/broker/eventbridge"
	"github.com/fogfish/tagver"
)

type CraftProps struct {
	*awscdk.StackProps
	Version tagver.Version

	// AWS S3 Bucket Identity for keeping source code
	SourceCodeBucket string

	// Max number of CPUs allocated for the cluster
	MaxvCpus *float64

	// The number of vCPUs reserved for the container.
	//
	// Default: 1.0
	Cpu *float64

	// The memory reserved for the container in GBs.
	// The memory have to be aligned with reserved CPUs
	// (e.g. 4 vCPU requires 8GB, 8 vCPU requires 16GB)
	//
	// Default: 4 GB
	Memory *float64

	// Enable spot instances
	Spot *bool
}

type Craft struct {
	awscdk.Stack
	vpc        awsec2.Vpc
	compute    awsbatch.FargateComputeEnvironment
	queue      awsbatch.IJobQueue
	role       awsiam.Role
	jobDeploy  awsbatch.EcsJobDefinition
	sourceCode awss3.IBucket
	broker     *eventbridge.Broker
}

func New(app awscdk.App, props *CraftProps) *Craft {
	stack := awscdk.NewStack(app,
		jsii.String(props.Version.Tag("craft")),
		props.StackProps,
	)

	if props.Spot == nil {
		props.Spot = jsii.Bool(true)
	}

	if props.Cpu == nil {
		props.Cpu = jsii.Number(1.0)
	}

	if props.Memory == nil {
		props.Memory = jsii.Number(4.0)
	}

	c := &Craft{Stack: stack}
	c.createSourceCode(props)

	c.createNetworking(props)
	c.createCompute(props)
	c.createQueue(props)
	c.createRole(props)
	c.createJobDeploy(props)
	c.createGateway(props)

	return c
}

func (c *Craft) createSourceCode(props *CraftProps) {
	c.sourceCode = awss3.NewBucket(c.Stack, jsii.String("Bucket"),
		&awss3.BucketProps{
			BucketName: jsii.String(props.SourceCodeBucket),
		},
	)
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
				"craft": awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
					Statements: &[]awsiam.PolicyStatement{
						awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
							Actions:   jsii.Strings("sts:AssumeRole"),
							Resources: jsii.Strings("arn:aws:iam::*:role/craft-*"),
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
			Cpu:                    props.Cpu,
			Memory:                 awscdk.Size_Gibibytes(props.Memory),
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
	c.broker = eventbridge.NewBroker(c.Stack, jsii.String("Broker"), nil)
	bus := c.broker.NewEventBus(nil)

	f := c.broker.NewSink(
		&eventbridge.SinkProps{
			Source:     []string{*bus.EventBusName()},
			Categories: []string{"EventCraft"},
			Function: &scud.FunctionGoProps{
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

	c.jobDeploy.GrantSubmitJob(f.Handler, c.queue)
	// c.sourceCode.GrantRead(f.Handler, nil)
}
