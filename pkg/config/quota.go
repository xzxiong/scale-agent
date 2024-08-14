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

package config

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apires "k8s.io/apimachinery/pkg/api/resource"
)

var DeltaMemory = apires.MustParse("64Mi")

type Quota struct {
	Cpu    apires.Quantity `yaml:"cpu,omitempty"`
	Memory apires.Quantity `yaml:"memory,omitempty"`
}

type QuotaConfig struct {
	GOMaxProcs    apires.Quantity `yaml:"goMaxProcs,omitempty"`
	MemoryCache   apires.Quantity `yaml:"memory_cache,omitempty"`
	DiskCache     apires.Quantity `yaml:"disk_cache,omitempty"`
	DiskIOPS      int64           `yaml:"disk_iops,omitempty"`
	DiskBandWidth apires.Quantity `yaml:"disk_bw,omitempty"`
}

var Quota2Config = make(map[string]QuotaConfig, 128)
var allQuotas []Quota

func GetQuotaKey(quota Quota) string {
	return fmt.Sprintf("%s-%s", quota.Cpu.String(), quota.Memory.String())
}

func GetQuotaConfig(quota Quota) QuotaConfig {
	cfg, exist := Quota2Config[GetQuotaKey(quota)]
	if !exist {
		return CalculatedQuotaConfig(quota)
	}
	return cfg
}

var GiBQuantity = apires.MustParse("1Gi")
var BasicGiBValue = GiBQuantity.Value()

func CalculatedQuotaConfig(quota Quota) QuotaConfig {
	cfg := QuotaConfig{}
	cpuMilli := quota.Cpu.MilliValue() // as 1000
	memQuota := quota.Memory.Value()
	cfg.GOMaxProcs = *apires.NewMilliQuantity(cpuMilli+500, apires.DecimalSI)
	cfg.MemoryCache = quota.Memory.DeepCopy()
	cfg.DiskCache = quota.Memory.DeepCopy()
	cfg.DiskCache.Set(memQuota * 10)
	cfg.DiskIOPS = memQuota / BasicGiBValue * 500
	cfg.DiskBandWidth = quota.Memory.DeepCopy()
	cfg.DiskBandWidth.Set(memQuota * 50 / 1024)
	return cfg
}

type GeneratedQuotaConfig struct {
	quota Quota       `yaml:"-"`
	cfg   QuotaConfig `yaml:"-"`

	Cpu           string `yaml:"cpu,omitempty"`
	Memory        string `yaml:"memory,omitempty"`
	GOMaxProcs    string `yaml:"goMaxProcs,omitempty"`
	MemoryCache   string `yaml:"memory_cache,omitempty"`
	DiskCache     string `yaml:"disk_cache,omitempty"`
	DiskIOPS      int64  `yaml:"disk_iops,omitempty"`
	DiskBandWidth string `yaml:"disk_bw,omitempty"`
}

func NewGeneratedQuotaConfig(quota Quota, cfg QuotaConfig) GeneratedQuotaConfig {
	c := GeneratedQuotaConfig{
		quota: quota,
		cfg:   cfg,
	}
	c.Format()
	return c
}

func (c *GeneratedQuotaConfig) Format() {
	c.Cpu = c.quota.Cpu.String()
	c.Memory = c.quota.Memory.String()
	c.GOMaxProcs = c.cfg.GOMaxProcs.String()
	c.MemoryCache = c.cfg.MemoryCache.String()
	c.DiskCache = c.cfg.DiskCache.String()
	c.DiskIOPS = c.cfg.DiskIOPS
	c.DiskBandWidth = c.cfg.DiskBandWidth.String()
}

func (c *GeneratedQuotaConfig) Parse() {
	c.quota.Cpu = apires.MustParse(c.Cpu)
	c.quota.Memory = apires.MustParse(c.Memory)
	c.cfg.GOMaxProcs = apires.MustParse(c.GOMaxProcs)
	c.cfg.MemoryCache = apires.MustParse(c.MemoryCache)
	c.cfg.DiskCache = apires.MustParse(c.DiskCache)
	c.cfg.DiskIOPS = c.DiskIOPS
	c.cfg.DiskBandWidth = apires.MustParse(c.DiskBandWidth)
}

var maxQuota = Quota{
	Cpu:    apires.MustParse("1000"),
	Memory: apires.MustParse("1000Gi"),
}

var minQuota = Quota{
	Cpu:    apires.MustParse("0"),
	Memory: apires.MustParse("0Gi"),
}

func GetScaleUpCpuQuota(quota Quota) Quota {
	var target Quota = maxQuota
	for _, q := range allQuotas {
		if q.Cpu.MilliValue() <= quota.Cpu.MilliValue() {
			continue
		}
		if q.Memory.Value() < target.Memory.Value() {
			target = q
		}
	}
	if target == maxQuota {
		return quota
	}
	return target
}

func GetScaleUpMemoryQuota(quota Quota) Quota {
	var target Quota = maxQuota
	for _, q := range allQuotas {
		if q.Memory.Value() <= quota.Memory.Value() {
			continue
		}
		if q.Cpu.MilliValue() < target.Cpu.MilliValue() {
			target = q
		}
	}
	if target == maxQuota {
		return quota
	}
	return target
}

func GetScaleDownCpuQuota(quota Quota) Quota {
	var target Quota = minQuota
	for _, q := range allQuotas {
		if q.Cpu.MilliValue() >= quota.Cpu.MilliValue() {
			continue
		}
		if q.Memory.Value() > target.Memory.Value() {
			target = q
		}
	}
	if target == minQuota {
		return quota
	}
	return target
}

func GetScaleDownMemoryQuota(quota Quota) Quota {
	var target Quota = maxQuota
	for _, q := range allQuotas {
		if q.Memory.Value() >= quota.Memory.Value() {
			continue
		}
		if q.Cpu.MilliValue() > target.Cpu.MilliValue() {
			target = q
		}
	}
	if target == maxQuota {
		return quota
	}
	return target
}

func GetFloorMemoryQuota(quota Quota) Quota {
	var target Quota = quota
	target.Memory.Set(0)
	for _, q := range allQuotas {
		if q.Cpu.MilliValue() != quota.Cpu.MilliValue() {
			continue
		}
		if q.Memory.Value() > quota.Memory.Value() {
			continue
		}
		if q.Memory.Value() > target.Memory.Value() {
			target = q
		}
	}
	if target.Memory.Value() == 0 {
		return quota
	}
	return target
}

func GetQuotaFromPod(cr *corev1.Pod) (Quota, bool) {
	var quota Quota
	var found = false
	for idx := range cr.Spec.Containers {
		if cr.Spec.Containers[idx].Name == "main" {
			found = true
			quota.Cpu = cr.Spec.Containers[idx].Resources.Limits.Cpu().DeepCopy()
			quota.Memory = cr.Spec.Containers[idx].Resources.Limits.Memory().DeepCopy()
		}
	}
	return quota, found
}

func init() {
	// fixme: manual init config.
	const maxCPU float64 = 32
	const maxMem float64 = 64
	for cpu := 0.5; cpu <= maxCPU; cpu += 0.5 {
		mem := cpu * 2
		memQuota := GiBQuantity.DeepCopy()
		memQuota.Set(int64(float64(memQuota.Value()) * mem))
		quota := Quota{
			Cpu:    *apires.NewMilliQuantity(int64(cpu*100), apires.DecimalSI),
			Memory: memQuota,
		}
		allQuotas = append(allQuotas, quota)
	}
}
