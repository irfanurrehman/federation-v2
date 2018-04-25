
/*
Copyright 2018 The Federation v2 Authors.

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


package replicaschedulingpreference

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	appsv1 "k8s.io/api/apps/v1"
	fedv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/federation/v1alpha1"
	fedschedulingv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/federatedscheduling/v1alpha1"
	fedclientset "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset_generated/clientset"
	"k8s.io/apimachinery/pkg/util/runtime"
	"github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
	"github.com/kubernetes-sigs/federation-v2/pkg/federatedtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/client-go/util/workqueue"
	crclientset "k8s.io/cluster-registry/pkg/client/clientset_generated/clientset"
)

const (
	allClustersKey = "ALL_CLUSTERS"
)

// ReplicaSchedulingPreferenceController synchronizes the state of a federated type
// to clusters that are members of the federation.
type ReplicaSchedulingPreferenceController struct {
	// For triggering reconciliation of a single resource. This is
	// used when there is an add/update/delete operation on a resource
	// in either federated API server or in some member of the
	// federation.
	deliverer *util.DelayingDeliverer

	// For triggering reconciliation of all target resources. This is
	// used when a new cluster becomes available.
	clusterDeliverer *util.DelayingDeliverer

	// Contains resources present in members of federation.
	informer util.FederatedInformer

	// Store for type self
	store cache.Store
	// Informer for type self
	controller cache.Controller

	// Store for the templates of the federated type
	templateStore cache.Store
	// Informer for the templates of the federated type
	templateController cache.Controller

	// Store for the override directives of the federated type
	overrideStore cache.Store
	// Informer controller for override directives of the federated type
	overrideController cache.Controller

	// Store for the override directives of the federated type
	placementStore cache.Store
	// Informer controller for override directives of the federated type
	placementController cache.Controller

	// Work queue allowing parallel processing of resources
	workQueue workqueue.Interface

	// Backoff manager
	backoff *flowcontrol.Backoff

	// For events
	eventRecorder record.EventRecorder

	reviewDelay             time.Duration
	clusterAvailableDelay   time.Duration
	clusterUnavailableDelay time.Duration
	smallDelay              time.Duration
	updateTimeout           time.Duration
}

// ReplicaSchedulingPreferenceController starts a new controller for ReplicaSchedulingPreferences
func StartReplicaSchedulingPreferenceController(fedConfig, kubeConfig, crConfig *restclient.Config, stopChan <-chan struct{}, minimizeLatency bool) error {
	userAgent := "replicaschedulingpreference-controller"
	restclient.AddUserAgent(fedConfig, userAgent)
	fedClient := fedclientset.NewForConfigOrDie(fedConfig)
	restclient.AddUserAgent(kubeConfig, userAgent)
	kubeClient := kubeclientset.NewForConfigOrDie(kubeConfig)
	restclient.AddUserAgent(crConfig, userAgent)
	crClient := crclientset.NewForConfigOrDie(crConfig)
	controller, err := newReplicaSchedulingPreferenceController(fedConfig, fedClient, kubeClient, crClient)
	if err != nil {
		return err
	}
	if minimizeLatency {
		controller.minimizeLatency()
	}
	glog.Infof(fmt.Sprintf("Starting replicaschedulingpreferences controller"))
	controller.Run(stopChan)
	return nil
}
/* TODO
func newFedInformer(fedClient fedclientset.Interface, objectType pkgruntime.Object, typeInterface interface{}, triggerFunc func(pkgruntime.Object)) (cache.Store, cache.Controller) {
	return cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (pkgruntime.Object, error) {
				return typeAdapter.List(metav1.NamespaceAll, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return typeAdapter.Watch(metav1.NamespaceAll, options)
			},
		},
		objectType,
		util.NoResyncPeriod,
		util.NewTriggerOnAllChanges(triggerFunc),
	)
}
*/
// newFederationSyncController returns a new sync controller for the given client and type adapter
func newReplicaSchedulingPreferenceController(fedConfig *restclient.Config, fedClient fedclientset.Interface, kubeClient kubeclientset.Interface, crClient crclientset.Interface) (*ReplicaSchedulingPreferenceController, error) {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: fmt.Sprintf("replicaschedulingpreference-controller")})

	s := &ReplicaSchedulingPreferenceController{
		reviewDelay:             time.Second * 10,
		clusterAvailableDelay:   time.Second * 20,
		clusterUnavailableDelay: time.Second * 60,
		smallDelay:              time.Second * 3,
		updateTimeout:           time.Second * 30,
		workQueue:               workqueue.New(),
		backoff:                 flowcontrol.NewBackOff(5*time.Second, time.Minute),
		eventRecorder:           recorder,
	}

	// Build delivereres for triggering reconciliations.
	s.deliverer = util.NewDelayingDeliverer()
	s.clusterDeliverer = util.NewDelayingDeliverer()

	// Start informers on the resources for the federated type
	deliverObj := func(obj pkgruntime.Object) {
		s.deliverObj(obj, 0, false)
	}

	s.templateStore, s.templateController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (pkgruntime.Object, error) {
				return fedClient.FederatedschedulingV1alpha1().ReplicaSchedulingPreferences(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return fedClient.FederatedschedulingV1alpha1().ReplicaSchedulingPreferences(metav1.NamespaceAll).Watch(options)
			},
		},
		&fedschedulingv1a1.ReplicaSchedulingPreference{},
		util.NoResyncPeriod,
		util.NewTriggerOnAllChanges(deliverObj),
	)

	s.templateStore, s.templateController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (pkgruntime.Object, error) {
				return fedClient.FederationV1alpha1().FederatedDeployments(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return fedClient.FederationV1alpha1().FederatedDeployments(metav1.NamespaceAll).Watch(options)
			},
		},
		&fedv1a1.FederatedDeployment{},
		util.NoResyncPeriod,
		util.NewTriggerOnAllChanges(deliverObj),
	)

	s.overrideStore, s.overrideController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (pkgruntime.Object, error) {
				return fedClient.FederationV1alpha1().FederatedDeploymentOverrides(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return fedClient.FederationV1alpha1().FederatedDeploymentOverrides(metav1.NamespaceAll).Watch(options)
			},
		},
		&fedv1a1.FederatedDeploymentOverride{},
		util.NoResyncPeriod,
		util.NewTriggerOnAllChanges(deliverObj),
	)

	s.placementStore, s.placementController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (pkgruntime.Object, error) {
				return fedClient.FederationV1alpha1().FederatedDeploymentPlacements(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return fedClient.FederationV1alpha1().FederatedDeploymentPlacements(metav1.NamespaceAll).Watch(options)
			},
		},
		&fedv1a1.FederatedDeploymentPlacement{},
		util.NoResyncPeriod,
		util.NewTriggerOnAllChanges(deliverObj),
	)

	// Federated informer on the resource type in members of federation.
	s.informer = util.NewFederatedInformer(
		fedClient,
		kubeClient,
		crClient,
		func(cluster *fedv1a1.FederatedCluster, targetClient kubeclientset.Interface) (cache.Store, cache.Controller) {
			return cache.NewInformer(
				&cache.ListWatch{
					ListFunc: func(options metav1.ListOptions) (pkgruntime.Object, error) {
						return kubeClient.AppsV1().Deployments(metav1.NamespaceAll).List(options)
					},
					WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
						return kubeClient.AppsV1().Deployments(metav1.NamespaceAll).Watch(options)
					},
				},
				&appsv1.Deployment{},
				util.NoResyncPeriod,
				// Trigger reconciliation whenever something in federated cluster is changed. In most cases it
				// would be just confirmation that some operation on the target resource type had succeeded.
				util.NewTriggerOnAllChanges(
					func(obj pkgruntime.Object) {
						s.deliverObj(obj, s.reviewDelay, false)
					},
				))
		},

		&util.ClusterLifecycleHandlerFuncs{
			ClusterAvailable: func(cluster *fedv1a1.FederatedCluster) {
				// When new cluster becomes available process all the target resources again.
				s.clusterDeliverer.DeliverAt(allClustersKey, nil, time.Now().Add(s.clusterAvailableDelay))
			},
			// When a cluster becomes unavailable process all the target resources again.
			ClusterUnavailable: func(cluster *fedv1a1.FederatedCluster, _ []interface{}) {
				s.clusterDeliverer.DeliverAt(allClustersKey, nil, time.Now().Add(s.clusterUnavailableDelay))
			},
		},
	)

	return s, nil
}

