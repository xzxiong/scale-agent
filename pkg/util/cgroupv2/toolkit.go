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

//go:build linux
// +build linux

package cgroupv2

import (
	"bufio"
	"context"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/kubelet/cm"

	selfkubelet "github.com/matrixorigin/scale-agent/pkg/kubelet"
)

var _ Toolkit = (*LinuxToolkit)(nil)

type LinuxToolkit struct {
	ctx        context.Context
	logger     logr.Logger
	name       cm.CgroupName
	server     *options.KubeletServer
	subSystems *cm.CgroupSubsystems

	interval time.Duration

	lastCpuStats CpuStat
	lastCpuTime  time.Time
	curCpuStat   CpuStat
	curCpuTime   time.Time
	mux          sync.Mutex
}

func (t *LinuxToolkit) CalculateThrottleRate() float64 {
	t.mux.Lock()
	defer t.mux.Unlock()
	if t.lastCpuTime == t.curCpuTime {
		return 0.0
	} else if t.curCpuStat.ThrottlingData.ThrottledPeriods < t.lastCpuStats.ThrottlingData.ThrottledPeriods {
		t.logger.Info("[Warn] throttled periods reset",
			"current", t.curCpuStat.ThrottlingData.ThrottledPeriods,
			"last", t.lastCpuStats.ThrottlingData.ThrottledPeriods)
		// fixme: metric count
		return 0.0
	} else {
		return float64(t.curCpuStat.ThrottlingData.ThrottledTime-t.lastCpuStats.ThrottlingData.ThrottledTime) /
			float64(t.curCpuTime.Sub(t.lastCpuTime))
	}
}

func (t *LinuxToolkit) CalculateCpuRate() float64 {
	t.mux.Lock()
	defer t.mux.Unlock()
	if t.lastCpuTime == t.curCpuTime {
		return 0.0
	} else if t.curCpuStat.CpuUsage.TotalUsage < t.lastCpuStats.CpuUsage.TotalUsage {
		t.logger.Info("[Warn] cpu usage reset",
			"current", t.curCpuStat.CpuUsage.TotalUsage,
			"last", t.lastCpuStats.CpuUsage.TotalUsage)
		// fixme: metric count
		return 0.0
	} else {
		return float64(t.curCpuStat.CpuUsage.TotalUsage-t.lastCpuStats.CpuUsage.TotalUsage) /
			float64(t.curCpuTime.Sub(t.lastCpuTime))
	}
}

func (t *LinuxToolkit) GetCpu() int {
	return 0
}

func (t *LinuxToolkit) GetMemory() int64 {
	return 0
}

func (t *LinuxToolkit) GetMemoryEvent() MemoryEvents {
	memEvents, _, err := t.getMemoryEvents()
	if err != nil {
		return MemoryEvents{}
	}
	return memEvents
}

func (t *LinuxToolkit) registerNotify() {
	// ref vendor/github.com/opencontainers/runc/libcontainer/notify_v2_linux.go
	// TODO: send event to loop().
}

func (t *LinuxToolkit) Init() (err error) {
	t.lastCpuTime = time.Now()
	t.lastCpuStats, t.lastCpuTime, err = t.getCpuStat()
	if err != nil {
		return
	}
	t.curCpuStat, t.curCpuTime = t.lastCpuStats, t.lastCpuTime
	go t.loop()
	return nil
}

func (t *LinuxToolkit) loop() {
	var ticker time.Ticker
mainL:
	for {
		ticker.Reset(t.interval)
		select {
		case <-ticker.C:
			cpuStats, now, err := t.getCpuStat()
			if err != nil {
				// fixme: os.IsNotExist(err) case.
				t.logger.Error(err, "failed to get cpu stats")
				// fixme: nil this cycle stats
				continue
			}
			t.mux.Lock()
			t.lastCpuStats, t.lastCpuTime, t.curCpuStat, t.curCpuTime = t.curCpuStat, t.curCpuTime, cpuStats, now
			t.mux.Unlock()
		case <-t.ctx.Done():
			t.logger.Info("toolkit quit")
			break mainL
		}
	}
}

