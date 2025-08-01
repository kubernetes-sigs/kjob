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

package helpers

import (
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

func AddFieldSelectorFlagVar(cmd *cobra.Command, p *string) {
	cmd.Flags().StringVar(p, "field-selector", "",
		"Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")
}

func AddLabelSelectorFlagVar(cmd *cobra.Command, p *string) {
	cmd.Flags().StringVarP(p, "selector", "l", "",
		"Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.")
}

func AddAllNamespacesFlagVar(cmd *cobra.Command, p *bool) {
	cmd.Flags().BoolVarP(p, "all-namespaces", "A", false,
		"If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
}

func AddProfileFlagVar(cmd *cobra.Command, p *string) {
	cmd.Flags().StringVarP(p, "profile", "p", "",
		"Filter by profile name which is associated with the resource.")
}

func AddLocalQueueFlagVar(cmd *cobra.Command, p *string) {
	cmd.Flags().StringVarP(p, "localqueue", "q", "",
		"Filter by localqueue which is associated with the resource.")
}

func FlagString(cmd *cobra.Command, flag string) string {
	s, err := cmd.Flags().GetString(flag)
	if err != nil {
		klog.Fatalf("error accessing flag %s for command %s: %v", flag, cmd.Name(), err)
	}
	return s
}
