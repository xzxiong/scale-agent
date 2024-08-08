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
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"

	libcontainercgroups "github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/kubelet"
	"k8s.io/kubernetes/pkg/kubelet/cadvisor"
	"k8s.io/kubernetes/pkg/kubelet/cm"

	"github.com/xzxiong/scale-agent-demo/pkg/util"
)

func buildContainerMgr() (*kubelet.Dependencies, error) {

	// construct a KubeletServer from kubeletFlags and kubeletConfig
	kubeletServer, err := GetKubeletServer()
	if err != nil {
		klog.ErrorS(err, "Failed to create a new kubelet configuration")
		os.Exit(1)
	}

	kubeletDeps, err := app.UnsecuredDependencies(kubeletServer, utilfeature.DefaultFeatureGate)
	_, err = app.UnsecuredDependencies(kubeletServer, utilfeature.DefaultFeatureGate)
	if err != nil {
		panic(fmt.Errorf("failed to construct kubelet dependencies: %w", err))
	}
	fmt.Printf("kubeletServer: %s\n", kubeletServer)
	fmt.Printf("kubeletDeps is key for container_manager & cgroup manager: %v\n", kubeletDeps)

	kubeDeps := kubeletDeps
	s := kubeletServer

	err = initCadvisorInterface(s, kubeDeps)
	if err != nil {
		return nil, err
	}

	err = newContainerManager(s, err, kubeDeps)
	if err != nil {
		return nil, err
	}
	// err = containerManager.Start()
	klog.Infof("NewContainerManager start: %v", kubeDeps.ContainerManager)

	return kubeDeps, nil
}

// initCadvisorInterface for get machineInfo by calling kubeDeps.CAdvisorInterface.MachineInfo()
// ref github.com/kubernetes/kubernetes/cmd/kubelet/app/server.go
func initCadvisorInterface(s *options.KubeletServer, kubeDeps *kubelet.Dependencies) error {
	var cgroupRoots []string
	nodeAllocatableRoot := cm.NodeAllocatableRoot(s.CgroupRoot, s.CgroupsPerQOS, s.CgroupDriver)
	cgroupRoots = append(cgroupRoots, nodeAllocatableRoot)
	kubeletCgroup, err := cm.GetKubeletContainer(s.KubeletCgroups)
	if err != nil {
		klog.InfoS("Failed to get the kubelet's cgroup. Kubelet system container metrics may be missing.", "err", err)
	} else if kubeletCgroup != "" {
		cgroupRoots = append(cgroupRoots, kubeletCgroup)
	}

	if s.RuntimeCgroups != "" {
		// RuntimeCgroups is optional, so ignore if it isn't specified
		cgroupRoots = append(cgroupRoots, s.RuntimeCgroups)
	}

	if s.SystemCgroups != "" {
		// SystemCgroups is optional, so ignore if it isn't specified
		cgroupRoots = append(cgroupRoots, s.SystemCgroups)
	}

	if kubeDeps.CAdvisorInterface == nil {
		imageFsInfoProvider := cadvisor.NewImageFsInfoProvider(s.ContainerRuntimeEndpoint)
		kubeDeps.CAdvisorInterface, err = cadvisor.New(imageFsInfoProvider, s.RootDirectory, cgroupRoots, cadvisor.UsingLegacyCadvisorStats(s.ContainerRuntimeEndpoint), s.LocalStorageCapacityIsolation)
		if err != nil {
			panic(err)
		}
	}
	// end init CAdvisorInterface
	return nil
}

