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
	"io/fs"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	"github.com/aws/aws-sdk-go-v2/service/batch/types"
	"github.com/fogfish/craft/internal/scheduler"
	"github.com/fogfish/it/v2"
	"github.com/fogfish/swarm"
)

const (
	cdkContext = `{"acc": "test"}`

	eventCraft = `{
	  "uid": "123-456-789",
		"module": "github.com/fogfish/craft",
		"context": {"acc": "test"}
	}`

	eventUnexpected = `{
	  "uid": "123-456-789",
		"module": "github.com/fogfish/unexpected",
		"context": {"acc": "test"}
	}`

	eventCorrupted = `{`

	eventUndefined = `{}`

	eventWrongType = `{
		"context": {"acc": "test"}
	}`
)

func TestSubmitJob(t *testing.T) {
	service := mockService(eventCraft, nil)

	rcv := make(chan swarm.Msg[*events.S3EventRecord])
	ack := make(chan swarm.Msg[*events.S3EventRecord])
	go service.Run(rcv, ack)

	rcv <- swarm.Msg[*events.S3EventRecord]{
		Ctx: swarm.NewContext(context.TODO(), "test", "na"),
		Object: &events.S3EventRecord{
			S3: events.S3Entity{
				Bucket: events.S3Bucket{Name: "test-s3"},
				Object: events.S3Object{Key: "test.craft.event.json"},
			},
		},
	}
	msg := <-ack
	it.Then(t).Should(it.Nil(msg.Ctx.Error))
}

func TestSubmitJobFailed(t *testing.T) {
	service := mockService(eventUnexpected, nil)

	rcv := make(chan swarm.Msg[*events.S3EventRecord])
	ack := make(chan swarm.Msg[*events.S3EventRecord])
	go service.Run(rcv, ack)

	rcv <- swarm.Msg[*events.S3EventRecord]{
		Ctx: swarm.NewContext(context.TODO(), "test", "na"),
		Object: &events.S3EventRecord{
			S3: events.S3Entity{
				Bucket: events.S3Bucket{Name: "test-s3"},
				Object: events.S3Object{Key: "test.craft.event.json"},
			},
		},
	}
	msg := <-ack
	it.Then(t).ShouldNot(it.Nil(msg.Ctx.Error))
}

func TestS3AccessFailed(t *testing.T) {
	service := mockService(eventCraft, fmt.Errorf("Access Denied"))

	rcv := make(chan swarm.Msg[*events.S3EventRecord])
	ack := make(chan swarm.Msg[*events.S3EventRecord])
	go service.Run(rcv, ack)

	rcv <- swarm.Msg[*events.S3EventRecord]{
		Ctx: swarm.NewContext(context.TODO(), "test", "na"),
		Object: &events.S3EventRecord{
			S3: events.S3Entity{
				Bucket: events.S3Bucket{Name: "test-s3"},
				Object: events.S3Object{Key: "test.craft.event.json"},
			},
		},
	}
	msg := <-ack
	it.Then(t).ShouldNot(it.Nil(msg.Ctx.Error))
}

func TestCorruptedEvents(t *testing.T) {
	for name, evt := range map[string]string{
		"Corrupted": eventCorrupted,
		"Undefined": eventUndefined,
		"WrongType": eventWrongType,
	} {
		t.Run(name, func(t *testing.T) {
			service := mockService(evt, nil)

			rcv := make(chan swarm.Msg[*events.S3EventRecord])
			ack := make(chan swarm.Msg[*events.S3EventRecord])
			go service.Run(rcv, ack)

			rcv <- swarm.Msg[*events.S3EventRecord]{
				Ctx: swarm.NewContext(context.TODO(), "test", "na"),
				Object: &events.S3EventRecord{
					S3: events.S3Entity{
						Bucket: events.S3Bucket{Name: "test-s3"},
						Object: events.S3Object{Key: "test.craft.event.json"},
					},
				},
			}
			msg := <-ack
			it.Then(t).ShouldNot(it.Nil(msg.Ctx.Error))
		})
	}
}

func TestUnknownS3Event(t *testing.T) {
	service := mockService(eventCraft, nil)

	rcv := make(chan swarm.Msg[*events.S3EventRecord])
	ack := make(chan swarm.Msg[*events.S3EventRecord])
	go service.Run(rcv, ack)

	t.Run("EmptyS3Event", func(t *testing.T) {
		rcv <- swarm.Msg[*events.S3EventRecord]{
			Ctx: swarm.NewContext(context.TODO(), "test", "na"),
		}
		msg := <-ack
		it.Then(t).Should(it.Nil(msg.Ctx.Error))
	})

	t.Run("UnknownKey", func(t *testing.T) {
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

//------------------------------------------------------------------------------

func mockService(evt string, err error) *Service {
	batch := &mock{
		returnVal: &batch.SubmitJobOutput{},
		expectVal: &batch.SubmitJobInput{
			JobDefinition: aws.String("test-job"),
			JobQueue:      aws.String("test-queue"),
			ContainerOverrides: &types.ContainerOverrides{
				Environment: []types.KeyValuePair{
					{Name: aws.String("CRAFT_BUCKET"), Value: aws.String("test-s3")},
					{Name: aws.String("CRAFT_MODULE"), Value: aws.String("github.com/fogfish/craft")},
					{Name: aws.String("CRAFT_CDK_CONTEXT"), Value: aws.String(cdkContext)},
				},
			},
		},
	}

	scheduler := scheduler.New(batch, "test-queue", "test-job", "test-s3")

	fsys := fsys{returnVal: []byte(evt), returnErr: err}

	return New(fsys, scheduler)
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

type file []byte

func (file) Stat() (fs.FileInfo, error)   { return nil, nil }
func (file) Close() error                 { return nil }
func (f file) Read(b []byte) (int, error) { return copy(b, f), nil }

type fsys struct {
	returnVal []byte
	returnErr error
}

func (f fsys) Open(name string) (fs.File, error) {
	return file(f.returnVal), f.returnErr
}
