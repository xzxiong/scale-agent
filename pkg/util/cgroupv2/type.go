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

package cgroupv2

import (
	"time"

	"github.com/matrixorigin/scale-agent/pkg/config"
)

type Toolkit interface {
	CalculateThrottleRate() float64
	CalculateCpuRate() float64
	GetCpu() int
	GetMemory() int64
	GetMemoryEvent() MemoryEvents
	HasMemoryHighEvent(time.Duration) bool
	SetCgroupLimit(config.Quota, config.QuotaConfig) error
	Init() error
}

// ThrottlingData
// ref k8s.io/kubernetes/vendor/github.com/opencontainers/runc/libcontainer/cgroups/stats.go
type ThrottlingData struct {
	// Number of periods with throttling active
	Periods uint64 `json:"periods,omitempty"`
	// Number of periods when the container hit its throttling limit.
	ThrottledPeriods uint64 `json:"throttled_periods,omitempty"`
	// Aggregate time the container was throttled for in nanoseconds.
	ThrottledTime uint64 `json:"throttled_time,omitempty"`
}

// CpuUsage ref k8s.io/kubernetes/vendor/github.com/opencontainers/runc/libcontainer/cgroups/fs2/cpu.go
type CpuUsage struct {
	// Total CPU time consumed.
	// Units: nanoseconds.
	// cgroupv2 / usage_usec
	TotalUsage uint64 `json:"total_usage,omitempty"`
	// Time spent by tasks of the cgroup in kernel mode.
	// Units: nanoseconds.
	// cgroupv2 / system_usec
	UsageInKernelmode uint64 `json:"usage_in_kernelmode"`
	// Time spent by tasks of the cgroup in user mode.
	// Units: nanoseconds.
	// cgroupv2 / user_usec
	UsageInUsermode uint64 `json:"usage_in_usermode"`

	// used in cgroupv1, ref k8s.io/kubernetes/vendor/github.com/opencontainers/runc/libcontainer/cgroups/fs/cpuacct.go
	// PercpuUsage []uint64 `json:"percpu_usage,omitempty"`
	// PercpuUsageInKernelmode []uint64 `json:"percpu_usage_in_kernelmode"`
	// PercpuUsageInUsermode []uint64 `json:"percpu_usage_in_usermode"`
}

type CpuStat struct {
	CpuUsage       CpuUsage       `json:"cpu_usage,omitempty"`
	ThrottlingData ThrottlingData `json:"throttling_data,omitempty"`
}

type MemoryEvents struct {
	High    uint64 `json:"high,omitempty"`
	Max     uint64 `json:"max,omitempty"`
	Low     uint64 `json:"low,omitempty"`
	OOM     uint64 `json:"oom,omitempty"`
	OOMKill uint64 `json:"oom_kill,omitempty"`
}

// ref k8s.io/kubernetes@v1.28.4/pkg/kubelet/cm/cgroup_manager_linux.go
const (
	Cgroup2MemoryHigh   string = "memory.high"
	Cgroup2MaxCpuLimit  string = "max"
	Cgroup2MemoryEvents string = "memory.events"
	Cgroup2CpuStat      string = "cpu.stat"
)
