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

//go:build !linux
// +build !linux

package kubelet

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/kubelet/cm"

	"github.com/matrixorigin/scale-agent/pkg/errcode"
)

func GetCgroupCpu(pod *corev1.Pod) *cm.ResourceConfig { return nil }
func GetKubeletServer(logr.Logger) (*options.KubeletServer, error) {
	return nil, errcode.ErrNotSupported
}
func IsCgroupV2() bool { return false }

const Mode = "kubelet"
