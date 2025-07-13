/*
Copyright 2025 The Kubernetes Authors.

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
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetapi "sigs.k8s.io/jobset/client-go/clientset/versioned/typed/jobset/v1alpha2"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	"sigs.k8s.io/kjob/pkg/cmd/completion"
	"sigs.k8s.io/kjob/pkg/cmd/helpers"
	"sigs.k8s.io/kjob/pkg/constants"
)

var (
	jobSetExample = templates.Examples(`
		# Delete JobSet 
  		kjobctl delete jobset my-application-profile-job-k2wzd
	`)
)

type JobSetOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	JobSetNames []string
	Namespace   string

	CascadeStrategy metav1.DeletionPropagation
	DryRunStrategy  helpers.DryRunStrategy

	Client jobsetapi.JobsetV1alpha2Interface

	PrintObj printers.ResourcePrinterFunc

	genericiooptions.IOStreams
}

func NewJobSetOptions(streams genericiooptions.IOStreams) *JobSetOptions {
	return &JobSetOptions{
		PrintFlags: genericclioptions.NewPrintFlags("deleted").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

func NewJobSetCmd(clientGetter helpers.ClientGetter, streams genericiooptions.IOStreams) *cobra.Command {
	o := NewJobSetOptions(streams)

	cmd := &cobra.Command{
		Use:                   "jobset NAME [--cascade STRATEGY] [--dry-run STRATEGY]",
		DisableFlagsInUseLine: true,
		Short:                 "Delete JobSet",
		Example:               jobSetExample,
		Args:                  cobra.MinimumNArgs(1),
		ValidArgsFunction:     completion.JobSetNameFunc(clientGetter),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := o.Complete(clientGetter, cmd, args)
			if err != nil {
				return err
			}

			return o.Run(cmd.Context())
		},
	}

	addCascadingFlag(cmd)
	helpers.AddDryRunFlag(cmd)

	o.PrintFlags.AddFlags(cmd)

	return cmd
}

func (o *JobSetOptions) Complete(clientGetter helpers.ClientGetter, cmd *cobra.Command, args []string) error {
	o.JobSetNames = args

	var err error

	o.Namespace, _, err = clientGetter.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.DryRunStrategy, err = helpers.GetDryRunStrategy(cmd)
	if err != nil {
		return err
	}

	err = helpers.PrintFlagsWithDryRunStrategy(o.PrintFlags, o.DryRunStrategy)
	if err != nil {
		return err
	}

	o.CascadeStrategy, err = getCascadingStrategy(cmd)
	if err != nil {
		return err
	}

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	o.PrintObj = printer.PrintObj

	clientset, err := clientGetter.JobSetClientset()
	if err != nil {
		return err
	}

	o.Client = clientset.JobsetV1alpha2()

	return nil
}

func (o *JobSetOptions) Run(ctx context.Context) error {
	for _, jobName := range o.JobSetNames {
		jobSet, err := o.Client.JobSets(o.Namespace).Get(ctx, jobName, metav1.GetOptions{})
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		if err != nil {
			fmt.Fprintln(o.ErrOut, err)
			continue
		}
		if _, ok := jobSet.Labels[constants.ProfileLabel]; !ok {
			fmt.Fprintf(o.ErrOut, "jobset \"%s\" not created via kjob\n", jobSet.Name)
			continue
		}
		if jobSet.Labels[constants.ModeLabel] != string(v1alpha1.JobSetMode) {
			fmt.Fprintf(o.ErrOut, "jobset \"%s\" created in \"%s\" mode. Switch to the correct mode to delete it\n",
				jobSet.Name, jobSet.Labels[constants.ModeLabel])
			continue
		}

		if o.DryRunStrategy != helpers.DryRunClient {
			deleteOptions := metav1.DeleteOptions{
				PropagationPolicy: ptr.To(o.CascadeStrategy),
			}

			if o.DryRunStrategy == helpers.DryRunServer {
				deleteOptions.DryRun = []string{metav1.DryRunAll}
			}

			if err := o.Client.JobSets(o.Namespace).Delete(ctx, jobName, deleteOptions); err != nil {
				return err
			}
		}

		if err := o.PrintObj(jobSet, o.Out); err != nil {
			return err
		}
	}

	return nil
}
