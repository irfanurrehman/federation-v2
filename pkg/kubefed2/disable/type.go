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

package disable

import (
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/kubernetes-sigs/federation-v2/pkg/apis/core/typeconfig"
	fedclient "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset/versioned"
	ctlutil "github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
	"github.com/kubernetes-sigs/federation-v2/pkg/kubefed2/options"
	"github.com/kubernetes-sigs/federation-v2/pkg/kubefed2/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	type_long = `
		Disables propagation of a Kubernetes API type.
		Use "kubefed2 delete" instead if the intention is also to
		remove the API resources added by "federate" command.

		Current context is assumed to be a Kubernetes cluster hosting
		the federation control plane. Please use the
		--host-cluster-context flag otherwise.`

	type_example = `
		# Disable propagation of type Service
		kubefed2 disable type Service`
)

type disableType struct {
	options.SubcommandOptions
	disableTypeOptions
}

type disableTypeOptions struct {
	targetName string
}

// NewCmdDisableType defines the `disable type` command that
// disables federation of a Kubernetes API type.
func NewCmdDisableType(cmdOut io.Writer, config util.FedConfig) *cobra.Command {
	opts := &disableType{}

	cmd := &cobra.Command{
		Use:     "type NAME",
		Short:   "Disables propagation of a Kubernetes API type",
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
func (j *disableType) Complete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("NAME is required")
	}
	j.targetName = args[0]

	return nil
}

// Run is the implementation of the `disable type` command.
func (j *disableType) Run(cmdOut io.Writer, config util.FedConfig) error {
	hostConfig, err := config.HostConfig(j.HostClusterContext, j.Kubeconfig)
	if err != nil {
		return fmt.Errorf("Failed to get host cluster config: %v", err)
	}

	typeConfigName := ctlutil.QualifiedName{
		Namespace: j.FederationNamespace,
		Name:      j.targetName,
	}

	fedClient, err := util.FedClientset(hostConfig)
	if err != nil {
		return fmt.Errorf("Failed to get federation clientset: %v", err)
	}

	_, err = DisableTypeFederation(fedClient, cmdOut, typeConfigName, j.DryRun)
	if err != nil {
		return err
	}

	return nil
}

func DisableTypeFederation(fedClient *fedclient.Clientset, cmdOut io.Writer, typeConfigName ctlutil.QualifiedName, dryRun bool) (typeconfig.Interface, error) {
	typeConfig, err := fedClient.CoreV1alpha1().FederatedTypeConfigs(typeConfigName.Namespace).Get(typeConfigName.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error retrieving FederatedTypeConfig %q: %v", typeConfigName, err)
	}

	if dryRun {
		return typeConfig, nil
	}

	if typeConfig.Spec.PropagationEnabled {
		typeConfig.Spec.PropagationEnabled = false
		_, err = fedClient.CoreV1alpha1().FederatedTypeConfigs(typeConfig.Namespace).Update(typeConfig)
		if err != nil {
			return nil, fmt.Errorf("Error disabling propagation for FederatedTypeConfig %q: %v", typeConfigName, err)
		}
		util.WriteString(cmdOut, fmt.Sprintf("Disabled propagation for FederatedTypeConfig %q\n", typeConfigName))
	} else {
		util.WriteString(cmdOut, fmt.Sprintf("Propagation already disabled for FederatedTypeConfig %q\n", typeConfigName))
	}

	return typeConfig, nil
}
