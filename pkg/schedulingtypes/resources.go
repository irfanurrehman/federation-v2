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
	"reflect"
	"strings"

	fedv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/core/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
)

var (
	FederatedDeployment = GetResourceKind(&fedv1a1.FederatedDeployment{})
	Deployment          = GetResourceKind(&appsv1.Deployment{})
	FederatedReplicaSet = GetResourceKind(&fedv1a1.FederatedReplicaSet{})
	ReplicaSet          = GetResourceKind(&appsv1.ReplicaSet{})
	Pod                 = GetResourceKind(&corev1.Pod{})
	Job                 = GetResourceKind(&batchv1.Job{})
)

var PodResource = &metav1.APIResource{
	Name:       GetPluralName(Pod),
	Group:      corev1.SchemeGroupVersion.Group,
	Version:    corev1.SchemeGroupVersion.Version,
	Kind:       Pod,
	Namespaced: true,
}

var JobResource = &metav1.APIResource{
	Name:       GetPluralName(Job),
	Group:      batchv1.SchemeGroupVersion.Group,
	Version:    batchv1.SchemeGroupVersion.Version,
	Kind:       Job,
	Namespaced: true,
}

// The resource name key is the corresponding federated resource name
// for the given k8s resource definition.
var ReplicaSechedulingResources = map[string]metav1.APIResource{
	FederatedDeployment: {
		Name:       GetPluralName(Deployment),
		Group:      appsv1.SchemeGroupVersion.Group,
		Version:    appsv1.SchemeGroupVersion.Version,
		Kind:       Deployment,
		Namespaced: true,
	},
	FederatedReplicaSet: {
		Name:       GetPluralName(ReplicaSet),
		Group:      appsv1.SchemeGroupVersion.Group,
		Version:    appsv1.SchemeGroupVersion.Version,
		Kind:       ReplicaSet,
		Namespaced: true,
	},
}

func GetResourceKind(obj pkgruntime.Object) string {
	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Ptr {
		panic("All types must be pointers to structs.")
	}

	t = t.Elem()
	return t.Name()
}

func GetPluralName(name string) string {
	return strings.ToLower(name) + "s"
}
