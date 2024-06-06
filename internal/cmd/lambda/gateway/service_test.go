//
// Copyright (C) 2024 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/craft
//

package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	"github.com/aws/aws-sdk-go-v2/service/batch/types"
	"github.com/fogfish/it/v2"
	"github.com/fogfish/swarm"
)

type mock struct {
	expectVal *batch.SubmitJobInput
	returnVal *batch.SubmitJobOutput
}

func (m *mock) SubmitJob(ctx context.Context, params *batch.SubmitJobInput, optFns ...func(*batch.Options)) (*batch.SubmitJobOutput, error) {
	if aws.ToString(params.JobQueue) != aws.ToString(m.expectVal.JobQueue) {
		return nil, fmt.Errorf("unexpected job queue")
	}

	if aws.ToString(params.JobDefinition) != aws.ToString(m.expectVal.JobDefinition) {
		return nil, fmt.Errorf("unexpected job definition")
	}

	env := map[string]string{}
	for _, e := range params.ContainerOverrides.Environment {
		env[aws.ToString(e.Name)] = aws.ToString(e.Value)
	}
	for _, e := range m.expectVal.ContainerOverrides.Environment {
		if x, has := env[aws.ToString(e.Name)]; !has || x != aws.ToString(e.Value) {
			return nil, fmt.Errorf("unexpected environment override")
		}
	}

	return m.returnVal, nil
}

func TestService(t *testing.T) {
	q := &mock{
		returnVal: &batch.SubmitJobOutput{},
		expectVal: &batch.SubmitJobInput{
			JobDefinition: aws.String("test-job"),
			JobQueue:      aws.String("test-queue"),
			ContainerOverrides: &types.ContainerOverrides{
				Environment: []types.KeyValuePair{
					{Name: aws.String("CRAFT_SOURCE"), Value: aws.String("s3://craft/github.com/fogfish/craft")},
					{Name: aws.String("CRAFT_TARGET"), Value: aws.String("github.com/fogfish/craft")},
					{Name: aws.String("CRAFT_CDK_CONTEXT"), Value: aws.String("test.cdk.context.json")},
				},
			},
		},
	}

	service := New(q, "test-queue", "test-job")

	rcv := make(chan swarm.Msg[*events.S3EventRecord])
	ack := make(chan swarm.Msg[*events.S3EventRecord])
	go service.Run(rcv, ack)

	t.Run("SubmitJob", func(t *testing.T) {
		rcv <- swarm.Msg[*events.S3EventRecord]{
			Ctx: swarm.NewContext(context.TODO(), "test", "na"),
			Object: &events.S3EventRecord{
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "craft"},
					Object: events.S3Object{Key: "github.com/fogfish/craft/test.cdk.context.json"},
				},
			},
		}
		msg := <-ack
		it.Then(t).Should(it.Nil(msg.Ctx.Error))
	})

	t.Run("SubmitJobFailed", func(t *testing.T) {
		rcv <- swarm.Msg[*events.S3EventRecord]{
			Ctx: swarm.NewContext(context.TODO(), "test", "na"),
			Object: &events.S3EventRecord{
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "craft"},
					Object: events.S3Object{Key: "github.com/fogfish/unexpected/test.cdk.context.json"},
				},
			},
		}
		msg := <-ack
		it.Then(t).ShouldNot(it.Nil(msg.Ctx.Error))
	})

	t.Run("EmptyEvent", func(t *testing.T) {
		rcv <- swarm.Msg[*events.S3EventRecord]{
			Ctx: swarm.NewContext(context.TODO(), "test", "na"),
		}
		msg := <-ack
		it.Then(t).Should(it.Nil(msg.Ctx.Error))
	})

	t.Run("InvalidKey", func(t *testing.T) {
		rcv <- swarm.Msg[*events.S3EventRecord]{
			Ctx: swarm.NewContext(context.TODO(), "test", "na"),
			Object: &events.S3EventRecord{
				S3: events.S3Entity{
					Object: events.S3Object{Key: "some-key"},
				},
			},
		}
		msg := <-ack
		it.Then(t).Should(it.Nil(msg.Ctx.Error))
	})
}
