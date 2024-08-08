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

//go:build linux
// +build linux

package kubelet

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	cadvisorapi "github.com/google/cadvisor/info/v1"
	"github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	v1qos "k8s.io/kubernetes/pkg/apis/core/v1/helper/qos"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/kubelet"
	kubeletconfiginternal "k8s.io/kubernetes/pkg/kubelet/apis/config"
	"k8s.io/kubernetes/pkg/kubelet/cm"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/topology"
	"k8s.io/kubernetes/pkg/kubelet/eviction"
	evictionapi "k8s.io/kubernetes/pkg/kubelet/eviction/api"
	"k8s.io/kubernetes/pkg/kubelet/kubeletconfig/configfiles"
	"k8s.io/kubernetes/pkg/kubelet/stats/pidlimit"
	utilfs "k8s.io/kubernetes/pkg/util/filesystem"
	"k8s.io/utils/cpuset"
)

var gCgroup cm.CgroupName

var qosCM cm.QOSContainerManager

var dummyResourceListGetter = func() v1.ResourceList {
	return v1.ResourceList{}
}

var dummyActivePodsFunc cm.ActivePodsFunc = func() []*v1.Pod {
	return nil
}

// newCgroupManager returns a CgroupManager based on the passed options.
func newCgroupManager(s *options.KubeletServer, kubeDeps *kubelet.Dependencies) (cm.CgroupManager, error) {

	if s.CgroupsPerQOS && s.CgroupRoot == "" {
		klog.InfoS("--cgroups-per-qos enabled, but --cgroup-root was not specified.  defaulting to /")
		s.CgroupRoot = "/"
	}

	machineInfo, err := kubeDeps.CAdvisorInterface.MachineInfo()
	if err != nil {
		return nil, err
	}
	reservedSystemCPUs, err := getReservedCPUs(machineInfo, s.ReservedSystemCPUs)
	if err != nil {
		return nil, err
	}
	if reservedSystemCPUs.Size() > 0 {
		// at cmd option validation phase it is tested either --system-reserved-cgroup or --kube-reserved-cgroup is specified, so overwrite should be ok
		klog.InfoS("Option --reserved-cpus is specified, it will overwrite the cpu setting in KubeReserved and SystemReserved", "kubeReservedCPUs", s.KubeReserved, "systemReservedCPUs", s.SystemReserved)
		if s.KubeReserved != nil {
			delete(s.KubeReserved, "cpu")
		}
		if s.SystemReserved == nil {
			s.SystemReserved = make(map[string]string)
		}
		s.SystemReserved["cpu"] = strconv.Itoa(reservedSystemCPUs.Size())
		klog.InfoS("After cpu setting is overwritten", "kubeReservedCPUs", s.KubeReserved, "systemReservedCPUs", s.SystemReserved)
	}

	kubeReserved, err := parseResourceList(s.KubeReserved)
	if err != nil {
		return nil, fmt.Errorf("--kube-reserved value failed to parse: %w", err)
	}
	systemReserved, err := parseResourceList(s.SystemReserved)
	if err != nil {
		return nil, fmt.Errorf("--system-reserved value failed to parse: %w", err)
	}
	var hardEvictionThresholds []evictionapi.Threshold
	// If the user requested to ignore eviction thresholds, then do not set valid values for hardEvictionThresholds here.
	if !s.ExperimentalNodeAllocatableIgnoreEvictionThreshold {
		hardEvictionThresholds, err = eviction.ParseThresholdConfig([]string{}, s.EvictionHard, nil, nil, nil)
		if err != nil {
			return nil, err
		}
	}
	experimentalQOSReserved, err := cm.ParseQOSReserved(s.QOSReserved)
	if err != nil {
		return nil, fmt.Errorf("--qos-reserved value failed to parse: %w", err)
	}

	var cpuManagerPolicyOptions map[string]string
	if utilfeature.DefaultFeatureGate.Enabled(features.CPUManagerPolicyOptions) {
		cpuManagerPolicyOptions = s.CPUManagerPolicyOptions
	} else if s.CPUManagerPolicyOptions != nil {
		return nil, fmt.Errorf("CPU Manager policy options %v require feature gates %q, %q enabled",
			s.CPUManagerPolicyOptions, features.CPUManager, features.CPUManagerPolicyOptions)
	}

	var topologyManagerPolicyOptions map[string]string
	if utilfeature.DefaultFeatureGate.Enabled(features.TopologyManagerPolicyOptions) {
		topologyManagerPolicyOptions = s.TopologyManagerPolicyOptions
	} else if s.TopologyManagerPolicyOptions != nil {
		return nil, fmt.Errorf("topology manager policy options %v require feature gates %q enabled",
			s.TopologyManagerPolicyOptions, features.TopologyManagerPolicyOptions)
	}

	nodeConfig := cm.NodeConfig{
		RuntimeCgroupsName:    s.RuntimeCgroups,
		SystemCgroupsName:     s.SystemCgroups,
		KubeletCgroupsName:    s.KubeletCgroups,
		KubeletOOMScoreAdj:    s.OOMScoreAdj,
		CgroupsPerQOS:         s.CgroupsPerQOS,
		CgroupRoot:            s.CgroupRoot,
		CgroupDriver:          s.CgroupDriver,
		KubeletRootDir:        s.RootDirectory,
		ProtectKernelDefaults: s.ProtectKernelDefaults,
		NodeAllocatableConfig: cm.NodeAllocatableConfig{
			KubeReservedCgroupName:   s.KubeReservedCgroup,
			SystemReservedCgroupName: s.SystemReservedCgroup,
			EnforceNodeAllocatable:   sets.NewString(s.EnforceNodeAllocatable...),
			KubeReserved:             kubeReserved,
			SystemReserved:           systemReserved,
			ReservedSystemCPUs:       reservedSystemCPUs,
			HardEvictionThresholds:   hardEvictionThresholds,
		},
		QOSReserved:                             *experimentalQOSReserved,
		CPUManagerPolicy:                        s.CPUManagerPolicy,
		CPUManagerPolicyOptions:                 cpuManagerPolicyOptions,
		CPUManagerReconcilePeriod:               s.CPUManagerReconcilePeriod.Duration,
		ExperimentalMemoryManagerPolicy:         s.MemoryManagerPolicy,
		ExperimentalMemoryManagerReservedMemory: s.ReservedMemory,
		PodPidsLimit:                            s.PodPidsLimit,
		EnforceCPULimits:                        s.CPUCFSQuota,
		CPUCFSQuotaPeriod:                       s.CPUCFSQuotaPeriod.Duration,
		TopologyManagerPolicy:                   s.TopologyManagerPolicy,
		TopologyManagerScope:                    s.TopologyManagerScope,
		TopologyManagerPolicyOptions:            topologyManagerPolicyOptions,
	}
	// END> copy from NewContainerManager

	subsystems, err := cm.GetCgroupSubsystems()
	if err != nil {
		return nil, fmt.Errorf("failed to get mounted cgroup subsystems: %v", err)
	}

	fmt.Printf("subsystems\n")
	for k, path := range subsystems.MountPoints {
		fmt.Printf("system: %s -> %s\n", k, path)
	}

	// Turn CgroupRoot from a string (in cgroupfs path format) to internal CgroupName
	cgroupRoot := cm.ParseCgroupfsToCgroupName(nodeConfig.CgroupRoot)
	cgroupManager := cm.NewCgroupManager(subsystems, nodeConfig.CgroupDriver)
	// Check if Cgroup-root actually exists on the node
	if nodeConfig.CgroupsPerQOS {
		// this does default to / when enabled, but this tests against regressions.
		if nodeConfig.CgroupRoot == "" {
			return nil, fmt.Errorf("invalid configuration: cgroups-per-qos was specified and cgroup-root was not specified. To enable the QoS cgroup hierarchy you need to specify a valid cgroup-root")
		}

		// we need to check that the cgroup root actually exists for each subsystem
		// of note, we always use the cgroupfs driver when performing this check since
		// the input is provided in that format.
		// this is important because we do not want any name conversion to occur.
		if err := cgroupManager.Validate(cgroupRoot); err != nil {
			return nil, fmt.Errorf("invalid configuration: %w", err)
		}
		klog.InfoS("Container manager verified user specified cgroup-root exists", "cgroupRoot", cgroupRoot)
		// Include the top level cgroup for enforcing node allocatable into cgroup-root.
		// This way, all sub modules can avoid having to understand the concept of node allocatable.
		cgroupRoot = cm.NewCgroupName(cgroupRoot, defaultNodeAllocatableCgroupName)
	}
	klog.InfoS("Creating Container Manager object based on Node Config", "nodeConfig", nodeConfig)

	qosContainerManager, err := cm.NewQOSContainerManager(subsystems, cgroupRoot, nodeConfig, cgroupManager)
	if err != nil {
		return nil, err
	}
	qosContainerManager.Start(dummyResourceListGetter, dummyActivePodsFunc)
	qosCM = qosContainerManager

	return cgroupManager, nil
}