func GetCgroupCpu(pod *corev1.Pod) *cm.ResourceConfig {

	// ref k8s.io/kubernetes@v1.28.4/pkg/kubelet/cm/cgroup_manager_linux.go

	// =======
	// ref cgm.SetCgroupConfig
	// =======
	cgm, err := buildCgroupMgr()
	if err != nil {
		panic(err)
	}

	//kubeletServer, err := GetKubeletServer()
	//if err != nil {
	//	panic(err)
	//}

	// kubeDeps, err := buildContainerMgr()
	// if err != nil {
	// 	panic(err)
	// }

	// subSystems, err := cm.GetCgroupSubsystems()
	// if err != nil {
	// 	panic(err)
	// }

	//cmgr := kubeDeps.ContainerManager
	//pcm := cmgr.NewPodContainerManager()
	//resCfg, err := pcm.GetPodCgroupConfig(nil, corev1.ResourceStorage)
	// TODO: pcm dependence on call qosContainersInfo, which is important to gen pod CgroupName
	// qosContainersInfo generate by qosContainerManager, construct while calling qosContainerManager.Start
	// pcm.Start called by cmgr.setupNode() <- cmgr.Start(....)
	// qosContainerManager, err := cm.NewQOSContainerManager(subsystems, cgroupRoot, nodeConfig, cgroupManager)
	//
	// deprecated:
	// podCgroupName, _ := pcm.GetPodContainerName(pod)
	podCgroupName, _ := GetPodContainerName(pod, cgm)

	// map: subsystem -> cgroup path
	//cgroupPaths := buildCgroupPaths(podCgroupName, kubeletServer.CgroupDriver, subSystems)
	//cpuCgroupPath := cgroupPaths[CgroupControllerCpu]

	fmt.Printf("cgroup path: %s\n", cgm.Name(podCgroupName))

	resourceConfig, err := cgm.GetCgroupConfig(podCgroupName, corev1.ResourceCPU)
	if err != nil {
		panic(err)
	}

	return resourceConfig
}

func setCgroupCpu(pod *corev1.Pod) error {

	// ref k8s.io/kubernetes@v1.28.4/pkg/kubelet/cm/cgroup_manager_linux.go

	// =======
	// ref cgm.SetCgroupConfig
	// =======
	// cgm, err := buildCgroupMgr()
	// if err != nil {
	// 	panic(err)
	// }

	kubeletServer, err := GetKubeletServer()
	if err != nil {
		panic(err)
	}

	kubeDeps, err := buildContainerMgr()
	if err != nil {
		panic(err)
	}

	subSystems, err := cm.GetCgroupSubsystems()
	if err != nil {
		panic(err)
	}

	cmgr := kubeDeps.ContainerManager
	pcm := cmgr.NewPodContainerManager()
	//resCfg, err := pcm.GetPodCgroupConfig(nil, corev1.ResourceStorage)
	podCgroupName, _ := pcm.GetPodContainerName(pod)

	// map: subsystem -> cgroup path
	cgroupPaths := buildCgroupPaths(podCgroupName, kubeletServer.CgroupDriver, subSystems)
	cpuCgroupPath := cgroupPaths[CgroupControllerCpu]

	// set CPU
	cpuPeriod := uint64(CPUPeriodUs)
	cpuMax := int64(CPUPeriodUs * 1.0)
	cpuShared := cm.MilliCPUToShares(cpuMax)
	err = setCgroupv2CpuConfig(cpuCgroupPath, &cm.ResourceConfig{
		CPUPeriod: &cpuPeriod,
		CPUQuota:  &cpuMax,
		CPUShares: &cpuShared,
	})

	return err
}

const CPUPeriodUs = 100000

const CgroupDriverSystemd = "systemd"

const CgroupControllerCpu = string(corev1.ResourceCPU)
const CgroupControllerMemory = string(corev1.ResourceMemory)
const CgroupControllerStorage = string(corev1.ResourceStorage) // this for Volume NOT for cgroup

// buildCgroupPaths ref k8s.io/kubernetes@v1.28.4/pkg/kubelet/cm/cgroup_manager_linux.go
func buildCgroupPaths(name cm.CgroupName, cgroupDriver string, subsystems *cm.CgroupSubsystems) map[string]string {
	// fixme: check
	cgroupFsAdaptedName := name.ToCgroupfs()
	if cgroupDriver == CgroupDriverSystemd {
		cgroupFsAdaptedName = name.ToSystemd()
	}
	cgroupPaths := make(map[string]string, len(subsystems.MountPoints))
	for key, val := range subsystems.MountPoints {
		cgroupPaths[key] = path.Join(val, cgroupFsAdaptedName)
	}
	return cgroupPaths
}

