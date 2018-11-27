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
	"github.com/kubernetes-sigs/federation-v2/pkg/apis/core/typeconfig"
	. "github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

type Scheduler interface {
	Kind() string
	ObjectType() pkgruntime.Object
	FedList(namespace string, options metav1.ListOptions) (pkgruntime.Object, error)
	FedWatch(namespace string, options metav1.ListOptions) (watch.Interface, error)

	Start(stopChan <-chan struct{})
	HasSynced() bool
	Stop()
	Reconcile(obj pkgruntime.Object, qualifiedName QualifiedName) ReconciliationStatus

	StartPlugin(typeConfig typeconfig.Interface, stopChan <-chan struct{}) error
}

type ScheduleResult interface {
	PlacementUpdateNeeded(names []string) bool
	OverrideUpdateNeeded(typeConfig typeconfig.Interface, obj *unstructured.Unstructured) bool
	SetPlacementSpec(obj *unstructured.Unstructured)
	SetOverrideSpec(obj *unstructured.Unstructured)
}

type SchedulerEventHandlers struct {
	FederationEventHandler   func(pkgruntime.Object)
	ClusterEventHandler      func(pkgruntime.Object)
	ClusterLifecycleHandlers *ClusterLifecycleHandlerFuncs
}

type SchedulerFactory func(controllerConfig *ControllerConfig, eventHandlers SchedulerEventHandlers) (Scheduler, error)
