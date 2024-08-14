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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubelet/cm"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/matrixorigin/scale-agent/pkg/config"
	"github.com/matrixorigin/scale-agent/pkg/errcode"
	selfkubelet "github.com/matrixorigin/scale-agent/pkg/kubelet"
	"github.com/matrixorigin/scale-agent/pkg/scale"
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
	// scaleType used while typ = EventTypeScaleUp, EventTypeScaleDown
	scaleType scale.ScaleType
}

type Worker struct {
	logr.Logger
	ctx context.Context

	eventC   chan Event
	pod      *corev1.Pod
	quota    config.Quota
	cli      client.Client
	interval time.Duration
}

func (w *Worker) SendEvent(e Event) {
	w.eventC <- e
}

func (w *Worker) loop() {
	var ticker *time.Ticker
	var toolkit cgroupv2.Toolkit
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
			// fixme: metric - how long each event run.
			switch event.typ {
			case EventTypeNew, EventTypeUpdate:
				quota, found := config.GetQuotaFromPod(event.pod)
				if !found {
					w.Error(errcode.ErrNotFound, "pod not found main container")
					selfutil.TagNeedAlert()
					continue
				}
				w.pod = event.pod
				w.quota = config.GetFloorMemoryQuota(quota)
				// set other cgroup config.
				quotaConfig := config.GetQuotaConfig(w.quota)
				err = toolkit.SetCgroupLimit(w.quota, quotaConfig)
				w.Error(err, "failed to set cgroup limit") // fixme: try again.
				// TODO: set mo config again, if scale-up
			case EventTypeDelete:
				// should do nothing
				w.Info("go down")
				GetManger().EvictPod(w.pod)
				break mainL
			case EventTypeScaleUp:
				var nextQuota config.Quota
				if event.scaleType == scale.ScaleUpCpu {
					nextQuota = config.GetScaleUpCpuQuota(w.quota)
				} else {
					nextQuota = config.GetScaleUpMemoryQuota(w.quota)
				}
				// TODO: check node quota
				w.updatePodQuota(w.ctx, nextQuota)
			case EventTypeScaleDown:
				var nextQuota config.Quota
				if event.scaleType == scale.ScaleDownCpu {
					nextQuota = config.GetScaleDownCpuQuota(w.quota)
				} else {
					nextQuota = config.GetScaleDownMemoryQuota(w.quota)
				}
				// TODO: Set MO config to wait scale-down.
				w.updatePodQuota(w.ctx, nextQuota)
			case EventTypeUnknown:
				fallthrough
			default:
				w.Error(errcode.ErrUnknownEvent, "pod worker catch unknown event", "event", event)
			}
		case ts := <-ticker.C:
			// fixme: metric - count ticker
			// impl regular check
			w.Info("regular check", "ts", ts)
			// get cgroup names
			cgroupNames, err := selfkubelet.GetCgroupNames(w.Logger, w.pod)
			if err != nil {
				w.Error(err, "failed to get cgroup names")
				selfutil.TagNeedAlert()
				// wait next loop
				continue
			}
			// calculate all need cgroup data
			// cgroup manager helps to calculate cgroup all data
			if toolkit == nil {
				toolkit = cgroupv2.NewToolkit(w.ctx, w.Logger, cgroupNames, server, subSystems)
				err = toolkit.Init()
				if err != nil {
					toolkit = nil
					w.Error(err, "failed to create cgroup.v2 toolkit")
					continue
				}
			}
			// strategy manager trigger scale-up / scale-down
			scaleManager := scale.GetManager()
			if scale, typ := scaleManager.CheckScaleUp(toolkit); scale {
				go w.SendEvent(Event{typ: EventTypeScaleUp, scaleType: typ})
			}
			if scale, typ := scaleManager.CheckScaleDown(toolkit); scale {
				go w.SendEvent(Event{typ: EventTypeScaleDown, scaleType: typ})
			}
		case <-w.ctx.Done():
			w.Info("pod loop stopped")
			break mainL
		}
	}
}

func (w *Worker) updatePodQuota(ctx context.Context, quota config.Quota) error {
	cr := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      w.pod.Name,
			Namespace: w.pod.Namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, w.cli, cr, func() error {
		found := false
		for idx := range cr.Spec.Containers {
			if cr.Spec.Containers[idx].Name == "main" {
				found = true
				// fixme: check diff or not.
				cr.Spec.Containers[idx].Resources.Limits.Cpu().SetMilli(quota.Cpu.MilliValue())
				cr.Spec.Containers[idx].Resources.Requests.Cpu().SetMilli(quota.Cpu.MilliValue())
				cr.Spec.Containers[idx].Resources.Limits.Memory().Set(quota.Memory.Value() + config.DeltaMemory.Value())
				cr.Spec.Containers[idx].Resources.Requests.Memory().Set(quota.Memory.Value() + config.DeltaMemory.Value())
			}
		}

		// NOT found main container.
		if !found {
			w.Info("not found the main container.")
			return nil
		}

		if cr.Annotations == nil {
			cr.Annotations = make(map[string]string, 1)
		}
		cr.Annotations[config.AnnotationModifyResource] = time.Now().Format(time.RFC3339)

		return nil
	}); err != nil {
		w.Error(err, "failed to pod Resources", "ResourceVersion", cr.ResourceVersion)
		return err
	}

	return nil
}

const DefaultCheckInterval = 3 * time.Minute

func NewWorker(ctx context.Context, logger logr.Logger, pod *corev1.Pod, cli client.Client) *Worker {
	w := &Worker{
		Logger: logger.WithValues("pod", GetNamespacedName(pod)),
		ctx:    ctx,
		pod:    pod,
		cli:    cli,

		interval: DefaultCheckInterval,
	}
	return w
}
