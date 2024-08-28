/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	capsintumv1 "fastgshare/fastfunc/api/v1"

	fastpodv1 "github.com/KontonGu/FaST-GShare/pkg/apis/fastgshare.caps.in.tum/v1"
	fastpodclientset "github.com/KontonGu/FaST-GShare/pkg/client/clientset/versioned"
	fastpodlisters "github.com/KontonGu/FaST-GShare/pkg/client/listers/fastgshare.caps.in.tum/v1"
	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// FaSTFuncReconciler reconciles a FaSTFunc object
type FaSTFuncReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	promv1api     promv1.API
	fastpodLister fastpodlisters.FaSTPodLister
}

// +kubebuilder:rbac:groups=caps.in.tum.fastgshare,resources=fastfuncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=caps.in.tum.fastgshare,resources=fastfuncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=caps.in.tum.fastgshare,resources=fastfuncs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FaSTFunc object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *FaSTFuncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FaSTFuncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a Prometheus API client
	promClient, err := api.NewClient(api.Config{
		Address: "http://prometheus.fastgshare.svc.cluster.local:9090",
	})
	if err != nil {
		return err
	}
	r.promv1api = promv1.NewAPI(promClient)
	client, _ := fastpodclientset.NewForConfig(ctrl.GetConfigOrDie())
	stopCh := make(chan struct{})
	r.fastpodLister = getFaSTPodLister(client, "fast-gshare-fn", stopCh)
	fastpodv1.AddToScheme(r.Scheme)

	return ctrl.NewControllerManagedBy(mgr).
		For(&capsintumv1.FaSTFunc{}).
		Complete(r)
}