// copy from k8s.io/kubernetes/cmd/kubelet/app/server.go
// copy from k8s.io/kubernetes/cmd/kubelet/app/server_linux.go
// ========================================================

func newContainerManager(s *options.KubeletServer, err error, kubeDeps *kubelet.Dependencies) error {
	if s.CgroupsPerQOS && s.CgroupRoot == "" {
		klog.InfoS("--cgroups-per-qos enabled, but --cgroup-root was not specified.  defaulting to /")
		s.CgroupRoot = "/"
	}

	machineInfo, err := kubeDeps.CAdvisorInterface.MachineInfo()
	if err != nil {
		return err
	}
	reservedSystemCPUs, err := getReservedCPUs(machineInfo, s.ReservedSystemCPUs)
	if err != nil {
		return err
	}
	if reservedSystemCPUs.Size() > 0 {
		// at cmd option validation phase it is tested either --system-reserved-cgroup or --kube-reserved-cgroup is specified, so overwrite should be ok
		klog.InfoS("Option --reserved-cpus is specified, it will overwrite the cpu setting in KubeReserved and SystemReserved", "kubeReservedCPUs", s.KubeReserved, "systemReservedCPUs", s.SystemReserved)
		if s.KubeReserved != nil {
			delete(s.KubeReserved, "cpu")
		}
		if s.SystemReserved == nil {
			s.SystemReserved = make(map[string]string)
		}
		s.SystemReserved["cpu"] = strconv.Itoa(reservedSystemCPUs.Size())
		klog.InfoS("After cpu setting is overwritten", "kubeReservedCPUs", s.KubeReserved, "systemReservedCPUs", s.SystemReserved)
	}

	kubeReserved, err := parseResourceList(s.KubeReserved)
	if err != nil {
		return fmt.Errorf("--kube-reserved value failed to parse: %w", err)
	}
	systemReserved, err := parseResourceList(s.SystemReserved)
	if err != nil {
		return fmt.Errorf("--system-reserved value failed to parse: %w", err)
	}
	var hardEvictionThresholds []evictionapi.Threshold
	// If the user requested to ignore eviction thresholds, then do not set valid values for hardEvictionThresholds here.
	if !s.ExperimentalNodeAllocatableIgnoreEvictionThreshold {
		hardEvictionThresholds, err = eviction.ParseThresholdConfig([]string{}, s.EvictionHard, nil, nil, nil)
		if err != nil {
			return err
		}
	}
	experimentalQOSReserved, err := cm.ParseQOSReserved(s.QOSReserved)
	if err != nil {
		return fmt.Errorf("--qos-reserved value failed to parse: %w", err)
	}

	var cpuManagerPolicyOptions map[string]string
	if utilfeature.DefaultFeatureGate.Enabled(features.CPUManagerPolicyOptions) {
		cpuManagerPolicyOptions = s.CPUManagerPolicyOptions
	} else if s.CPUManagerPolicyOptions != nil {
		return fmt.Errorf("CPU Manager policy options %v require feature gates %q, %q enabled",
			s.CPUManagerPolicyOptions, features.CPUManager, features.CPUManagerPolicyOptions)
	}

	var topologyManagerPolicyOptions map[string]string
	if utilfeature.DefaultFeatureGate.Enabled(features.TopologyManagerPolicyOptions) {
		topologyManagerPolicyOptions = s.TopologyManagerPolicyOptions
	} else if s.TopologyManagerPolicyOptions != nil {
		return fmt.Errorf("topology manager policy options %v require feature gates %q enabled",
			s.TopologyManagerPolicyOptions, features.TopologyManagerPolicyOptions)
	}

	kubeDeps.ContainerManager, err = cm.NewContainerManager(
		kubeDeps.Mounter,
		kubeDeps.CAdvisorInterface,
		cm.NodeConfig{
			RuntimeCgroupsName:    s.RuntimeCgroups,
			SystemCgroupsName:     s.SystemCgroups,
			KubeletCgroupsName:    s.KubeletCgroups,
			KubeletOOMScoreAdj:    s.OOMScoreAdj,
			CgroupsPerQOS:         s.CgroupsPerQOS,
			CgroupRoot:            s.CgroupRoot,
			CgroupDriver:          s.CgroupDriver,
			KubeletRootDir:        s.RootDirectory,
			ProtectKernelDefaults: s.ProtectKernelDefaults,
			NodeAllocatableConfig: cm.NodeAllocatableConfig{
				KubeReservedCgroupName:   s.KubeReservedCgroup,
				SystemReservedCgroupName: s.SystemReservedCgroup,
				EnforceNodeAllocatable:   sets.NewString(s.EnforceNodeAllocatable...),
				KubeReserved:             kubeReserved,
				SystemReserved:           systemReserved,
				ReservedSystemCPUs:       reservedSystemCPUs,
				HardEvictionThresholds:   hardEvictionThresholds,
			},
			QOSReserved:                             *experimentalQOSReserved,
			CPUManagerPolicy:                        s.CPUManagerPolicy,
			CPUManagerPolicyOptions:                 cpuManagerPolicyOptions,
			CPUManagerReconcilePeriod:               s.CPUManagerReconcilePeriod.Duration,
			ExperimentalMemoryManagerPolicy:         s.MemoryManagerPolicy,
			ExperimentalMemoryManagerReservedMemory: s.ReservedMemory,
			PodPidsLimit:                            s.PodPidsLimit,
			EnforceCPULimits:                        s.CPUCFSQuota,
			CPUCFSQuotaPeriod:                       s.CPUCFSQuotaPeriod.Duration,
			TopologyManagerPolicy:                   s.TopologyManagerPolicy,
			TopologyManagerScope:                    s.TopologyManagerScope,
			TopologyManagerPolicyOptions:            topologyManagerPolicyOptions,
		},
		s.FailSwapOn,
		kubeDeps.Recorder,
		kubeDeps.KubeClient,
	)

	return nil
}

