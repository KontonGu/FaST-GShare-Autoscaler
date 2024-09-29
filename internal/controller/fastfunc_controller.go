/*
Copyright 2024 FaST-GShare Authors, KontonGu (Jianfeng Gu), et. al.
@Techinical University of Munich, CAPS Cloud Team

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
	"math"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// FaSTFuncReconciler reconciles a FaSTFunc object
type FaSTFuncReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	promv1api     promv1.API
	fastpodLister fastpodlisters.FaSTPodLister
}

type FaSTPodConfig struct {
	Quota       int64 // percentage
	SMPartition int64 // percentage
	Mem         int64
	Replicas    int64
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
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	tried := false
	for range ticker.C {
		ctx := context.TODO()
		// logger := log.FromContext(ctx)

		var allFaSTfuncs fastfuncv1.FaSTFuncList
		if err := r.List(ctx, &allFaSTfuncs); err != nil {
			klog.Error(err, "Failed to get FaSTFuncs.")
			return
		}

		// reconcile for each FaSTFunc
		for _, fstfunc := range allFaSTfuncs.Items {

			//KONTON_Testing
			funcName := fstfunc.ObjectMeta.Name
			klog.Infof("Checking FaSTFunc %s.", funcName)
			if !tried {
				tried = true
				config, _ := getMostEfficientConfig()
				config.Replicas = 1
				var configslist []*FaSTPodConfig
				configslist = append(configslist, &config)
				config2, _ := getMostEfficientConfig()
				configslist = append(configslist, &config2)
				config2.Quota = int64(40)
				fastpods, _ := r.configs2FaSTPods(&fstfunc, configslist)
				// err := r.reconcileDesiredFaSTFunc(fastpods)
				err := r.Create(context.TODO(), fastpods[0])
				if err != nil {
					klog.Errorf("Failed to create the fastfunc %s.", funcName)
				}
				// err = r.Create(context.TODO(), fastpods[1])
				// if err != nil {
				// 	klog.Errorf("Failed to create the fastpod %s.", fastpods[1].Name)
				// }
			}
			//KONTON_Testing_end

			// funcName := fstfunc.ObjectMeta.Name
			klog.Infof("Checking FaSTFunc %s.", funcName)
			// make a Prometheus query to get the RPS of the function
			query := fmt.Sprintf("rate(gateway_function_invocation_total{function_name='%s.%s'}[10s])", funcName, fstfunc.ObjectMeta.Namespace)
			klog.Infof("Prometheus Query: %s.", query)
			queryRes, _, err := r.promv1api.Query(ctx, query, time.Now())
			curRPS := float64(0.0)
			if err != nil {
				klog.Errorf("Error Failed to get RPS of function %s: %s.", funcName, err.Error())
				continue
			}

			if queryRes.(model.Vector).Len() != 0 {
				klog.Infof("Current rps vec for function %s is %v.", funcName, queryRes)
				curRPS = float64(queryRes.(model.Vector)[0].Value)
			}
			klog.Infof("Current rps for function %s is %f.", funcName, curRPS)

			// make a Prometheus query to get the RPS of past 30s
			pastRPS := float64(0.0)
			pastquery := fmt.Sprintf("rate(gateway_function_invocation_total{function_name='%s.%s'}[30s])", funcName, fstfunc.ObjectMeta.Namespace)
			// klog.Infof("Prometheus Query: %s.", pastquery)
			pastqueryVec, _, err := r.promv1api.Query(ctx, pastquery, time.Now())
			if err != nil {
				klog.Errorf("Error Failed to get past 30s RPS of function %s.", funcName)
				continue
			}
			if pastqueryVec.(model.Vector).Len() != 0 {
				klog.Infof("Past 30s rps vec for function %s is %v.", funcName, pastqueryVec)
				pastRPS = float64(pastqueryVec.(model.Vector)[0].Value)
			}
			klog.Infof("Past 30s rps for function %s is %f.", funcName, pastRPS)

			// make a Prometheus query to get the RPS of old 30s
			oldRPS := float64(0.0)
			oldTime := time.Now().Add(-30 * time.Second)
			// klog.Infof("Prometheus Query: %s.", pastquery)
			oldqueryVec, _, err := r.promv1api.Query(ctx, pastquery, oldTime)
			if err != nil {
				klog.Errorf("Error Failed to get old 30s RPS of function %s.", funcName)
				continue
			}
			if oldqueryVec.(model.Vector).Len() != 0 {
				klog.Infof("Old 30s rps vec for function %s is %v.", funcName, oldqueryVec)
				oldRPS = float64(oldqueryVec.(model.Vector)[0].Value)
			}
			klog.Infof("Old 30s rps for function %s is %f.", funcName, oldRPS)

			// desiredFastpods := r.getDesiredFastfuncSpec(&fstfunc, curRPS, pastRPS, oldRPS)
			// if desiredFastpods == nil {
			// 	continue
			// }
			// err = r.reconcileDesiredFaSTFunc(desiredFastpods)
			// if err != nil {
			// 	klog.Errorf("Error Cannot reconcile the desired FaSTFunc %s.", funcName)
			// 	continue
			// }
		}
	}
}

// Get the desired FaSTFunc Specification for scaling
func (r *FaSTFuncReconciler) getDesiredFastfuncSpec(fastfunc *fastfuncv1.FaSTFunc, curRPS, pastRPS, oldRPS float64) []*fastpodv1.FaSTPod {
	// Compare current processing capability and current rps, if current rps is larger, scaling up, otherwise scaling down
	nominalRPSmap, totalRPSCap := r.getFuncCurrentNominalRPS(fastfunc.ObjectMeta.Name)
	// rps difference
	deltaReqs := curRPS - totalRPSCap
	klog.Infof("RPS difference (DeltaRPS) = %f.", deltaReqs)
	// scaling := (pastRPS - oldRPS) > 0

	// if deltaReqs > 0 && !scaling {
	// 	klog.Info("Scaling up initially.")
	// 	configslist := r.schedule(deltaReqs)
	// 	for _, config := range configslist {
	// 		config.Replicas += nominalRPSmap[getResKeyName(config.Quota, config.SMPartition)]
	// 	}
	// 	fastpods, _ := r.configs2FaSTPods(fastfunc, configslist)
	// 	return fastpods
	// } else if scaling && oldRPS > 0 && pastRPS > 0 {
	// 	klog.Infof("Scaling up normally")
	// 	configlist := r.schedule(deltaReqs)
	// 	for _, config := range configlist {
	// 		klog.Infof("originally required replica: %d", config.Replicas)
	// 		factor := pastRPS / oldRPS
	// 		var tmp float64
	// 		tmp = float64(config.Replicas) * factor
	// 		config.Replicas = int64(math.Ceil(tmp))
	// 		klog.Infof("new replica: %d", config.Replicas)
	// 	}
	// 	fastpods, _ := r.configs2FaSTPods(fastfunc, configlist)
	// 	return fastpods
	// } else if deltaReqs < 0 {
	// 	// scaling down
	// 	klog.Info("Scaling down")
	// 	deltaReqs = deltaReqs * (-1)
	// 	for key, replica := range nominalRPSmap {
	// 		quota, smPartition := parseFromKeyName(key)
	// 		rpsUnit := retrieveResource2RPSCapability(fastfunc.ObjectMeta.Name, float64(quota)/100.0, int64(smPartition))
	// 		n := int64(deltaReqs / rpsUnit)
	// 		if replica > n {
	// 			replica = replica - n
	// 			deltaReqs = deltaReqs - float64(n)*rpsUnit
	// 		} else {
	// 			replica = 0
	// 			deltaReqs = deltaReqs - float64(replica)*rpsUnit
	// 		}
	// 		r.scaleDownFaSTFunc(fastfunc.ObjectMeta.Name+key, replica)
	// 	}

	// }

	// KONTON Testing Start
	if deltaReqs > 0.2*totalRPSCap {
		klog.Info("Scaling up initially.")
		configslist := r.schedule(deltaReqs)
		for _, config := range configslist {
			config.Replicas += nominalRPSmap[getResKeyName(config.Quota, config.SMPartition)]
		}
		fastpods, _ := r.configs2FaSTPods(fastfunc, configslist)
		return fastpods
	} else if deltaReqs < 0 {
		if float64(-1*deltaReqs) > 0.2*totalRPSCap {
			// scaling down
			klog.Info("Scaling down")
			deltaReqs = deltaReqs * (-1)
			for key, replica := range nominalRPSmap {
				quota, smPartition := parseFromKeyName(key)
				rpsUnit := retrieveResource2RPSCapability(fastfunc.ObjectMeta.Name, float64(quota)/100.0, int64(smPartition))
				n := int64(deltaReqs / rpsUnit)
				if replica > n {
					replica = replica - n
					deltaReqs = deltaReqs - float64(n)*rpsUnit
				} else {
					replica = 0
					deltaReqs = deltaReqs - float64(replica)*rpsUnit
				}
				r.scaleDownFaSTFunc(fastfunc.ObjectMeta.Name+key, replica)
			}
		}
	}
	// KONTON Testing End
	return nil
}

// Get processing capability (nominal RPS) of existing FaSTPods of the function funcnname
func (r *FaSTFuncReconciler) getFuncCurrentNominalRPS(funcname string) (map[string]int64, float64) {
	klog.Infof("To get the current nominal rps of the function %s.", funcname)
	selector := labels.SelectorFromSet(labels.Set{"faas_function": funcname})
	fastpodList, _ := r.fastpodLister.List(selector)
	quota := 0.0
	smPartition := int64(100)
	klog.Infof("Number of the FaSTPods of the function %s: %d", funcname, len(fastpodList))
	totalRPSCap := 0.0
	nominalRPSmap := make(map[string]int64)
	for _, fastpod := range fastpodList {
		quota, _ = strconv.ParseFloat(fastpod.ObjectMeta.Annotations[fastpodv1.FaSTGShareGPUQuotaRequest], 64)
		smPartition, _ = strconv.ParseInt(fastpod.ObjectMeta.Annotations[fastpodv1.FaSTGShareGPUSMPartition], 10, 64)
		replicas := int64(*fastpod.Spec.Replicas)
		klog.Infof("Got Function: %s, Quota: %f, SMPartition: %d, Replicas: %d.", funcname, quota, smPartition, replicas)
		nominalRPSmap[getResKeyName(int64(quota*100), smPartition)] = replicas
		totalRPSCap += retrieveResource2RPSCapability(funcname, quota, smPartition) * float64(replicas)
	}
	klog.Infof("Total current processing capability of function %s is %f.", funcname, totalRPSCap)
	return nominalRPSmap, totalRPSCap
}

// schdule the most efficient resource configurations to satisfy the request workload
func (r *FaSTFuncReconciler) schedule(deltaReq float64) []*FaSTPodConfig {
	var configs []*FaSTPodConfig
	configs_eff, rps_eff := getMostEfficientConfig()
	configs_eff.Replicas = int64(deltaReq / rps_eff)
	residual_loads := math.Mod(deltaReq, rps_eff)

	// schedule the residual request loads
	if residual_loads > 0.0 {
		//should be argminp(ProcessRatei[p] − ri) where ProcessRatei[p] > ri, currently for testing, just use
		//the most efficient resource configuration
		configs_ideal, _ := getMostEfficientConfig()
		if configs_eff.Quota == configs_ideal.Quota && configs_eff.SMPartition == configs_ideal.SMPartition {
			configs_eff.Replicas += 1
		} else {
			configs = append(configs, &configs_ideal)
		}
	}

	configs = append(configs, &configs_eff)
	return configs
}

// Based on Resource Configuration and create corresponding fastpods' specification for FaSTFunc `fastfunc`
func (r *FaSTFuncReconciler) configs2FaSTPods(fastfunc *fastfuncv1.FaSTFunc, configs []*FaSTPodConfig) ([]*fastpodv1.FaSTPod, error) {
	fastpodlist := make([]*fastpodv1.FaSTPod, 0)
	for _, config := range configs {
		klog.Infof("Trying to update the FaSTPod %s with replica = %d.", getResKeyName(config.Quota, config.SMPartition), config.Replicas)
		podSpec := corev1.PodSpec{}
		selector := metav1.LabelSelector{}

		if &fastfunc.Spec != nil {
			fastfunc.Spec.PodSpec.DeepCopyInto(&podSpec)
			fastfunc.Spec.Selector.DeepCopyInto(&selector)
		}
		extendedAnnotations := make(map[string]string)
		extendedLabels := make(map[string]string)

		// write the spec
		quota := fmt.Sprintf("%0.2f", float64(config.Quota)/100.0)
		smPartition := strconv.Itoa(int(config.SMPartition))
		mem := strconv.Itoa(int(config.Mem))
		extendedLabels["com.openfaas.scale.min"] = strconv.Itoa(int(config.Replicas))
		extendedLabels["com.openfaas.scale.max"] = strconv.Itoa(int(config.Replicas))
		extendedLabels["fast_function"] = fastfunc.ObjectMeta.Name
		extendedAnnotations[fastpodv1.FaSTGShareGPUQuotaRequest] = quota
		extendedAnnotations[fastpodv1.FaSTGShareGPUQuotaLimit] = "1.0"
		extendedAnnotations[fastpodv1.FaSTGShareGPUSMPartition] = smPartition
		extendedAnnotations[fastpodv1.FaSTGShareGPUMemory] = mem
		fixedReplica_int32 := int32(config.Replicas)
		fastpod := &fastpodv1.FaSTPod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        fastfunc.ObjectMeta.Name + getResKeyName(config.Quota, config.SMPartition),
				Namespace:   "fast-gshare-fn",
				Labels:      extendedLabels,
				Annotations: extendedAnnotations,
			},
			Spec: fastpodv1.FaSTPodSpec{
				Selector: &selector,
				PodSpec:  podSpec,
				Replicas: &fixedReplica_int32,
			},
		}
		// ToDo: SetControllerReference here is useless, as the controller delete svc upon trial completion
		// Add owner reference to the service so that it could be GC
		if err := controllerutil.SetControllerReference(fastfunc, fastpod, r.Scheme); err != nil {
			klog.Info("Error setting ownerref")
			return nil, err
		}
		fastpodlist = append(fastpodlist, fastpod)
	}
	return fastpodlist, nil
}

// scaling down to update the replicas of fastpod of the FaSTFunc to be replica
func (r *FaSTFuncReconciler) scaleDownFaSTFunc(fastpodName string, replica int64) {
	existed := &fastpodv1.FaSTPod{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: fastpodName, Namespace: "fast-gshare-fn"}, existed)
	if err != nil {
		klog.Errorf("Error Failed to scaling down because of not finding the FaSTPod %s.", fastpodName)
		return
	}

	replicas := int32(replica)
	klog.Infof("Updating replicas of the FaSTPod %s to be %d.", fastpodName, replicas)

	existedCopy := existed.DeepCopy()
	existedCopy.Spec.Replicas = &replicas
	existedCopy.ObjectMeta.Labels["com.openfaas.scale.max"] = strconv.Itoa(int(replicas))
	existedCopy.ObjectMeta.Labels["com.openfaas.scale.min"] = strconv.Itoa(int(replicas))
	err = r.Update(context.TODO(), existedCopy)
	if err != nil {
		klog.Errorf("Error failed to update FaSTPod %s when scaling down", fastpodName)
		return
	}
}

// reconcile the desired replicas of the FaSTFunc
func (r *FaSTFuncReconciler) reconcileDesiredFaSTFunc(fastpods []*fastpodv1.FaSTPod) error {
	for _, fastpod := range fastpods {
		existed := &fastpodv1.FaSTPod{}
		err := r.Get(context.TODO(), types.NamespacedName{Name: fastpod.GetName(), Namespace: fastpod.GetNamespace()}, existed)
		if err != nil {
			// if the FaSTPod is not created, create the FaSTod
			if errors.IsNotFound(err) {
				klog.Infof("TO create new FaSTPod %s with replica=%d.", fastpod.GetName(), *fastpod.Spec.Replicas)
				err = r.Create(context.TODO(), fastpod)
				if err != nil {
					klog.Errorf("Error Failed to create the FaSTPod %s.", fastpod.GetName())
					return err
				}
				return nil
			} else {
				klog.Errorf("Error failed to get the fastpod %s.", fastpod.GetName())
				return err
			}
		}
		existedCopy := existed.DeepCopy()
		replicas := int32(*fastpod.Spec.Replicas)
		existedCopy.Spec.Replicas = &replicas
		klog.Infof("Updating FaSTPod %s to have replicas %d.", fastpod.GetName(), replicas)
		err = r.Update(context.TODO(), existedCopy)
		if err != nil {
			klog.Errorf("Error failed to update the replicas of the FaSTPod %s.", fastpod.GetName())
			return err
		}
	}
	return nil
}

// Initialize the FaSTPod Lister
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
		Address: "http://prometheus.fast-gshare.svc.cluster.local:9090",
	})
	if err != nil {
		klog.Error("Failed to create the Prometheus client.")
		return err
	}
	r.promv1api = promv1.NewAPI(promClient)
	client, _ := fastpodclientset.NewForConfig(ctrl.GetConfigOrDie())
	stopCh := make(chan struct{})
	r.fastpodLister = getFaSTPodLister(client, "fast-gshare-fn", stopCh)
	fastpodv1.AddToScheme(r.Scheme)

	return ctrl.NewControllerManagedBy(mgr).
		For(&fastfuncv1.FaSTFunc{}).
		Complete(r)
}
