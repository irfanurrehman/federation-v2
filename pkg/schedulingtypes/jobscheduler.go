/*
Copyright 2018 The Kubernetes Authors.

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

package schedulingtypes

import (
	"fmt"

	fedschedulingv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/scheduling/v1alpha1"
	fedclientset "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset/versioned"
	"github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
	. "github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
	"github.com/kubernetes-sigs/federation-v2/pkg/controller/util/planner"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	kubeclientset "k8s.io/client-go/kubernetes"
	crclientset "k8s.io/cluster-registry/pkg/client/clientset/versioned"

	"github.com/golang/glog"
	"github.com/kubernetes-sigs/federation-v2/pkg/schedulingtypes/adapters"
)

const (
	JobSchedulingKind = "JobSchedulingPreference"
)

func init() {
	RegisterSchedulingType(JobSchedulingKind, NewJobScheduler)
}

type JobScheduler struct {
	plugin    *Plugin
	fedClient fedclientset.Interface
}

func NewJobScheduler(fedClient fedclientset.Interface, kubeClient kubeclientset.Interface, crClient crclientset.Interface, fedNamespace, clusterNamespace, targetNamespace string, federationEventHandler, clusterEventHandler func(pkgruntime.Object), handlers *ClusterLifecycleHandlerFuncs) Scheduler {
	scheduler := &JobScheduler{}
	scheduler.fedClient = fedClient
	adapter := adapters.NewFederatedJobAdapter(fedClient)
	tc, err := fedClient.CoreV1alpha1().FederatedTypeConfigs(util.DefaultFederationSystemNamespace).Get(JobTypeName, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Failed to get APIResource for %s: %v", JobTypeName, err)
	}
	apiResource := tc.GetTarget()
	scheduler.plugin = NewPlugin(
		adapter,
		&apiResource,
		fedClient,
		kubeClient,
		crClient,
		fedNamespace,
		clusterNamespace,
		targetNamespace,
		federationEventHandler,
		clusterEventHandler,
		handlers,
	)

	return scheduler
}

func (j *JobScheduler) Kind() string {
	return RSPKind
}

func (j *JobScheduler) ObjectType() pkgruntime.Object {
	return &fedschedulingv1a1.JobSchedulingPreference{}
}

func (j *JobScheduler) FedList(namespace string, options metav1.ListOptions) (pkgruntime.Object, error) {
	return j.fedClient.SchedulingV1alpha1().JobSchedulingPreferences(metav1.NamespaceAll).List(options)
}

func (j *JobScheduler) FedWatch(namespace string, options metav1.ListOptions) (watch.Interface, error) {
	return j.fedClient.SchedulingV1alpha1().JobSchedulingPreferences(metav1.NamespaceAll).Watch(options)
}

func (j *JobScheduler) Start(stopChan <-chan struct{}) {
	j.plugin.Start(stopChan)
}

func (j *JobScheduler) HasSynced() bool {
	return j.plugin.HasSynced()
}

func (j *JobScheduler) Stop() {
	j.plugin.Stop()
}

func (j *JobScheduler) Reconcile(obj pkgruntime.Object, qualifiedName QualifiedName) ReconciliationStatus {
	jsp, ok := obj.(*fedschedulingv1a1.JobSchedulingPreference)
	if !ok {
		runtime.HandleError(fmt.Errorf("Incorrect runtime object for JSP: %v", jsp))
		return StatusError
	}

	clusterNames, err := j.plugin.ReadyClusterNames()
	if err != nil {
		runtime.HandleError(fmt.Errorf("Failed to get cluster list: %v", err))
		return StatusError
	}
	if len(clusterNames) == 0 {
		// no joined clusters, nothing to do
		return StatusAllOK
	}

	result := j.schedule(jsp, qualifiedName, clusterNames)
	err = j.ReconcileFederationTargets(j.fedClient, qualifiedName, result)
	if err != nil {
		runtime.HandleError(fmt.Errorf("Failed to reconcile Federation Targets for JSP named %s: %v", qualifiedName, err))
		return StatusError
	}

	return StatusAllOK
}

func (j *JobScheduler) ReconcileFederationTargets(fedClient fedclientset.Interface, qualifiedName QualifiedName, result map[string]adapters.JobSchedulingResult) error {
	newClusterNames := []string{}
	for name := range result {
		newClusterNames = append(newClusterNames, name)
	}

	err := j.plugin.adapter.ReconcilePlacement(fedClient, qualifiedName, newClusterNames)
	if err != nil {
		return err
	}

	err = j.plugin.adapter.ReconcileOverride(fedClient, qualifiedName, result)
	if err != nil {
		return err
	}

	return nil
}

func (j *JobScheduler) schedule(jsp *fedschedulingv1a1.JobSchedulingPreference, qualifiedName QualifiedName, clusterNames []string) map[string]adapters.JobSchedulingResult {
	key := qualifiedName.String()
	if len(jsp.Spec.ClusterWeights) == 0 {
		jsp.Spec.ClusterWeights = map[string]int32{
			"*": 1,
		}
	}

	plnr := planner.NewJobPlanner(jsp)
	parallelismResult := plnr.Plan(clusterNames, jsp.Spec.TotalParallelism, key)

	clusterNames = nil
	for clusterName := range parallelismResult {
		clusterNames = append(clusterNames, clusterName)
	}
	completionsResult := plnr.Plan(clusterNames, jsp.Spec.TotalCompletions, key)

	result := make(map[string]adapters.JobSchedulingResult)
	for _, clusterName := range clusterNames {
		result[clusterName] = adapters.JobSchedulingResult{
			Parallelism: parallelismResult[clusterName],
			Completions: completionsResult[clusterName],
		}
	}

	return result
}
