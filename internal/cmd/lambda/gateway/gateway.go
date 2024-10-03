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
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/batch"

	"github.com/fogfish/craft/internal/events"
	"github.com/fogfish/craft/internal/scheduler"
	_ "github.com/fogfish/logger/v3"
	"github.com/fogfish/swarm"
	"github.com/fogfish/swarm/broker/eventbridge"
	"github.com/fogfish/swarm/dequeue"
)

func main() {
	// AWS Batch Service
	aws, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		slog.Error("fatal failure of batch client", "err", err)
		panic(err)
	}

	// AWS Batch Job Scheduler
	scheduler := scheduler.New(
		batch.NewFromConfig(aws),
		os.Getenv("CONFIG_BATCH_QUEUE"),
		os.Getenv("CONFIG_BATCH_JOB_CRAFT"),
		os.Getenv("CONFIG_S3"),
	)

	// Run event consumption loop
	service := New(scheduler)

	q, err := eventbridge.NewDequeuer("default",
		eventbridge.WithConfig(
			swarm.WithLogStdErr(),
		),
	)
	if err != nil {
		slog.Error("fatal failure of eventbrige client", "err", err)
		panic(err)
	}

	go service.Run(dequeue.Typed[events.EventCraft](q))

	q.Await()
}
