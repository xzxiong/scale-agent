// Copyright 2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pod

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/matrixorigin/scale-agent/pkg/errcode"
	"time"

	corev1 "k8s.io/api/core/v1"
)

type EventType int

const (
	EventTypeUnknown EventType = iota
	EventTypeNew
	EventTypeUpdate
	EventTypeDelete
)

type Event struct {
	typ EventType
	pod *corev1.Pod
}

type Worker struct {
	logr.Logger
	ctx context.Context

	eventC   chan Event
	pod      *corev1.Pod
	interval time.Duration
}

func (w *Worker) SendEvent(e Event) {
	w.eventC <- e
}

func (w *Worker) loop() {
	var ticker *time.Ticker
mainL:
	for {
		ticker.Reset(w.interval)
		select {
		case event := <-w.eventC:

			switch event.typ {
			case EventTypeNew:
				// TODO:
			case EventTypeUpdate:
				// TODO:
			case EventTypeDelete:
				// should do nothing
				w.Info("go down")
				GetManger().EvictPod(w.pod)
				break mainL
			case EventTypeUnknown:
				fallthrough
			default:
				w.Error(errcode.ErrUnknownEvent, "pod worker catch unknown event", "event", event)
			}

		case ts := <-ticker.C:
			// TODO: impl regular check
			w.Info("regular check", "ts", ts)

		case <-w.ctx.Done():
			w.Info("pod loop stopped")
			break mainL
		}
	}
}

const DefaultCheckInterval = 3 * time.Minute

func NewWorker(ctx context.Context, logger logr.Logger, pod *corev1.Pod) *Worker {
	w := &Worker{
		Logger: logger.WithValues("pod", GetNamespacedName(pod)),
		ctx:    ctx,
		pod:    pod,

		interval: DefaultCheckInterval,
	}
	return w
}
