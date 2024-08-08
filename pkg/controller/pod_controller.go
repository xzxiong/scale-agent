// Copyright 2024 Matrix Origin.
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

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apilabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/matrixorigin/scale-agent/pkg/config"
	"github.com/matrixorigin/scale-agent/pkg/errcode"
	selfpod "github.com/matrixorigin/scale-agent/pkg/pod"
)

// PodReconciler reconciles a Guestbook object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	cfg := config.GetConfiguration()
	l := log.FromContext(ctx)

	pod := &corev1.Pod{}
	if err = r.Get(ctx, req.NamespacedName, pod); err != nil {
		if !apierrors.IsNotFound(err) {
			l.Info("Get pod error, wait next reconcile", "error", err)
			return ctrl.Result{Requeue: true}, err
		}
		// TODO: trigger PodManger check pod exist. req.NamespacedName -> podManger.eventC
		return ctrl.Result{}, nil
	}

	// check the nodeName
	if pod.Spec.NodeName != cfg.App.GetNodeName() {
		l.Info("passed, pod not belong to this node", "podNode", pod.Spec.NodeName, "curNode", cfg.App.GetNodeName())
		return ctrl.Result{}, nil
	}

	// selected by selector
	selector, err := cfg.MO.GetLabelSelector()
	if err != nil {
		l.Info("failed to get config label selector error", "error", err)
		goto errorL
	} else if !selector.Matches(apilabels.Set(pod.Labels)) {
		l.Info("pod label not match", "labels", pod.Labels)
		goto successL
	} else {
		// check resource-rage
		resourceJson, exist := pod.Annotations[cfg.App.ResourceRange]
		if !exist {
			err = errcode.ErrorNoResourceRange
			l.Error(err, "failed to get pod ResourceRange")
			goto errorL
		}

		_, err = config.ParseResourceRange(resourceJson)
		if err != nil {
			l.Error(err, "failed to parse resource-range", "resource", resourceJson)
			goto errorL
		}
	}

	// fixme: check resource-ranges

	// send event
	err = selfpod.GetManger().NewPod(pod)
	if err != nil {
		l.Error(err, "failed to create pod", "pod", pod)
		goto errorL
	}

successL:
	l.Info("done")
	return ctrl.Result{}, nil

errorL:
	l.Info("wait next reconcile")
	return ctrl.Result{Requeue: true}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
