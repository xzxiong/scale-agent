package setup

import (
	"context"
	"fmt"
	"os"
	"sync"

	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/matrixorigin/scale-agent/pkg/config"
	"github.com/matrixorigin/scale-agent/pkg/errcode"
)

func GetNodeName(ctx context.Context) (string, error) {

	clientset := GetK8sClient()

	podName := os.Getenv(config.EnvPodName)
	podNS := os.Getenv(config.EnvPodNamespace)
	setupLog.Info("base info", "namespace", podNS, "name", podName)
	if podName == "" {
		err := errcode.ErrNoPodName
		setupLog.Error(err, fmt.Sprintf("env %s is empty", config.EnvPodName))
		return "", err
	}
	if podNS == "" {
		err := errcode.ErrNoNamespace
		setupLog.Error(err, fmt.Sprintf("env %s is empty", config.EnvPodNamespace))
		return "", err
	}

	// 获取当前 Pod 的信息
	pod, err := clientset.CoreV1().Pods(podNS).Get(ctx, podName, v1.GetOptions{})
	if err != nil {
		setupLog.Error(err, "failed to load current pod info")
		return "", err
	}
	return pod.Spec.NodeName, nil
}

var getClinetOnce sync.Once
var gClientset *kubernetes.Clientset

func GetK8sClient() *kubernetes.Clientset {
	getClinetOnce.Do(func() {
		config := controllerruntime.GetConfigOrDie()
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}
		gClientset = clientset
	})
	return gClientset
}

func InitManagerIndexer(mgr manager.Manager, ctx context.Context) {
	indexer := mgr.GetFieldIndexer()
	indexer.IndexField(ctx, &v12.Pod{}, config.K8sFieldNodeName, func(o client.Object) []string {
		nodeName := o.(*v12.Pod).Spec.NodeName
		if nodeName != "" {
			return []string{nodeName}
		}
		return nil
	})
}
