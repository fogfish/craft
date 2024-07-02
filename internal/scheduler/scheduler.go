//
// Copyright (C) 2024 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/craft
//

package scheduler

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	"github.com/aws/aws-sdk-go-v2/service/batch/types"
	"github.com/fogfish/craft/internal/events"
)

type JobQueue interface {
	SubmitJob(ctx context.Context, params *batch.SubmitJobInput, optFns ...func(*batch.Options)) (*batch.SubmitJobOutput, error)
}

type Service struct {
	api        JobQueue
	queue      string
	definition string
	bucket     string
}

func New(api JobQueue, queue string, definition string, bucket string) *Service {
	return &Service{
		api:        api,
		queue:      queue,
		definition: definition,
		bucket:     bucket,
	}
}

func (s *Service) Schedule(evt events.EventCraft) error {
	val, err := s.api.SubmitJob(context.Background(),
		&batch.SubmitJobInput{
			JobName:       aws.String(evt.UID),
			JobDefinition: aws.String(s.definition),
			JobQueue:      aws.String(s.queue),
			ContainerOverrides: &types.ContainerOverrides{
				Environment: []types.KeyValuePair{
					{Name: aws.String("CRAFT_BUCKET"), Value: aws.String(s.bucket)},
					{Name: aws.String("CRAFT_MODULE"), Value: aws.String(evt.Module)},
					{Name: aws.String("CRAFT_CDK_CONTEXT"), Value: aws.String(string(evt.Context))},
				},
			},
		},
	)
	if err != nil {
		return err
	}

	slog.Info("job scheduled", "uid", evt.UID, "job", val.JobId)

	return nil
}
