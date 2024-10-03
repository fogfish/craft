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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	"github.com/aws/aws-sdk-go-v2/service/batch/types"
	"github.com/fogfish/craft/internal/events"
	"github.com/fogfish/craft/internal/scheduler"
	"github.com/fogfish/it/v2"
	"github.com/fogfish/swarm"
)

var (
	eventCraft = events.EventCraft{
		UID:     "123-456-789",
		Module:  "github.com/fogfish/craft",
		Context: []byte(`{"acc": "test"}`),
	}

	eventUnexpected = events.EventCraft{
		UID:     "123-456-789",
		Module:  "github.com/fogfish/unexpected",
		Context: []byte(`{"acc": "test"}`),
	}

	eventUndefined = events.EventCraft{}

	eventWrongType = events.EventCraft{
		Context: []byte(`{"acc": "test"}`),
	}
)

func TestSubmitJob(t *testing.T) {
	service := mockService()

	rcv := make(chan swarm.Msg[events.EventCraft])
	ack := make(chan swarm.Msg[events.EventCraft])
	go service.Run(rcv, ack)

	rcv <- swarm.Msg[events.EventCraft]{
		Category: "test",
		Object:   eventCraft,
	}
	msg := <-ack
	it.Then(t).Should(it.Nil(msg.Error))
}

func TestSubmitJobFailed(t *testing.T) {
	service := mockService()

	rcv := make(chan swarm.Msg[events.EventCraft])
	ack := make(chan swarm.Msg[events.EventCraft])
	go service.Run(rcv, ack)

	rcv <- swarm.Msg[events.EventCraft]{
		Category: "test",
		Object:   eventUnexpected,
	}
	msg := <-ack
	it.Then(t).ShouldNot(it.Nil(msg.Error))
}

func TestCorruptedEvents(t *testing.T) {
	for name, evt := range map[string]events.EventCraft{
		"Undefined": eventUndefined,
		"WrongType": eventWrongType,
	} {
		t.Run(name, func(t *testing.T) {
			service := mockService()

			rcv := make(chan swarm.Msg[events.EventCraft])
			ack := make(chan swarm.Msg[events.EventCraft])
			go service.Run(rcv, ack)

			rcv <- swarm.Msg[events.EventCraft]{
				Category: "test",
				Object:   evt,
			}
			msg := <-ack
			it.Then(t).ShouldNot(it.Nil(msg.Error))
		})
	}
}

//------------------------------------------------------------------------------

func mockService() *Service {
	batch := &mock{
		returnVal: &batch.SubmitJobOutput{},
		expectVal: &batch.SubmitJobInput{
			JobDefinition: aws.String("test-job"),
			JobQueue:      aws.String("test-queue"),
			ContainerOverrides: &types.ContainerOverrides{
				Environment: []types.KeyValuePair{
					{Name: aws.String("CRAFT_BUCKET"), Value: aws.String("test-s3")},
					{Name: aws.String("CRAFT_MODULE"), Value: aws.String("github.com/fogfish/craft")},
					{Name: aws.String("CRAFT_CDK_CONTEXT"), Value: aws.String(string(eventCraft.Context))},
				},
			},
		},
	}

	scheduler := scheduler.New(batch, "test-queue", "test-job", "test-s3")

	return New(scheduler)
}

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
		key := aws.ToString(e.Name)
		if x, has := env[key]; !has || x != aws.ToString(e.Value) {
			return nil, fmt.Errorf("unexpected environment override: %s, %s", key, x)
		}
	}

	return m.returnVal, nil
}
