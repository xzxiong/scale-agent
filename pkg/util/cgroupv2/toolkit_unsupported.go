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
// limitations under the License

//go:build !linux
// +build !linux

package cgroupv2

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/kubelet/cm"

	"github.com/matrixorigin/scale-agent/pkg/config"
	"github.com/matrixorigin/scale-agent/pkg/errcode"
)

// calculate throttled_rate
var _ Toolkit = (*UnsupportedToolkit)(nil)

type UnsupportedToolkit struct{}

func (t UnsupportedToolkit) CalculateThrottleRate() float64        { return 0.0 }
func (t UnsupportedToolkit) CalculateCpuRate() float64             { return 0.0 }
func (t UnsupportedToolkit) GetCpu() int                           { return 0 }
func (t UnsupportedToolkit) GetMemory() int64                      { return 0 }
func (t UnsupportedToolkit) GetMemoryEvent() MemoryEvents          { return MemoryEvents{} }
func (t UnsupportedToolkit) HasMemoryHighEvent(time.Duration) bool { return false }
func (t UnsupportedToolkit) Init() error                           { return errcode.ErrNotSupported }

func (t UnsupportedToolkit) SetCgroupLimit(config.Quota, config.QuotaConfig) error { return nil }

func NewToolkit(context.Context, logr.Logger, cm.CgroupName, *options.KubeletServer, *cm.CgroupSubsystems) Toolkit {
	return UnsupportedToolkit{}
}
