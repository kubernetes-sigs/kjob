/*
Copyright 2024 The Kubernetes Authors.

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

package version

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/kubectl/pkg/util/templates"

	"sigs.k8s.io/kjob/pkg/version"
)

var (
	versionExample = templates.Examples(`
		# Prints the client version
  		kjobctl version
	`)
)

// VersionOptions is a struct to support version command
type VersionOptions struct {
	genericiooptions.IOStreams
}

// NewOptions returns initialized Options
func NewOptions(streams genericiooptions.IOStreams) *VersionOptions {
	return &VersionOptions{
		IOStreams: streams,
	}
}

// NewVersionCmd returns a new cobra.Command for fetching version
func NewVersionCmd(streams genericiooptions.IOStreams) *cobra.Command {
	o := NewOptions(streams)

	cmd := &cobra.Command{
		Use:                   "version",
		Short:                 "Prints the client version",
		Example:               versionExample,
		Args:                  cobra.NoArgs,
		DisableFlagsInUseLine: true,
		Run: func(_ *cobra.Command, _ []string) {
			o.Run()
		},
	}

	return cmd
}

// Run executes version command
func (o *VersionOptions) Run() {
	fmt.Fprintf(o.Out, "Client Version: %s\n", version.GitVersion)
}