// parseResourceList parses the given configuration map into an API
// ResourceList or returns an error.
func parseResourceList(m map[string]string) (v1.ResourceList, error) {
	if len(m) == 0 {
		return nil, nil
	}
	rl := make(v1.ResourceList)
	for k, v := range m {
		switch v1.ResourceName(k) {
		// CPU, memory, local storage, and PID resources are supported.
		case v1.ResourceCPU, v1.ResourceMemory, v1.ResourceEphemeralStorage, pidlimit.PIDs:
			q, err := resource.ParseQuantity(v)
			if err != nil {
				return nil, fmt.Errorf("failed to parse quantity %q for %q resource: %w", v, k, err)
			}
			if q.Sign() == -1 {
				return nil, fmt.Errorf("resource quantity for %q cannot be negative: %v", k, v)
			}
			rl[v1.ResourceName(k)] = q
		default:
			return nil, fmt.Errorf("cannot reserve %q resource", k)
		}
	}
	return rl, nil
}

func getReservedCPUs(machineInfo *cadvisorapi.MachineInfo, cpus string) (cpuset.CPUSet, error) {
	emptyCPUSet := cpuset.New()

	if cpus == "" {
		return emptyCPUSet, nil
	}

	topo, err := topology.Discover(machineInfo)
	if err != nil {
		return emptyCPUSet, fmt.Errorf("unable to discover CPU topology info: %s", err)
	}
	reservedCPUSet, err := cpuset.Parse(cpus)
	if err != nil {
		return emptyCPUSet, fmt.Errorf("unable to parse reserved-cpus list: %s", err)
	}
	allCPUSet := topo.CPUDetails.CPUs()
	if !reservedCPUSet.IsSubsetOf(allCPUSet) {
		return emptyCPUSet, fmt.Errorf("reserved-cpus: %s is not a subset of online-cpus: %s", cpus, allCPUSet.String())
	}
	return reservedCPUSet, nil
}

