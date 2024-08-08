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

package util

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type ProcessCmdLine struct {
	Comm string
	Args []string
}

func GetProcessList(inHostNamespace bool) ([]ProcessCmdLine, error) {
	//format := "user,pid,ppid,stime,pcpu,pmem,rss,vsz,stat,time,comm,psr,cgroup"
	format := "comm,cmd"
	out, err := getPsOutput(inHostNamespace, format)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	ret := make([]ProcessCmdLine, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 { // It is the end-line, normally.
			continue
		}
		fmt.Printf("line: %s\n", line)
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("invalid format: %s", line)
		}
		ret = append(ret, ProcessCmdLine{Comm: fields[0], Args: fields[2:]})
	}

	return ret, nil
}

func getPsOutput(inHostNamespace bool, format string) ([]byte, error) {
	args := []string{}
	command := "ps"
	if !inHostNamespace {
		command = "/usr/sbin/chroot"
		rootfs := GetRootFS()
		args = append(args, rootfs, "ps")
	}
	args = append(args, "-e", "-o", format)
	out, err := exec.Command(command, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute %q command: %v", command, err)
	}
	return out, err
}

const EnvRootfs = "ROOTFS"
const DefaultRootfs = "/rootfs"

// GetRootFS get from evn EnvRootfs val first, if nil return DefaultRootfs
func GetRootFS() string {
	v := os.Getenv(EnvRootfs)
	if len(v) == 0 {
		return DefaultRootfs
	}
	return v
}
