//
// Copyright (C) 2024 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/craft
//

package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	craftevents "github.com/fogfish/craft/internal/events"
	"github.com/fogfish/swarm"
)

type Scheduler interface {
	Schedule(evt craftevents.EventCraft) error
}

type Service struct {
	fsys      fs.FS
	scheduler Scheduler
}

func New(fsys fs.FS, scheduler Scheduler) *Service {
	return &Service{
		fsys:      fsys,
		scheduler: scheduler,
	}
}

func (s *Service) Run(rcv <-chan swarm.Msg[*events.S3EventRecord], ack chan<- swarm.Msg[*events.S3EventRecord]) {
	for msg := range rcv {
		if msg.Object == nil {
			ack <- msg
			continue
		}

		key := msg.Object.S3.Object.Key
		if strings.HasSuffix(key, craftevents.EVENT_CRAFT) {
			if err := s.onEvtCraft(msg.Object); err != nil {
				ack <- msg.Fail(err)
				continue
			}
		}

		ack <- msg
	}
}

func (s *Service) onEvtCraft(evt *events.S3EventRecord) error {
	fd, err := s.fsys.Open("/" + evt.S3.Object.Key)
	if err != nil {
		slog.Error("failed to access event", "key", evt.S3.Object.Key, "err", err)
		return err
	}
	defer fd.Close()

	var req craftevents.EventCraft
	if err := json.NewDecoder(fd).Decode(&req); err != nil {
		slog.Error("failed to decode event", "key", evt.S3.Object.Key, "err", err)
		return err
	}

	if req.UID == "" || req.Module == "" || req.Context == nil {
		slog.Error("failed to decode event", "key", evt.S3.Object.Key, "err", "invalid event format")
		return fmt.Errorf("invalid event format")
	}

	if err := s.scheduler.Schedule(req); err != nil {
		slog.Error("failed to schedule event", "key", evt.S3.Object.Key, "err", err)
		return err
	}

	return nil
}
