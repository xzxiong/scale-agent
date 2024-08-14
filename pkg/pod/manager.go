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
	"sync"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
)

type Manager interface {
	NewPod(pod *corev1.Pod) error
	// DeletePod drove by event.
	DeletePod(pod *corev1.Pod) error
	// EvictPod cleanup all meta, NO new event.
	EvictPod(pod *corev1.Pod)
	Start() error
	Stop() error
}

var gMgr Manager

func GetManger() Manager {
	return gMgr
}

var _ Manager = (*DaemonSetManager)(nil)

type DaemonSetManager struct {
	ManagerConfig
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once

	eventC chan *corev1.Pod
	mux    sync.Mutex
	cache  map[string]*Worker
}

func (d *DaemonSetManager) Start() error {
	d.once.Do(func() {
		go d.loop()
	})
	return nil
}

func (d *DaemonSetManager) Stop() error {
	d.cancel()
	// fixme: wait exist
	return nil
}

func (d *DaemonSetManager) NewPod(pod *corev1.Pod) error {
	//TODO implement me
	panic("implement me")
}

func (d *DaemonSetManager) DeletePod(pod *corev1.Pod) error {
	//TODO implement me
	panic("implement me")
}

func (d *DaemonSetManager) EvictPod(pod *corev1.Pod) {
	//TODO implement me
	panic("implement me")
}

// loop is the main event center
func (d *DaemonSetManager) loop() {
	for {
		select {

		case pod := <-d.eventC:
			d.mux.Lock()
			cachedKey := GetNamespacedName(pod)
			worker, exist := d.cache[cachedKey]
			if exist {
				worker.SendEvent(Event{EventTypeUpdate, pod})
			} else {
				worker = NewWorker(d.ctx, d.logger, pod)
				d.cache[cachedKey] = worker
				go worker.loop()
				worker.SendEvent(Event{EventTypeNew, pod})
			}

		case <-d.ctx.Done():
		}
	}

}

type ManagerConfig struct {
	logger logr.Logger
}

type Option func(*ManagerConfig)

func (o Option) Apply(cfg *ManagerConfig) {
	o(cfg)
}

func WithLogger(logger logr.Logger) Option {
	return func(cfg *ManagerConfig) {
		cfg.logger = logger
	}
}

func NewDaemonSetManager(ctx context.Context, options ...Option) *DaemonSetManager {
	m := &DaemonSetManager{
		eventC: make(chan *corev1.Pod, 16),
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	for _, opt := range options {
		opt.Apply(&m.ManagerConfig)
	}

	return m
}
