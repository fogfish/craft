//
// Copyright (C) 2024 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/craft
//

package main

import (
	"fmt"
	"log/slog"

	"github.com/fogfish/craft/internal/events"
	"github.com/fogfish/swarm"
)

type Scheduler interface {
	Schedule(evt events.EventCraft) error
}

type Service struct {
	scheduler Scheduler
}

func New(scheduler Scheduler) *Service {
	return &Service{
		scheduler: scheduler,
	}
}

func (s *Service) Run(rcv <-chan swarm.Msg[events.EventCraft], ack chan<- swarm.Msg[events.EventCraft]) {
	for msg := range rcv {
		if err := s.onEvtCraft(msg.Object); err != nil {
			ack <- msg.Fail(err)
			continue
		}

		ack <- msg
	}
}

func (s *Service) onEvtCraft(evt events.EventCraft) error {
	if evt.UID == "" || evt.Module == "" || evt.Context == nil {
		slog.Error("invalid event format", "evt", evt)
		return fmt.Errorf("invalid event format")
	}

	if err := s.scheduler.Schedule(evt); err != nil {
		slog.Error("failed to schedule event", "evt", evt, "err", err)
		return err
	}

	return nil
}
