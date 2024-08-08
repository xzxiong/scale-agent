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
	apires "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/matrixorigin/scale-agent/pkg/util"
)

// ResourceRange
// example: '{"min":{"cpu":"1","memory":"4Gi"},"max":{"cpu":"15","memory":"60Gi"}}'
type ResourceRange struct {
	Min Resource `json:"min"`
	Max Resource `json:"max"`
}

type Resource struct {
	Cpu    string `json:"cpu"`
	Memory string `json:"memory"`

	cpuVal    apires.Quantity `json:"-"`
	memoryVal apires.Quantity `json:"-"`
}

func (r *Resource) Parse() error {
	var err error
	r.cpuVal, err = apires.ParseQuantity(r.Cpu)
	if err != nil {
		return err
	}
	r.memoryVal, err = apires.ParseQuantity(r.Memory)
	if err != nil {
		return err
	}
	return nil
}

func (r *Resource) GetCpu() apires.Quantity    { return r.cpuVal }
func (r *Resource) GetMemory() apires.Quantity { return r.memoryVal }

func ParseResourceRange(jsonStr string) (*ResourceRange, error) {
	r := &ResourceRange{}
	if err := json.Unmarshal(util.UnsafeStringToBytes(jsonStr), r); err != nil {
		return nil, err
	}
	if err := r.Min.Parse(); err != nil {
		return nil, err
	}
	if err := r.Max.Parse(); err != nil {
		return nil, err
	}

	return r, nil
}
