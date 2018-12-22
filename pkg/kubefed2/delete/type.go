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

package delete

import (
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	apiextv1b1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"

	"github.com/kubernetes-sigs/federation-v2/pkg/apis/core/typeconfig"
	ctlutil "github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
	"github.com/kubernetes-sigs/federation-v2/pkg/kubefed2/disable"
	"github.com/kubernetes-sigs/federation-v2/pkg/kubefed2/options"
	"github.com/kubernetes-sigs/federation-v2/pkg/kubefed2/util"
)

var (
	type_long = `
		Deletes API resources used to federate a type. Use
		"kubefed2 disable" instead if the intention is to only
		diable propagation of the given type.

		Current context is assumed to be a Kubernetes cluster hosting
		the federation control plane. Please use the
		--host-cluster-context flag otherwise.`

	type_example = `
		# Delete API resources used for propagation of the type Service
		kubefed2 delete type Service

		# Delete API resources used for propagation of resource my-svc of type Service
		kubefed2 delete resource Service my-svc`
)

type deleteType struct {
	options.SubcommandOptions
	deleteTypeOptions
}

type deleteTypeOptions struct {
	targetName string
}

// NewCmdDeleteType defines the `delete type` command that
// deletes API resources used for federating a kubernetes type.
func NewCmdDeleteType(cmdOut io.Writer, config util.FedConfig) *cobra.Command {
	opts := &deleteType{}

	cmd := &cobra.Command{
		Use:     "type NAME",
		Short:   "Deletes API resources used to federate a type",
		Long:    type_long,
		Example: type_example,
		Run: func(cmd *cobra.Command, args []string) {
			err := opts.Complete(args)
			if err != nil {
				glog.Fatalf("error: %v", err)
			}

			err = opts.Run(cmdOut, config)
			if err != nil {
				glog.Fatalf("error: %v", err)
			}
		},
	}

	flags := cmd.Flags()
	opts.CommonBind(flags)

	return cmd
}

// Complete ensures that options are valid and marshals them if necessary.
func (j *deleteType) Complete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("NAME is required")
	}
	j.targetName = args[0]

	return nil
}

// Run is the implementation of the `delete type` command.
func (j *deleteType) Run(cmdOut io.Writer, config util.FedConfig) error {
	hostConfig, err := config.HostConfig(j.HostClusterContext, j.Kubeconfig)
	if err != nil {
		return fmt.Errorf("Failed to get host cluster config: %v", err)
	}

	typeConfigName := ctlutil.QualifiedName{
		Namespace: j.FederationNamespace,
		Name:      j.targetName,
	}

	err = DeleteTypeFederation(cmdOut, hostConfig, typeConfigName, j.DryRun)
	if err != nil {
		return err
	}

	return nil
}

func DeleteTypeFederation(cmdOut io.Writer, config *rest.Config, typeConfigName ctlutil.QualifiedName, dryRun bool) error {
	fedClient, err := util.FedClientset(config)
	if err != nil {
		return fmt.Errorf("Failed to get federation clientset: %v", err)
	}

	typeConfig, err := disable.DisableTypeFederation(fedClient, cmdOut, typeConfigName, dryRun)
	if err != nil {
		return err
	}

	if dryRun {
		return nil
	}

	// TODO(marun) consider waiting for the sync controller to be stopped before attempting deletion
	deletePrimitives(config, typeConfig, cmdOut)
	err = fedClient.CoreV1alpha1().FederatedTypeConfigs(typeConfigName.Namespace).Delete(typeConfigName.Name, nil)
	if err != nil {
		return fmt.Errorf("Error deleting FederatedTypeConfig %q: %v", typeConfigName, err)
	}
	util.WriteString(cmdOut, fmt.Sprintf("federatedtypeconfig %q deleted\n", typeConfigName))

	return nil
}

func deletePrimitives(config *rest.Config, typeConfig typeconfig.Interface, cmdOut io.Writer) error {
	client, err := apiextv1b1client.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Error creating crd client: %v", err)
	}

	failedDeletion := []string{}
	crdNames := primitiveCRDNames(typeConfig)
	for _, crdName := range crdNames {
		err := client.CustomResourceDefinitions().Delete(crdName, nil)
		if err != nil && !errors.IsNotFound(err) {
			glog.Errorf("Failed to delete crd %q: %v", crdName, err)
			failedDeletion = append(failedDeletion, crdName)
			continue
		}
		util.WriteString(cmdOut, fmt.Sprintf("customresourcedefinition %q deleted\n", crdName))
	}
	if len(failedDeletion) > 0 {
		return fmt.Errorf("The following crds were not deleted successfully (see error log for details): %v", failedDeletion)
	}

	return nil
}

func primitiveCRDNames(typeConfig typeconfig.Interface) []string {
	return []string{
		typeconfig.GroupQualifiedName(typeConfig.GetTemplate()),
		typeconfig.GroupQualifiedName(typeConfig.GetPlacement()),
		typeconfig.GroupQualifiedName(typeConfig.GetOverride()),
	}
}