// minimizeLatency reduces delays and timeouts to make the controller more responsive (useful for testing).
func (s *ReplicaSchedulingPreferenceController) minimizeLatency() {
	s.clusterAvailableDelay = time.Second
	s.clusterUnavailableDelay = time.Second
	s.reviewDelay = 50 * time.Millisecond
	s.smallDelay = 20 * time.Millisecond
	s.updateTimeout = 5 * time.Second
}

func (s *ReplicaSchedulingPreferenceController) Run(stopChan <-chan struct{}) {
	go s.templateController.Run(stopChan)
	go s.overrideController.Run(stopChan)
	go s.placementController.Run(stopChan)

	s.informer.Start()
	s.deliverer.StartWithHandler(func(item *util.DelayingDelivererItem) {
		s.workQueue.Add(item)
	})
	s.clusterDeliverer.StartWithHandler(func(_ *util.DelayingDelivererItem) {
		s.reconcileOnClusterChange()
	})

	go wait.Until(s.worker, time.Second, stopChan)

	util.StartBackoffGC(s.backoff, stopChan)

	// Ensure all goroutines are cleaned up when the stop channel closes
	go func() {
		<-stopChan
		s.informer.Stop()
		s.workQueue.ShutDown()
		s.deliverer.Stop()
		s.clusterDeliverer.Stop()
	}()
}

