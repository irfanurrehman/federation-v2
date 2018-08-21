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

package adapters

import (
	"fmt"

	fedv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/core/v1alpha1"
	fedclientset "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset/versioned"
	"github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

type JobSchedulingResult struct {
	Parallelism int32
	Completions int32
}

type FederatedJobAdapter struct {
	fedClient fedclientset.Interface
}

func NewFederatedJobAdapter(fedClient fedclientset.Interface) Adapter {
	return &FederatedJobAdapter{
		fedClient: fedClient,
	}
}

func (d *FederatedJobAdapter) TemplateObject() pkgruntime.Object {
	return &fedv1a1.FederatedJob{}
}

func (d *FederatedJobAdapter) TemplateList(namespace string, options metav1.ListOptions) (pkgruntime.Object, error) {
	return d.fedClient.CoreV1alpha1().FederatedJobs(namespace).List(options)
}

func (d *FederatedJobAdapter) TemplateWatch(namespace string, options metav1.ListOptions) (watch.Interface, error) {
	return d.fedClient.CoreV1alpha1().FederatedJobs(namespace).Watch(options)
}

func (d *FederatedJobAdapter) OverrideObject() pkgruntime.Object {
	return &fedv1a1.FederatedJobOverride{}
}

func (d *FederatedJobAdapter) OverrideList(namespace string, options metav1.ListOptions) (pkgruntime.Object, error) {
	return d.fedClient.CoreV1alpha1().FederatedJobOverrides(namespace).List(options)
}

func (d *FederatedJobAdapter) OverrideWatch(namespace string, options metav1.ListOptions) (watch.Interface, error) {
	return d.fedClient.CoreV1alpha1().FederatedJobOverrides(namespace).Watch(options)
}

func (d *FederatedJobAdapter) PlacementObject() pkgruntime.Object {
	return &fedv1a1.FederatedJobPlacement{}
}

func (d *FederatedJobAdapter) PlacementList(namespace string, options metav1.ListOptions) (pkgruntime.Object, error) {
	return d.fedClient.CoreV1alpha1().FederatedJobPlacements(namespace).List(options)
}

func (d *FederatedJobAdapter) PlacementWatch(namespace string, options metav1.ListOptions) (watch.Interface, error) {
	return d.fedClient.CoreV1alpha1().FederatedJobPlacements(namespace).Watch(options)
}

func (d *FederatedJobAdapter) ReconcilePlacement(fedClient fedclientset.Interface, qualifiedName util.QualifiedName, newClusterNames []string) error {
	_, err := fedClient.CoreV1alpha1().FederatedJobPlacements(qualifiedName.Namespace).Get(qualifiedName.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		newPlacement := &fedv1a1.FederatedJobPlacement{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: qualifiedName.Namespace,
				Name:      qualifiedName.Name,
			},
			Spec: fedv1a1.FederatedJobPlacementSpec{
				ClusterNames: newClusterNames,
			},
		}
		_, err := fedClient.CoreV1alpha1().FederatedJobPlacements(qualifiedName.Namespace).Create(newPlacement)
		return err
	}

	// Completions in a k8s job is immutable. As part of this first implementation,
	// the algorithm can be used to schedule a federated job, as one time distribution
	// across the federated clusters. Placement is thus not updated if its exists.
	// TODO (i@irfanurrehman), Extend the algorithm so that users can update intent
	// on a federated job.

	return nil
}

func (d *FederatedJobAdapter) ReconcileOverride(fedClient fedclientset.Interface, qualifiedName util.QualifiedName, untypedResult interface{}) error {
	result, ok := untypedResult.(map[string]JobSchedulingResult)
	if !ok {
		return fmt.Errorf("Wrong result type passed in FederatedJobAdapter for %s", qualifiedName)
	}

	_, err := fedClient.CoreV1alpha1().FederatedJobOverrides(qualifiedName.Namespace).Get(qualifiedName.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		newOverride := &fedv1a1.FederatedJobOverride{
			ObjectMeta: metav1.ObjectMeta{
				Name:      qualifiedName.Name,
				Namespace: qualifiedName.Namespace,
			},
			Spec: fedv1a1.FederatedJobOverrideSpec{},
		}

		for clusterName, jsr := range result {
			var p int32 = int32(jsr.Parallelism)
			var c int32 = int32(jsr.Completions)
			clusterOverride := fedv1a1.FederatedJobClusterOverride{
				ClusterName: clusterName,
				Parallelism: &p,
				Completions: &c,
			}
			newOverride.Spec.Overrides = append(newOverride.Spec.Overrides, clusterOverride)
		}

		_, err := fedClient.CoreV1alpha1().FederatedJobOverrides(qualifiedName.Namespace).Create(newOverride)
		return err
	}

	// Completions in a k8s job is immutable. As part of this first implementation,
	// the algorithm can be used to schedule a federated job, as one time distribution
	// across the federated clusters. Override is thus not updated if its exists.
	// TODO (i@irfanurrehman), Extend the algorithm so that users can update intent
	// on a federated job.

	return nil
}
