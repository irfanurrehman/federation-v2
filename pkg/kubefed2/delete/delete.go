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
	"io"

	"github.com/golang/glog"
	"github.com/kubernetes-sigs/federation-v2/pkg/kubefed2/util"
	"github.com/spf13/cobra"
)

var (
	delete_long = `
		Deletes the API resources used to federate a type or a resource.
		Use "kubefed2 disable" instead, if the intent is to only disable
		propagation of the given type or a resource.

		Current context is assumed to be a Kubernetes cluster hosting
		the federation control plane. Please use the
		--host-cluster-context flag otherwise.`

	delete_example = `
		# Delete all API resources associated with and used for propagation
		of type Service.
		kubefed2 delete type Service`
)

// NewCmdDDelete  creates a command object for the "delete" action,
// and adds subcommands used to deletes API resources used for federating
// a kubernetes type or a resource.
func NewCmdDelete(cmdOut io.Writer, config util.FedConfig) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "delete SUBCOMMAND",
		Short:   "Delete the API resources used to federate a type or a resource",
		Long:    delete_long,
		Example: delete_example,
		Run: func(_ *cobra.Command, args []string) {
			if len(args) < 1 {
				glog.Fatalf("missing subcommand; \"delete\" is not meant to be run on its own")
			} else {
				glog.Fatalf("invalid subcommand: %q", args[0])
			}
		},
	}

	cmd.AddCommand(NewCmdDeleteType(cmdOut, config))

	return cmd
}