type reconciliationStatus int

const (
	statusAllOK reconciliationStatus = iota
	statusNeedsRecheck
	statusError
	statusNotSynced
)

func (s *ReplicaSchedulingPreferenceController) worker() {
	for {
		obj, quit := s.workQueue.Get()
		if quit {
			return
		}

		item := obj.(*util.DelayingDelivererItem)
		qualifiedName := item.Value.(*federatedtypes.QualifiedName)
		status := s.reconcile(*qualifiedName)
		s.workQueue.Done(item)

		switch status {
		case statusAllOK:
			break
		case statusError:
			s.deliver(*qualifiedName, 0, true)
		case statusNeedsRecheck:
			s.deliver(*qualifiedName, s.reviewDelay, false)
		case statusNotSynced:
			s.deliver(*qualifiedName, s.clusterAvailableDelay, false)
		}
	}
}

func (s *ReplicaSchedulingPreferenceController) deliverObj(obj pkgruntime.Object, delay time.Duration, failed bool) {
	qualifiedName := federatedtypes.NewQualifiedName(obj)
	s.deliver(qualifiedName, delay, failed)
}

// Adds backoff to delay if this delivery is related to some failure. Resets backoff if there was no failure.
func (s *ReplicaSchedulingPreferenceController) deliver(qualifiedName federatedtypes.QualifiedName, delay time.Duration, failed bool) {
	key := qualifiedName.String()
	if failed {
		s.backoff.Next(key, time.Now())
		delay = delay + s.backoff.Get(key)
	} else {
		s.backoff.Reset(key)
	}
	s.deliverer.DeliverAfter(key, &qualifiedName, delay)
}


// Check whether all data stores are in sync. False is returned if any of the informer/stores is not yet
// synced with the corresponding api server.
func (s *ReplicaSchedulingPreferenceController) isSynced() bool {
	if !s.informer.ClustersSynced() {
		glog.V(2).Infof("Cluster list not synced")
		return false
	}

	clusters, err := s.informer.GetReadyClusters()
	if err != nil {
		runtime.HandleError(fmt.Errorf("Failed to get ready clusters: %v", err))
		return false
	}
	if !s.informer.GetTargetStore().ClustersSynced(clusters) {
		return false
	}
	return true
}

