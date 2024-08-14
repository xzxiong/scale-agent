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

package scale

import (
	"sync"

	"github.com/matrixorigin/scale-agent/pkg/util/cgroupv2"
)

type ScaleType int

const (
	ScaleNone ScaleType = iota
	ScaleUpCpu
	ScaleUpMemory

	ScaleDownCpu
	ScaleDownMemory
)

type Manager interface {
	CheckScaleUp(cgroupv2.Toolkit) (bool, ScaleType)
	CheckScaleDown(cgroupv2.Toolkit) (bool, ScaleType)
}

var once sync.Once
var mgr Manager

func GetManager() Manager {
	once.Do(func() {
		mgr = NewBaseManager()
	})
	return mgr
}
