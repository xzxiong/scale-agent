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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/kubernetes/pkg/kubelet/cm"

	corev1 "k8s.io/api/core/v1"

	"github.com/matrixorigin/scale-agent/pkg/errcode"
	selfkubelet "github.com/matrixorigin/scale-agent/pkg/kubelet"
	selfutil "github.com/matrixorigin/scale-agent/pkg/util"
	"github.com/matrixorigin/scale-agent/pkg/util/cgroupv2"
)

type EventType int

const (
	EventTypeUnknown EventType = iota
	EventTypeNew
	EventTypeUpdate
	EventTypeDelete
	EventTypeScaleUp
	EventTypeScaleDown
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
	// double-check
	server, err := selfkubelet.GetKubeletServer(w.Logger)
	if err != nil {
		w.Info("[Warn] kubelet server get error, pod worker quit", "err", err)
		return
	}
	subSystems, err := cm.GetCgroupSubsystems()
	if err != nil {
		w.Info("[Warn] get cgroup subsystems error, pod worker quit", "err", err)
		return
	}
mainL:
	for {
		ticker.Reset(w.interval)
		select {
		// all events go serially
		case event := <-w.eventC:
			// TODO: metric - how long each event run.
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
			case EventTypeScaleUp:
				// TODO:
			case EventTypeScaleDown:
				// TODO:
			case EventTypeUnknown:
				fallthrough
			default:
				w.Error(errcode.ErrUnknownEvent, "pod worker catch unknown event", "event", event)
			}
		case ts := <-ticker.C:
			// TODO: metric - count ticker
			// TODO: impl regular check
			w.Info("regular check", "ts", ts)
			// TODO: get cgroup names
			cgroupNames, err := selfkubelet.GetCgroupNames(w.Logger, w.pod)
			if err != nil {
				w.Error(err, "failed to get cgroup names")
				selfutil.TagNeedAlert() // fixme: need alert
				// wait next loop
				continue
			}
			// TODO: calculate all need cgroup data
			// cgroup manager helps to calculate cgroup all data
			toolkit, err := cgroupv2.NewToolkit(w.ctx, w.Logger, cgroupNames, server, subSystems)
			// TODO: strategy manager trigger scale-up / scale-down

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