// The function triggers reconciliation of all target federated resources.
func (s *ReplicaSchedulingPreferenceController) reconcileOnClusterChange() {
	if !s.isSynced() {
		s.clusterDeliverer.DeliverAt(allClustersKey, nil, time.Now().Add(s.clusterAvailableDelay))
	}
	for _, obj := range s.templateStore.List() {
		qualifiedName := federatedtypes.NewQualifiedName(obj.(pkgruntime.Object))
		s.deliver(qualifiedName, s.smallDelay, false)
	}
}

func (s *ReplicaSchedulingPreferenceController) reconcile(qualifiedName federatedtypes.QualifiedName) reconciliationStatus {
	if !s.isSynced() {
		return statusNotSynced
	}

	kind := "ReplicaSchedulingPreference"
	key := qualifiedName.String()
	//namespace := qualifiedName.Namespace

	glog.V(4).Infof("Starting to reconcile %v %v", kind, key)
	startTime := time.Now()
	defer glog.V(4).Infof("Finished reconciling %v %v (duration: %v)", kind, key, time.Now().Sub(startTime))

	rsp, err := s.objFromCache(s.store, kind, key)
	if err != nil {
		return statusError
	}
	if rsp == nil {
		return statusAllOK
	}

/* TODO delete
	meta := templateAdapter.ObjectMeta(template)
	if meta.DeletionTimestamp != nil {
		err := s.delete(template, meta, kind, qualifiedName)
		if err != nil {
			msg := "Failed to delete %s %q: %v"
			args := []interface{}{kind, qualifiedName, err}
			runtime.HandleError(fmt.Errorf(msg, args...))
			s.eventRecorder.Eventf(template, corev1.EventTypeWarning, "DeleteFailed", msg, args...)
			return statusError
		}
		return statusAllOK
	}

	glog.V(3).Infof("Ensuring finalizers exist on %s %q", kind, key)
	template, err = s.deletionHelper.EnsureFinalizers(template)
	if err != nil {
		runtime.HandleError(fmt.Errorf("Failed to ensure finalizers for %s %q: %v", kind, key, err))
		return statusError
	}
*/
	clusterNames, err := s.clusterNames()
	if err != nil {
		runtime.HandleError(fmt.Errorf("Failed to get cluster list: %v", err))
		return statusNotSynced
	}

	glog.Infof(fmt.Sprintf("The available clusters listed are %v", clusterNames))

/*	selectedClusters, unselectedClusters, err := s.placementPlugin.ComputePlacement(key, clusterNames)
	if err != nil {
		runtime.HandleError(fmt.Errorf("Failed to compute placement for %s %q: %v", kind, key, err))
		return statusError
	}
*/
	template, err := s.objFromCache(s.templateStore, "FederatedDeployment", key)
		if err != nil {
			return statusError
	}

	glog.Infof(fmt.Sprintf("The available template listed are %v", template))


	return statusAllOK
}

func (s *ReplicaSchedulingPreferenceController) objFromCache(store cache.Store, kind, key string) (pkgruntime.Object, error) {
	cachedObj, exist, err := store.GetByKey(key)
	if err != nil {
		wrappedErr := fmt.Errorf("Failed to query %s store for %q: %v", kind, key, err)
		runtime.HandleError(wrappedErr)
		return nil, err
	}
	if !exist {
		return nil, nil
	}
	return cachedObj.(pkgruntime.Object).DeepCopyObject(), nil
}


func (s *ReplicaSchedulingPreferenceController) clusterNames() ([]string, error) {
	clusters, err := s.informer.GetReadyClusters()
	if err != nil {
		return nil, err
	}
	clusterNames := []string{}
	for _, cluster := range clusters {
		clusterNames = append(clusterNames, cluster.Name)
	}

	return clusterNames, nil
}