func buildCgroupMgr() (cm.CgroupManager, error) {
	kubeletServer, err := GetKubeletServer()
	if err != nil {
		panic(err)
	}

	kubeletDeps, err := app.UnsecuredDependencies(kubeletServer, utilfeature.DefaultFeatureGate)
	_, err = app.UnsecuredDependencies(kubeletServer, utilfeature.DefaultFeatureGate)
	if err != nil {
		panic(fmt.Errorf("failed to construct kubelet dependencies: %w", err))
	}
	fmt.Printf("kubeletServer: %s\n", kubeletServer)
	fmt.Printf("kubeletDeps is key for container_manager & cgroup manager: %v\n", kubeletDeps)

	kubeDeps := kubeletDeps
	s := kubeletServer

	err = initCadvisorInterface(s, kubeDeps)
	if err != nil {
		return nil, err
	}

	mgr, err := newCgroupManager(s, kubeDeps)
	if err != nil {
		panic(err)
	}
	return mgr, err
}

// setCgroupCpuConfig ref k8s.io/kubernetes@v1.28.4/pkg/kubelet/cm/cgroup_manager_linux.go
func setCgroupCpuConfig(cgroupPath string, resourceConfig *cm.ResourceConfig) error {
	if libcontainercgroups.IsCgroup2UnifiedMode() {
		return setCgroupv2CpuConfig(cgroupPath, resourceConfig)
	} else {
		// return setCgroupv1CpuConfig(cgroupPath, resourceConfig)
		return fmt.Errorf("NOT IMPLEMENTED")
	}
}

// setCgroupv2CpuConfig ref k8s.io/kubernetes@v1.28.4/pkg/kubelet/cm/cgroup_manager_linux.go
func setCgroupv2CpuConfig(cgroupPath string, resourceConfig *cm.ResourceConfig) error {
	if resourceConfig.CPUQuota != nil {
		if resourceConfig.CPUPeriod == nil {
			return fmt.Errorf("CpuPeriod must be specified in order to set CpuLimit")
		}
		cpuLimitStr := cm.Cgroup2MaxCpuLimit
		if *resourceConfig.CPUQuota > -1 {
			cpuLimitStr = strconv.FormatInt(*resourceConfig.CPUQuota, 10)
		}
		cpuPeriodStr := strconv.FormatUint(*resourceConfig.CPUPeriod, 10)
		cpuMaxStr := fmt.Sprintf("%s %s", cpuLimitStr, cpuPeriodStr)
		if err := os.WriteFile(filepath.Join(cgroupPath, "cpu.max"), []byte(cpuMaxStr), 0700); err != nil {
			return fmt.Errorf("failed to write %v to %v: %v", cpuMaxStr, cgroupPath, err)
		}
	}
	if resourceConfig.CPUShares != nil {
		cpuWeight := cm.CpuSharesToCpuWeight(*resourceConfig.CPUShares)
		cpuWeightStr := strconv.FormatUint(cpuWeight, 10)
		if err := os.WriteFile(filepath.Join(cgroupPath, "cpu.weight"), []byte(cpuWeightStr), 0700); err != nil {
			return fmt.Errorf("failed to write %v to %v: %v", cpuWeightStr, cgroupPath, err)
		}
	}
	return nil
}

const componentKubelet = "kubelet"

var chrootOnce sync.Once