func (t *LinuxToolkit) getCpuStat() (stats CpuStat, ts time.Time, err error) {
	var key string
	var v uint64
	cgroupPaths := selfkubelet.BuildCgroupPaths(t.name, t.server.CgroupDriver, t.subSystems)
	cgroupPath := cgroupPaths[selfkubelet.CgroupControllerCpu]

	// ref  k8s.io/kubernetes/vendor/github.com/opencontainers/runc/libcontainer/cgroups/fs2/cpu.go
	// example cpu.stat
	// usage_usec 645411557023
	// user_usec 605879006885
	// system_usec 39532550137
	// nr_periods 0
	// nr_throttled 0
	// throttled_usec 0
	f, err := cgroups.OpenFile(cgroupPath, Cgroup2CpuStat, os.O_RDONLY)
	if err != nil {
		t.logger.Error(err, "failed to read cpu.stat file", "cgroup", cgroupPath)
		return CpuStat{}, time.Time{}, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, v, err = fscommon.ParseKeyValue(sc.Text())
		if err != nil {
			t.logger.Error(err, "failed to read cpu.stat file", "cgroup", cgroupPath)
			return
		}
		switch key {
		case "usage_usec":
			stats.CpuUsage.TotalUsage = v * 1000
		case "user_usec":
			stats.CpuUsage.UsageInUsermode = v * 1000
		case "system_usec":
			stats.CpuUsage.UsageInKernelmode = v * 1000
		case "nr_periods":
			stats.ThrottlingData.Periods = v
		case "nr_throttled":
			stats.ThrottlingData.ThrottledPeriods = v
		case "throttled_usec":
			stats.ThrottlingData.ThrottledTime = v * 1000
		}
	}
	if err = sc.Err(); err != nil {
		if os.IsNotExist(err) {
			t.logger.Error(err, "file cpu.stat is not exist", "cgroup", cgroupPath)
			return CpuStat{}, time.Time{}, err
		}
		t.logger.Error(err, "failed to read cpu.stat file", "cgroup", cgroupPath)
		return
	}
	return stats, time.Now(), nil
}

func (t *LinuxToolkit) getMemoryEvents() (stats MemoryEvents, ts time.Time, err error) {
	var key string
	var v uint64
	cgroupPaths := selfkubelet.BuildCgroupPaths(t.name, t.server.CgroupDriver, t.subSystems)
	cgroupPath := cgroupPaths[selfkubelet.CgroupControllerMemory]

	f, err := cgroups.OpenFile(cgroupPath, Cgroup2MemoryEvents, os.O_RDONLY)
	if err != nil {
		t.logger.Error(err, "failed to read memory.events file", "cgroup", cgroupPath)
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, v, err = fscommon.ParseKeyValue(sc.Text())
		if err != nil {
			t.logger.Error(err, "failed to read memory.events file", "cgroup", cgroupPath)
			return
		}
		switch key {
		case "high":
			stats.High = v
		case "max":
			stats.Max = v
		case "low":
			stats.Low = v
		case "oom":
			stats.OOM = v
		case "oom_kill":
			stats.OOMKill = v
		}
	}
	if err = sc.Err(); err != nil {
		if os.IsNotExist(err) {
			t.logger.Error(err, "file memory.events is not exist", "cgroup", cgroupPath)
			return MemoryEvents{}, time.Time{}, err
		}
		t.logger.Error(err, "failed to read memory.events file", "cgroup", cgroupPath)
		return
	}
	return stats, time.Now(), nil
}

func NewToolkit(ctx context.Context, logger logr.Logger, cgroupName cm.CgroupName, server *options.KubeletServer, subSystems *cm.CgroupSubsystems) Toolkit {
	t := &LinuxToolkit{
		ctx:        ctx,
		logger:     logger,
		name:       cgroupName,
		server:     server,
		subSystems: subSystems,

		interval: time.Second,
	}
	return t
}
