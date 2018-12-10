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
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/kubernetes-sigs/federation-v2/pkg/kubefed2/util"
)

var (
	disable_long = `
		Disables propagation of a Kubernetes API type or a Federated
		Resource into federated clusters.
		Use "kubefed2 delete" instead if the intention is also to remove
		the API resources added while federating the type or the resource.

		Current context is assumed to be a Kubernetes cluster hosting
		the federation control plane. Please use the
		--host-cluster-context flag otherwise.`

	disable_example = `
		# Disable propagation of API type Service
		kubefed2 disable type Service`
)

// NewCmdDisable creates a command object for the "disable" action,
// and adds sub commands used to disable propagation of an API type or a resource.
func NewCmdDisable(cmdOut io.Writer, config util.FedConfig) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "disable SUBCOMMAND",
		Short:   "Disables propagation of a Kubernetes API type or a resource.",
		Long:    disable_long,
		Example: disable_example,
		Run: func(_ *cobra.Command, args []string) {
			if len(args) < 1 {
				glog.Fatalf("missing subcommand; \"disable\" is not meant to be run on its own")
			} else {
				glog.Fatalf("invalid subcommand: %q", args[0])
			}
		},
	}

	cmd.AddCommand(NewCmdDisableType(cmdOut, config))

	return cmd
}