func loadConfigFile(name string) (*kubeletconfiginternal.KubeletConfiguration, error) {
	const errFmt = "failed to load Kubelet config file %s, error %v"
	// compute absolute path based on current working dir
	kubeletConfigFile, err := filepath.Abs(name)
	if err != nil {
		return nil, fmt.Errorf(errFmt, name, err)
	}
	loader, err := configfiles.NewFsLoader(&utilfs.DefaultFs{}, kubeletConfigFile)
	if err != nil {
		return nil, fmt.Errorf(errFmt, name, err)
	}
	kc, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf(errFmt, name, err)
	}

	// EvictionHard may be nil if it was not set in kubelet's config file.
	// EvictionHard can have OS-specific fields, which is why there's no default value for it.
	// See: https://github.com/kubernetes/kubernetes/pull/110263
	if kc.EvictionHard == nil {
		kc.EvictionHard = eviction.DefaultEvictionHard
	}
	return kc, err
}

// newFlagSetWithGlobals constructs a new pflag.FlagSet with global flags registered
// on it.
func newFlagSetWithGlobals() *pflag.FlagSet {
	fs := pflag.NewFlagSet("", pflag.ExitOnError)
	// set the normalize func, similar to k8s.io/component-base/cli//flags.go:InitFlags
	fs.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	// explicitly add flags from libs that register global flags
	options.AddGlobalFlags(fs)
	return fs
}

