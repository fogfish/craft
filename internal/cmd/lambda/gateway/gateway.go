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
	_ "github.com/fogfish/logger/v3"
	"github.com/fogfish/swarm"
	"github.com/fogfish/swarm/broker/events3"
	"github.com/fogfish/swarm/queue"
)

func main() {
	q := queue.Must(events3.New(os.Getenv("CONFIG_BUS"), swarm.WithLogStdErr()))

	aws, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		slog.Error("fatal failure of batch client", "err", err)
		panic(err)
	}

	service := New(
		batch.NewFromConfig(aws),
		os.Getenv("CONFIG_QUEUE"),
		os.Getenv("CONFIG_JOB_DEPLOY"),
	)

	go service.Run(events3.Dequeue(q))

	q.Await()
}
