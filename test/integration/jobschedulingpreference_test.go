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

package integration

import (
	"testing"

	fedv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/core/v1alpha1"
	fedschedulingv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/scheduling/v1alpha1"
	clientset "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset/versioned"
	"github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
	"github.com/kubernetes-sigs/federation-v2/pkg/schedulingtypes"
	"github.com/kubernetes-sigs/federation-v2/pkg/schedulingtypes/adapters"
	"github.com/kubernetes-sigs/federation-v2/test/common"
	"github.com/kubernetes-sigs/federation-v2/test/integration/framework"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"
)

// TestReplicaSchedulingPreference validates basic replica scheduling preference calculations.
var TestJobSchedulingPreference = func(t *testing.T) {
	t.Parallel()
	tl := framework.NewIntegrationLogger(t)

	controllerFixture, fedClient := initJSPTest(tl, FedFixture)
	defer controllerFixture.TearDown(tl)

	clusters := FedFixture.ClusterNames()
	if len(clusters) != 2 {
		tl.Fatalf("Expected two clusters to be part of Federation Fixture setup")
	}

	kubeClient := FedFixture.KubeApi.NewClient(tl, "jsp")
	testNamespace := framework.CreateTestNamespace(tl, kubeClient, "jsp-")

	testCases := map[string]struct {
		jspSpec  fedschedulingv1a1.JobSchedulingPreferenceSpec
		expected map[string]adapters.JobSchedulingResult
	}{
		"Distribution happens evenly across all clusters": {
			jspSpec: jspSpecWithoutClusterWeights(int32(4), int32(4)),
			expected: map[string]adapters.JobSchedulingResult{
				clusters[0]: {
					Parallelism: int32(2),
					Completions: int32(2),
				},
				clusters[1]: {
					Parallelism: int32(2),
					Completions: int32(2),
				},
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			name, err := createJSPTestObjs(testCase.jspSpec, testNamespace, fedClient)
			if err != nil {
				tl.Fatalf("Creation of test objects failed in federation")
			}

			err = waitForMatchingJobPlacement(fedClient, name, testNamespace, testCase.expected)
			if err != nil {
				tl.Fatalf("Failed waiting for matching placements")
			}

			err = waitForMatchingJobOverride(fedClient, name, testNamespace, testCase.expected)
			if err != nil {
				tl.Fatalf("Failed waiting for matching overrides")
			}
		})
	}
}

func initJSPTest(tl common.TestLogger, fedFixture *framework.FederationFixture) (*framework.ControllerFixture, clientset.Interface) {
	config := fedFixture.KubeApi.NewConfig(tl)
	restclient.AddUserAgent(config, "rsp-test")
	kubeClient := fedFixture.KubeApi.NewClient(tl, "rsp-test")
	fedClient := clientset.NewForConfigOrDie(config)
	err := common.EnableType(kubeClient, fedClient, schedulingtypes.JobTypeName, util.DefaultFederationSystemNamespace)
	if err != nil {
		tl.Fatalf("Failed to enable federated type :%s, with error: %v", schedulingtypes.JobTypeName, err)
	}

	fixture := framework.NewSchedulingControllerFixture(tl, config, schedulingtypes.JobSchedulingKind, fedFixture.SystemNamespace, fedFixture.SystemNamespace, metav1.NamespaceAll)
	return fixture, fedClient
}

func jspSpecWithoutClusterWeights(parallelism, completions int32) fedschedulingv1a1.JobSchedulingPreferenceSpec {
	return fedschedulingv1a1.JobSchedulingPreferenceSpec{
		TotalParallelism: parallelism,
		TotalCompletions: completions,
	}
}

func createJSPTestObjs(jspSpec fedschedulingv1a1.JobSchedulingPreferenceSpec, namespace string, fedClient clientset.Interface) (string, error) {
	// TODO(irfanurrehman): should the order of this creation matter?
	t, err := fedClient.CoreV1alpha1().FederatedJobs(namespace).Create(getFederatedJobTemplate(namespace))
	if err != nil {
		return "", err
	}

	jsp := &fedschedulingv1a1.JobSchedulingPreference{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: namespace,
		},
		Spec: jspSpec,
	}
	_, err = fedClient.SchedulingV1alpha1().JobSchedulingPreferences(namespace).Create(jsp)
	if err != nil {
		return "", err
	}

	return t.Name, nil
}

func waitForMatchingJobPlacement(fedClient clientset.Interface, name, namespace string, expected map[string]adapters.JobSchedulingResult) error {
	err := wait.Poll(framework.DefaultWaitInterval, wait.ForeverTestTimeout, func() (bool, error) {
		clusterNames := []string{}
		p, err := fedClient.CoreV1alpha1().FederatedJobPlacements(namespace).Get(name, metav1.GetOptions{})
		clusterNames = p.Spec.ClusterNames
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			} else {
				return false, err
			}
		}

		if len(clusterNames) > 0 {
			totalClusters := 0
			for _, clusterName := range clusterNames {
				totalClusters++
				_, exists := expected[clusterName]
				if !exists {
					return false, nil
				}
			}

			// All clusters in placement has a matched cluster name as in expected.
			if totalClusters == len(expected) {
				return true, nil
			}
		}
		return false, nil
	})

	return err
}

func waitForMatchingJobOverride(fedClient clientset.Interface, name, namespace string, expected map[string]adapters.JobSchedulingResult) error {
	err := wait.Poll(framework.DefaultWaitInterval, wait.ForeverTestTimeout, func() (bool, error) {
		override, err := fedClient.CoreV1alpha1().FederatedJobOverrides(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			} else {
				return false, err
			}
		}

		if override != nil {
			// We do not consider a case where overrides won't have any clusters listed
			match := false
			totalClusters := 0
			for _, clusterOverride := range override.Spec.Overrides {
				match = false // Check for each cluster listed in overrides
				totalClusters++
				name := clusterOverride.ClusterName
				expectedVals, exists := expected[name]
				// Overrides should have exact mapping values as in expected
				if !exists {
					return false, nil
				}
				if clusterOverride.Parallelism != nil &&
					*clusterOverride.Parallelism == expectedVals.Parallelism &&
					clusterOverride.Completions != nil &&
					*clusterOverride.Completions == expectedVals.Completions {
					match = true
					continue
				}

			}

			if match && (totalClusters == len(expected)) {
				return true, nil
			}
		}
		return false, nil
	})

	return err
}

func getFederatedJobTemplate(namespace string) *fedv1a1.FederatedJob {
	parallelism := int32(1)
	completions := int32(1)
	return &fedv1a1.FederatedJob{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-job-",
			Namespace:    namespace,
		},
		Spec: fedv1a1.FederatedJobSpec{
			Template: batchv1.Job{
				Spec: batchv1.JobSpec{
					Parallelism: &parallelism,
					Completions: &completions,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"foo": "bar"},
						},
						Spec: apiv1.PodSpec{
							Containers: []apiv1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
						},
					},
				},
			},
		},
	}
}
