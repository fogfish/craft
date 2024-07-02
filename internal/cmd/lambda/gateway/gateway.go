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
	"github.com/fogfish/craft/internal/scheduler"
	_ "github.com/fogfish/logger/v3"
	"github.com/fogfish/stream"
	"github.com/fogfish/swarm"
	"github.com/fogfish/swarm/broker/events3"
	"github.com/fogfish/swarm/queue"
)

func main() {
	// AWS Batch Service
	aws, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		slog.Error("fatal failure of batch client", "err", err)
		panic(err)
	}

	batch := batch.NewFromConfig(aws)

	// AWS Batch Job Scheduler
	scheduler := scheduler.New(
		batch,
		os.Getenv("CONFIG_BATCH_QUEUE"),
		os.Getenv("CONFIG_BATCH_JOB_CRAFT"),
		os.Getenv("CONFIG_S3"),
	)

	// Mount S3 as file system
	s3fs, err := stream.NewFS(os.Getenv("CONFIG_S3"))
	if err != nil {
		slog.Error("fatal failure of s3 client", "err", err)
		panic(err)
	}

	// Run event consumption loop
	service := New(s3fs, scheduler)

	q := queue.Must(events3.New(os.Getenv("CONFIG_S3"), swarm.WithLogStdErr()))

	go service.Run(events3.Dequeue(q))

	q.Await()
}
