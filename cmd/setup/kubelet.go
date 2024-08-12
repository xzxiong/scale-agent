package setup

import (
	selfkubelet "github.com/matrixorigin/scale-agent/pkg/kubelet"
	selfutil "github.com/matrixorigin/scale-agent/pkg/util"
)

func GetKubeletServer() error {
	_, err := selfkubelet.GetKubeletServer(selfutil.GetLogger())
	return err
}

func IsCgroupV2() (bool, error) {
	if err := GetKubeletServer(); err != nil {
		return false, err
	}
	return selfkubelet.IsCgroupV2(), nil
}
