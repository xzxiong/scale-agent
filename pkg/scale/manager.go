package scale

import (
	"time"

	"github.com/matrixorigin/scale-agent/pkg/util/cgroupv2"
)

var _ Manager = (*BaseManager)(nil)

type BaseManager struct {
	lowerCpuRate float64
}

func (b *BaseManager) CheckScaleUp(toolkit cgroupv2.Toolkit) (bool, ScaleType) {
	// strategy 1
	rate := toolkit.CalculateThrottleRate()
	if rate > 0.0 {
		return true, ScaleUpCpu
	}
	// strategy 2
	if toolkit.HasMemoryHighEvent(time.Second) {
		return true, ScaleUpMemory
	}
	return false, ScaleNone
}

func (b *BaseManager) CheckScaleDown(toolkit cgroupv2.Toolkit) (bool, ScaleType) {
	cpuRate := toolkit.CalculateCpuRate()
	if cpuRate <= b.lowerCpuRate {
		// TODO: support window calculation, like last 5min
		return true, ScaleDownCpu
	}

	return false, ScaleNone
}

func NewBaseManager() *BaseManager {
	return &BaseManager{
		lowerCpuRate: 0.1,
	}
}
