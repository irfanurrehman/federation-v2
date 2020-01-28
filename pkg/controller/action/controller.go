/*
Copyright 2020 The Kubernetes Authors.

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

package action

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"time"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	fedv1b1 "sigs.k8s.io/kubefed/pkg/apis/core/v1beta1"
	turbov1a1 "sigs.k8s.io/kubefed/pkg/apis/turbo/v1alpha1"
	genericclient "sigs.k8s.io/kubefed/pkg/client/generic"
	"sigs.k8s.io/kubefed/pkg/controller/util"
)

const (
	allClustersKey = "ALL_CLUSTERS"
)

// Controller manages ServiceDNSRecord resources in the host cluster.
type Controller struct {
	client genericclient.Client

	// For triggering reconciliation of all target resources. This is
	// used when a new cluster becomes available.
	clusterDeliverer *util.DelayingDeliverer

	// informer for pods (and in turn containers) in member clusters
	podInformer util.FederatedInformer

	// Store for the Action objects
	actionStore cache.Store
	// Informer for the Action objects
	actionController cache.Controller

	worker util.ReconcileWorker

	clusterAvailableDelay   time.Duration
	clusterUnavailableDelay time.Duration
	smallDelay              time.Duration

	fedNamespace string
}

// StartController starts the Controller for managing ServiceDNSRecord objects.
func StartController(config *util.ControllerConfig, stopChan <-chan struct{}) error {
	controller, err := newController(config)
	if err != nil {
		return err
	}
	if config.MinimizeLatency {
		controller.minimizeLatency()
	}
	klog.Infof("Starting ServiceDNS controller")
	controller.Run(stopChan)
	return nil
}

// newController returns a new controller to manage ServiceDNSRecord objects.
func newController(config *util.ControllerConfig) (*Controller, error) {
	client := genericclient.NewForConfigOrDieWithUserAgent(config.KubeConfig, "Action")
	s := &Controller{
		client:                  client,
		clusterAvailableDelay:   config.ClusterAvailableDelay,
		clusterUnavailableDelay: config.ClusterUnavailableDelay,
		smallDelay:              time.Second * 3,
		fedNamespace:            config.KubeFedNamespace,
	}

	s.worker = util.NewReconcileWorker(s.reconcile, util.WorkerTiming{
		ClusterSyncDelay: s.clusterAvailableDelay,
	})

	// Build deliverer for triggering cluster reconciliations.
	s.clusterDeliverer = util.NewDelayingDeliverer()

	// Informer for Actions in the host cluster
	var err error
	s.actionStore, s.actionController, err = util.NewGenericInformer(
		config.KubeConfig,
		config.TargetNamespace,
		&turbov1a1.Action{},
		util.NoResyncPeriod,
		s.worker.EnqueueObject,
	)
	if err != nil {
		return nil, err
	}

	// Federated informer for pods in member clusters
	// This will also get clients for all member clusters
	s.podInformer, err = util.NewFederatedInformer(
		config,
		client,
		&metav1.APIResource{
			Group:        "",
			Version:      "v1",
			Kind:         "Pod",
			Name:         "pods",
			SingularName: "pod",
			Namespaced:   true},
		s.worker.EnqueueObject,
		&util.ClusterLifecycleHandlerFuncs{
			ClusterAvailable: func(cluster *fedv1b1.KubeFedCluster) {
				// When new cluster becomes available process all the target resources again.
				s.clusterDeliverer.DeliverAt(allClustersKey, nil, time.Now().Add(s.clusterAvailableDelay))
			},
			// When a cluster becomes unavailable process all the target resources again.
			ClusterUnavailable: func(cluster *fedv1b1.KubeFedCluster, _ []interface{}) {
				s.clusterDeliverer.DeliverAt(allClustersKey, nil, time.Now().Add(s.clusterUnavailableDelay))
			},
		},
	)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// minimizeLatency reduces delays and timeouts to make the controller more responsive (useful for testing).
func (c *Controller) minimizeLatency() {
	c.clusterAvailableDelay = time.Second
	c.clusterUnavailableDelay = time.Second
	c.smallDelay = 20 * time.Millisecond
	c.worker.SetDelay(50*time.Millisecond, c.clusterAvailableDelay)
}

// Run runs the Controller.
func (c *Controller) Run(stopChan <-chan struct{}) {
	go c.actionController.Run(stopChan)
	c.podInformer.Start()
	c.clusterDeliverer.StartWithHandler(func(_ *util.DelayingDelivererItem) {
		c.reconcileOnClusterChange()
	})

	c.worker.Run(stopChan)

	// Ensure all goroutines are cleaned up when the stop channel closes
	go func() {
		<-stopChan
		c.podInformer.Stop()
		c.clusterDeliverer.Stop()
	}()
}

// Check whether all data stores are in sync. False is returned if any of the serviceInformer/stores is not yet
// synced with the corresponding api server.
func (c *Controller) isSynced() bool {
	if !c.podInformer.ClustersSynced() {
		klog.V(2).Infof("Cluster list not synced")
		return false
	}
	clusters, err := c.podInformer.GetReadyClusters()
	if err != nil {
		utilruntime.HandleError(errors.Wrap(err, "Failed to get ready clusters"))
		return false
	}
	if !c.podInformer.GetTargetStore().ClustersSynced(clusters) {
		return false
	}

	return true
}

// The function triggers reconciliation of all target federated resources.
func (c *Controller) reconcileOnClusterChange() {
	if !c.isSynced() {
		c.clusterDeliverer.DeliverAt(allClustersKey, nil, time.Now().Add(c.clusterAvailableDelay))
	}
	for _, obj := range c.actionStore.List() {
		qualifiedName := util.NewQualifiedName(obj.(pkgruntime.Object))
		c.worker.EnqueueWithDelay(qualifiedName, c.smallDelay)
	}
}

func (c *Controller) reconcile(qualifiedName util.QualifiedName) util.ReconciliationStatus {
	if !c.isSynced() {
		return util.StatusNotSynced
	}

	key := qualifiedName.String()

	klog.V(1).Infof("Starting to reconcile Action resource: %v", key)
	startTime := time.Now()
	defer func() {
		klog.V(1).Infof("Finished reconciling Action resource %v (duration: %v)", key, time.Since(startTime))
	}()

	cachedObj, exist, err := c.actionStore.GetByKey(key)
	if err != nil {
		klog.Error(errors.Wrapf(err, "Failed to query Action store for %q", key))
		return util.StatusError
	}
	if !exist {
		return util.StatusAllOK
	}
	action := cachedObj.(*turbov1a1.Action)

	klog.V(1).Infof("got action action:%q as %v", key, action)
	// IRF: Find out if this is a newly created action
	//     is status.condition == "" or "unknown"
	//     if so this is a new action set condition to "executing"
	//     process the action
	// get source pod
	// TODO: build multiple stateful conditions to this action
	// For example, targetCreated, sourceDeleted to cleanup incomplete actions
	// A more resilient system would be pull based and that action describes source resource(s)
	// and target resource(s) and individual agents (aka kubeturbo) in individual
	// clusters executes these actions. The actions store desired state and
	// separate sub states, to be achieved/updated by individual agents.

	srcCluster := action.Spec.Clusters.Source
	dstCluster := action.Spec.Clusters.Destination
	if srcCluster == "" || dstCluster == "" {
		klog.Error("Missing source or destination cluster name src: %s, dst: %s, %v", srcCluster, dstCluster, err)
		// TODO: update action status and return all-ok for reconcile because we cannot do anything for this action
		return util.StatusError
	}

	srcClient, err := c.podInformer.GetDynamicClientForCluster(srcCluster)
	if err != nil {
		klog.Error("Could not get client for destination cluster: %s, %v", srcCluster, err)
		return util.StatusError
	}

	dstClient, err := c.podInformer.GetDynamicClientForCluster(dstCluster)
	if err != nil {
		klog.Error("Could not get client for destination cluster: %s, %v", srcCluster, err)
		return util.StatusError
	}

	// 1. Get source pod and its parent
	// 2. See if we have a parent at destination
	// 3a. If we have a parent at the destination, create copy pod at destination assigned to a node
	//    assign this pod to parent and increase the replica count on parent
	// 3b. If we do not have a parent at destination, create the pod assigned to a given node
	//    create a parent with replica 1 to adopt this pod.
	// 4. Delete the source pod and reduce replica of source parent using part of the movePod utils

	srcPodKey := fmt.Sprintf("%s/%s", action.Spec.TargetRef.Namespace, action.Spec.TargetRef.Name)
	res := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	srcPod, err := srcClient.Resource(res).Namespace(action.Spec.TargetRef.Namespace).Get(action.Spec.TargetRef.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Could not convert unstructured pod: %s to a typed structure, %v", srcPodKey, err)
		return util.StatusError
	}

	pod := &corev1.Pod{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(srcPod.Object, pod); err != nil {
		klog.Errorf("Could not convert unstructured pod: %s to a typed structure, %v \n %v", srcPodKey, err, srcPod)
		return util.StatusError
	}

	executor, err := newActionExecutor(srcClient, dstClient, pod, action.Spec.TargetNode)
	if err != nil {
		klog.Errorf("Error initiating action executor for action %q: %v ", key, err)
		return util.StatusError
	}

	err = executor.executeAction()
	if err != nil {
		klog.Errorf("Error executing action %q: %v ", key, err)
		return util.StatusError
	}

	// TODO: update action status after successful completion
	return util.StatusAllOK
}
