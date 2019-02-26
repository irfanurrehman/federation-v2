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

package common

import (
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"

	"github.com/kubernetes-sigs/federation-v2/pkg/apis/core/typeconfig"
	"github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
	"github.com/kubernetes-sigs/federation-v2/pkg/kubefed2/federate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NewTestObject(typeConfig typeconfig.Interface, namespace string, clusterNames []string, fixture *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	obj := newTestUnstructured(typeConfig.GetFederatedType(), namespace)

	template, ok, err := unstructured.NestedFieldCopy(fixture.Object, util.TemplateField)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving template field")
	}
	if ok {
		err := unstructured.SetNestedField(obj.Object, template, util.SpecField, util.TemplateField)
		if err != nil {
			return nil, err
		}
	}

	overrides, ok, err := OverridesFromFixture(clusterNames, fixture)
	if err != nil {
		return nil, err
	}
	if ok {
		err = unstructured.SetNestedSlice(obj.Object, overrides, util.SpecField, util.OverridesField)
		if err != nil {
			return nil, err
		}
	}

	err = util.SetClusterNames(obj, clusterNames)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func OverridesFromFixture(clusterNames []string, fixture *unstructured.Unstructured) ([]interface{}, bool, error) {
	overridesSlice, ok, err := unstructured.NestedSlice(fixture.Object, util.OverridesField)
	if err != nil {
		return nil, false, errors.Wrap(err, "Error retrieving overrides field")
	}
	if ok && len(clusterNames) > 0 {
		targetOverrides := overridesSlice[0].(map[string]interface{})
		targetOverrides[util.ClusterNameField] = clusterNames[0]
		overridesSlice[0] = targetOverrides
		return overridesSlice, true, nil
	}
	return nil, false, nil
}

func NewTestTargetObject(typeConfig typeconfig.Interface, namespace string, fixture *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	template, ok, err := unstructured.NestedFieldCopy(fixture.Object, util.TemplateField)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving template field")
	}
	if ok {
		obj.Object = template.(map[string]interface{})
	}

	federate.SetBasicMetaFields(obj, typeConfig.GetTarget(), "", namespace, "test-e2e-")
	return obj, nil
}

func newTestUnstructured(apiResource metav1.APIResource, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetKind(apiResource.Kind)
	gv := schema.GroupVersion{Group: apiResource.Group, Version: apiResource.Version}
	obj.SetAPIVersion(gv.String())
	obj.SetGenerateName("test-e2e-")
	if apiResource.Namespaced {
		obj.SetNamespace(namespace)
	}
	return obj
}

func fixturePath() string {
	// Get the directory of the current executable
	_, filename, _, _ := runtime.Caller(0)
	commonPath := filepath.Dir(filename)
	return filepath.Join(commonPath, "fixtures")
}