// newFakeFlagSet constructs a pflag.FlagSet with the same flags as fs, but where
// all values have noop Set implementations
func newFakeFlagSet(fs *pflag.FlagSet) *pflag.FlagSet {
	ret := pflag.NewFlagSet("", pflag.ExitOnError)
	ret.SetNormalizeFunc(fs.GetNormalizeFunc())
	fs.VisitAll(func(f *pflag.Flag) {
		ret.VarP(cliflag.NoOp{}, f.Name, f.Shorthand, f.Usage)
	})
	return ret
}

// kubeletConfigFlagPrecedence re-parses flags over the KubeletConfiguration object.
// We must enforce flag precedence by re-parsing the command line into the new object.
// This is necessary to preserve backwards-compatibility across binary upgrades.
// See issue #56171 for more details.
func kubeletConfigFlagPrecedence(kc *kubeletconfiginternal.KubeletConfiguration, args []string) error {
	// We use a throwaway kubeletFlags and a fake global flagset to avoid double-parses,
	// as some Set implementations accumulate values from multiple flag invocations.
	fs := newFakeFlagSet(newFlagSetWithGlobals())
	// register throwaway KubeletFlags
	options.NewKubeletFlags().AddFlags(fs)
	// register new KubeletConfiguration
	options.AddKubeletConfigFlags(fs, kc)
	// Remember original feature gates, so we can merge with flag gates later
	original := kc.FeatureGates
	// avoid duplicate printing the flag deprecation warnings during re-parsing
	fs.SetOutput(io.Discard)
	// re-parse flags
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Add back feature gates that were set in the original kc, but not in flags
	for k, v := range original {
		if _, ok := kc.FeatureGates[k]; !ok {
			kc.FeatureGates[k] = v
		}
	}
	return nil
}

// copy from k8s.io/kubernetes@v1.28.4/pkg/kubelet/cm/pod_container_manager_linux.go
// ========================================================

// GetPodContainerName returns the CgroupName identifier, and its literal cgroupfs form on the host.
func GetPodContainerName(pod *v1.Pod, cgroupManager cm.CgroupManager) (cm.CgroupName, string) {
	podQOS := v1qos.GetPodQOS(pod)
	// Get the parent QOS container name
	var parentContainer cm.CgroupName
	qosContainersInfo := qosCM.GetQOSContainersInfo()
	switch podQOS {
	case v1.PodQOSGuaranteed:
		parentContainer = qosContainersInfo.Guaranteed
	case v1.PodQOSBurstable:
		parentContainer = qosContainersInfo.Burstable
	case v1.PodQOSBestEffort:
		parentContainer = qosContainersInfo.BestEffort
	}
	podContainer := cm.GetPodCgroupNameSuffix(pod.UID)

	// Get the absolute path of the cgroup
	cgroupName := cm.NewCgroupName(parentContainer, podContainer)
	// Get the literal cgroupfs name
	cgroupfsName := cgroupManager.Name(cgroupName)

	return cgroupName, cgroupfsName
}

// copy from k8s.io/kubernetes/pkg/kubelet/cm/node_container_manager_linux.go
// ========================================================

const (
	defaultNodeAllocatableCgroupName = "kubepods"
)
