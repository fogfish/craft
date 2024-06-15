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
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	"github.com/aws/aws-sdk-go-v2/service/batch/types"
	"github.com/fogfish/swarm"
)

type JobQueue interface {
	SubmitJob(ctx context.Context, params *batch.SubmitJobInput, optFns ...func(*batch.Options)) (*batch.SubmitJobOutput, error)
}

type Service struct {
	api    JobQueue
	queue  string
	deploy string
}

func New(api JobQueue, queue string, deploy string) *Service {
	return &Service{
		api:    api,
		queue:  queue,
		deploy: deploy,
	}
}

func (s *Service) Run(rcv <-chan swarm.Msg[*events.S3EventRecord], ack chan<- swarm.Msg[*events.S3EventRecord]) {
	for msg := range rcv {
		evt := msg.Object
		if evt == nil {
			ack <- msg
			continue
		}

		key := evt.S3.Object.Key
		if !strings.HasSuffix(key, "cdk.context.json") {
			ack <- msg
			continue
		}

		// naming convention for context file
		// module__job-name.cdk.context.json
		craft_cdk_context := filepath.Base(key)
		craft_target := filepath.Dir(key)
		craft_source := fmt.Sprintf("s3://%s/%s", evt.S3.Bucket.Name, craft_target)

		craft_mod := modName(key)
		craft_job := jobName(key)

		val, err := s.api.SubmitJob(context.Background(),
			&batch.SubmitJobInput{
				JobName:       aws.String(craft_job),
				JobDefinition: aws.String(s.deploy),
				JobQueue:      aws.String(s.queue),
				ContainerOverrides: &types.ContainerOverrides{
					Environment: []types.KeyValuePair{
						{Name: aws.String("CRAFT_SOURCE"), Value: aws.String(craft_source)},
						{Name: aws.String("CRAFT_TARGET"), Value: aws.String(craft_target)},
						{Name: aws.String("CRAFT_MODULE"), Value: aws.String(craft_mod)},
						{Name: aws.String("CRAFT_CDK_CONTEXT"), Value: aws.String(craft_cdk_context)},
					},
				},
			},
		)
		if err != nil {
			slog.Error("job failed", "key", key, "err", err)
			ack <- msg.Fail(err)
			continue
		}

		slog.Info("job sceduled", "key", key, "job", val.JobId)
		ack <- msg
	}
}

func modName(key string) string {
	base := filepath.Base(key)
	seq := strings.Split(base, "__")
	if len(seq) == 2 {
		return seq[0]
	}

	return "."
}

func jobName(key string) string {
	dir := filepath.Base(filepath.Dir(key))
	uid := strings.TrimSuffix(filepath.Base(key), ".cdk.context.json")

	return fmt.Sprintf("%s-%s", dir, uid)
}
