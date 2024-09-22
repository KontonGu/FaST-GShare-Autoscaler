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
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fastfuncv1 "fastgshare/fastfunc/api/v1"

	fastpodv1 "github.com/KontonGu/FaST-GShare/pkg/apis/fastgshare.caps.in.tum/v1"
	fastpodclientset "github.com/KontonGu/FaST-GShare/pkg/client/clientset/versioned"
	fastpodinformer "github.com/KontonGu/FaST-GShare/pkg/client/informers/externalversions"
	fastpodlisters "github.com/KontonGu/FaST-GShare/pkg/client/listers/fastgshare.caps.in.tum/v1"
	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// FaSTFuncReconciler reconciles a FaSTFunc object
type FaSTFuncReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	promv1api     promv1.API
	fastpodLister fastpodlisters.FaSTPodLister
}

var once sync.Once

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

	once.Do(func() {
		go r.persistentReconcile()
	})

	return ctrl.Result{}, nil
}

func (r *FaSTFuncReconciler) persistentReconcile() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.TODO()
		// logger := log.FromContext(ctx)

		var allFaSTfuncs fastfuncv1.FaSTFuncList
		if err := r.List(ctx, &allFaSTfuncs); err != nil {
			klog.Error(err, "Failed to get FaSTFuncs.")
			return
		}

		for _, fstfunc := range allFaSTfuncs.Items {
			funcName := fstfunc.ObjectMeta.Name
			// make a Prometheus query to get the RPS of the function
			query := fmt.Sprintf("rate(gateway_function_invocation_total{function_name='%s.%s'}[10s])", funcName, fstfunc.ObjectMeta.Namespace)
			klog.Infof("Prometheus Query: %s.", query)
			queryRes, _, err := r.promv1api.Query(ctx, query, time.Now())
			curRPS := float64(0.0)
			if err != nil {
				klog.Errorf("Error Failed to get RPS of function %s.", funcName)
				continue
			}

			if queryRes.(model.Vector).Len() != 0 {
				klog.Infof("Current rps vec for function %s is %v.", funcName, queryRes)
				curRPS = float64(queryRes.(model.Vector)[0].Value)
			}
			klog.Infof("Current rps for function %s is %f.", funcName, curRPS)

		}

	}
}

func getFaSTPodLister(client fastpodclientset.Interface, namespace string, stopCh chan struct{}) fastpodlisters.FaSTPodLister {
	// create a shared informer factory for the FaasShare API group
	informerFactory := fastpodinformer.NewSharedInformerFactoryWithOptions(
		client,
		0,
		fastpodinformer.WithNamespace(namespace),
	)
	// retrieve the shared informer for FaSTPods
	fastpodInformer := informerFactory.Fastgshare().V1().FaSTPods().Informer()
	informerFactory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh, fastpodInformer.HasSynced) {
		return nil
	}
	// create a lister for FaSTPods using the shared informer's indexers
	fastpodLister := fastpodlisters.NewFaSTPodLister(fastpodInformer.GetIndexer())
	return fastpodLister

}

// SetupWithManager sets up the controller with the Manager.
func (r *FaSTFuncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a Prometheus API client
	promClient, err := api.NewClient(api.Config{
		Address: "http://prometheus.fastgshare.svc.cluster.local:9090",
	})
	if err != nil {
		klog.Error("Failed to create the Prometheus client.")
		return err
	}
	r.promv1api = promv1.NewAPI(promClient)
	client, _ := fastpodclientset.NewForConfig(ctrl.GetConfigOrDie())
	stopCh := make(chan struct{})
	klog.Info("Before function getFaSTPodLister")
	r.fastpodLister = getFaSTPodLister(client, "fast-gshare-fn", stopCh)
	klog.Info("After function getFaSTPodLister")
	fastpodv1.AddToScheme(r.Scheme)

	return ctrl.NewControllerManagedBy(mgr).
		For(&fastfuncv1.FaSTFunc{}).
		Complete(r)
}