// GetKubeletServer
// 1. chroot to rootfs
// 2. get kubelet cmdline
// 3. parse all cmdline args
// 4. load kubeletConfig
// 5. use cmdline args cover kubeletConfig's value
func GetKubeletServer() (*options.KubeletServer, error) {

	// init
	kubeletFlags := options.NewKubeletFlags()
	kubeletConfig, err := options.NewKubeletConfiguration()
	if err != nil {
		klog.ErrorS(err, "Failed to create a new kubelet configuration")
		os.Exit(1)
	}

	filePaths, err := filepath.Glob("/*")
	if err != nil {
		panic(err)
	}
	foundRootFs := false
	for _, filePath := range filePaths {
		fmt.Println(filePath)
		if filePath == util.DefaultRootfs {
			foundRootFs = true
		}
	}
	fmt.Printf("found rootfs(%s): %v\n", util.DefaultRootfs, foundRootFs)

	// Step 1.
	chrootOnce.Do(func() {
		rootfs := util.GetRootFS()
		err = util.Chroot(rootfs)
		if err != nil {
			panic(err)
		}
	})

	// Step 2. find kubelet progress
	var args []string
	processes, err := util.GetProcessList(true) // Step 1 already do the chroot
	if err != nil {
		panic(err)
	}
	for _, process := range processes {
		if process.Comm == componentKubelet {
			args = process.Args
			break
		}
	}
	if len(args) == 0 {
		panic(fmt.Errorf("kubelet cmdline args is empty"))
	}

	// Step 3. parse all cmdline args
	cleanFlagSet := pflag.NewFlagSet(componentKubelet, pflag.ContinueOnError)
	cleanFlagSet.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	kubeletFlags.AddFlags(cleanFlagSet)
	options.AddKubeletConfigFlags(cleanFlagSet, kubeletConfig)
	options.AddGlobalFlags(cleanFlagSet)
	adaptKubelet_1_23_3_ConfigFlags(cleanFlagSet)
	// initial flag parse, since we disable cobra's flag parsing
	if err := cleanFlagSet.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse kubelet flag: %w", err)
	}
	// check if there are non-flag arguments in the command line
	cmds := cleanFlagSet.Args()
	if len(cmds) > 0 {
		return nil, fmt.Errorf("unknown command %+s", cmds[0])
	}

	// Step 4.
	// load kubelet config file, if provided
	if len(kubeletFlags.KubeletConfigFile) > 0 {
		kubeletConfig, err = loadConfigFile(kubeletFlags.KubeletConfigFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubelet config file, path: %s, error: %w", kubeletFlags.KubeletConfigFile, err)
		}
	}

	// Step 5.
	if len(kubeletFlags.KubeletConfigFile) > 0 || len(kubeletFlags.KubeletDropinConfigDirectory) > 0 {
		// We must enforce flag precedence by re-parsing the command line into the new object.
		// This is necessary to preserve backwards-compatibility across binary upgrades.
		// See issue #56171 for more details.
		if err := kubeletConfigFlagPrecedence(kubeletConfig, args); err != nil {
			return nil, fmt.Errorf("failed to precedence kubeletConfigFlag: %w", err)
		}
		// update feature gates based on new config
		if err := utilfeature.DefaultMutableFeatureGate.SetFromMap(kubeletConfig.FeatureGates); err != nil {
			return nil, fmt.Errorf("failed to set feature gates from initial flags-based config: %w", err)
		}
	}

	// construct a KubeletServer from kubeletFlags and kubeletConfig
	kubeletServer := &options.KubeletServer{
		KubeletFlags:         *kubeletFlags,
		KubeletConfiguration: *kubeletConfig,
	}
	return kubeletServer, err
}

// ====================
// adapt old version
// ====================

type AdaptConfig struct {
	ContainerRuntime string // --container-runtime
}

var adaptConfig AdaptConfig

func adaptKubelet_1_23_3_ConfigFlags(mainfs *pflag.FlagSet) {
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	defer func() {
		deprecated := "This parameter should NOT be set in NEW Version."
		notDeprecated := map[string]bool{
			"notDeprecated-key": true,
		}
		fs.VisitAll(func(f *pflag.Flag) {
			if notDeprecated[f.Name] {
				return
			}
			f.Deprecated = deprecated
		})
		mainfs.AddFlagSet(fs)
	}()

	// --container-runtime string
	// The container runtime to use. Possible value: 'remote'. (default "remote") (DEPRECATED: will be removed in 1.27 as the only valid value is 'remote')
	fs.StringVar(&adaptConfig.ContainerRuntime, "container-runtime", adaptConfig.ContainerRuntime, "adapt 1.23.3 / 1.26 kubelet cmd-line")
}
