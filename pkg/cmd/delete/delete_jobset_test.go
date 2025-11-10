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
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubetesting "k8s.io/client-go/testing"
	jobsetapi "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	"sigs.k8s.io/jobset/client-go/clientset/versioned/fake"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	cmdtesting "sigs.k8s.io/kjob/pkg/cmd/testing"
	"sigs.k8s.io/kjob/pkg/testing/wrappers"
)

func TestJobSetCmd(t *testing.T) {
	testCases := map[string]struct {
		ns             string
		args           []string
		objs           []runtime.Object
		wantJobSetJobs []jobsetapi.JobSet
		wantOut        string
		wantOutErr     string
		wantErr        string
	}{
		"shouldn't delete jobset because it is not found": {
			args: []string{"j"},
			objs: []runtime.Object{
				wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Profile("p1").Mode(v1alpha1.JobSetMode).Obj(),
				wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p2").Mode(v1alpha1.JobSetMode).Obj(),
			},
			wantJobSetJobs: []jobsetapi.JobSet{
				*wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Profile("p1").Mode(v1alpha1.JobSetMode).Obj(),
				*wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p2").Mode(v1alpha1.JobSetMode).Obj(),
			},
			wantOutErr: "jobsets.jobset.x-k8s.io \"j\" not found\n",
		},
		"shouldn't delete jobset because it is not created via kjob": {
			args: []string{"j1"},
			objs: []runtime.Object{
				wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Obj(),
			},
			wantJobSetJobs: []jobsetapi.JobSet{
				*wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Obj(),
			},
			wantOutErr: "jobset \"j1\" not created via kjob\n",
		},
		"shouldn't delete jobSet because it is not used for JobSet mode": {
			args: []string{"j1", "j2"},
			objs: []runtime.Object{
				wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Profile("p1").Obj(),
				wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p1").Mode(v1alpha1.SlurmMode).Obj(),
			},
			wantJobSetJobs: []jobsetapi.JobSet{
				*wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Profile("p1").Obj(),
				*wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p1").Mode(v1alpha1.SlurmMode).Obj(),
			},
			wantOutErr: `jobset "j1" created in "" mode. Switch to the correct mode to delete it
jobset "j2" created in "Slurm" mode. Switch to the correct mode to delete it
`,
		},
		"should delete jobSet": {
			args: []string{"j1"},
			objs: []runtime.Object{
				wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Profile("p1").Mode(v1alpha1.JobSetMode).Obj(),
				wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p2").Mode(v1alpha1.JobSetMode).Obj(),
			},
			wantJobSetJobs: []jobsetapi.JobSet{
				*wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p2").Mode(v1alpha1.JobSetMode).Obj(),
			},
			wantOut: "jobset.jobset.x-k8s.io/j1 deleted\n",
		},
		"should delete jobSets": {
			args: []string{"j1", "j2"},
			objs: []runtime.Object{
				wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Profile("p1").Mode(v1alpha1.JobSetMode).Obj(),
				wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p2").Mode(v1alpha1.JobSetMode).Obj(),
			},
			wantOut: "jobset.jobset.x-k8s.io/j1 deleted\njobset.jobset.x-k8s.io/j2 deleted\n",
		},
		"should delete only one jobset": {
			args: []string{"j1", "j"},
			objs: []runtime.Object{
				wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Profile("p1").Mode(v1alpha1.JobSetMode).Obj(),
				wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p2").Mode(v1alpha1.JobSetMode).Obj(),
			},
			wantJobSetJobs: []jobsetapi.JobSet{
				*wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p2").Mode(v1alpha1.JobSetMode).Obj(),
			},
			wantOut:    "jobset.jobset.x-k8s.io/j1 deleted\n",
			wantOutErr: "jobsets.jobset.x-k8s.io \"j\" not found\n",
		},
		"shouldn't delete jobSet with client dry run": {
			args: []string{"j1", "--dry-run", "client"},
			objs: []runtime.Object{
				wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Profile("p1").Mode(v1alpha1.JobSetMode).Obj(),
				wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p2").Mode(v1alpha1.JobSetMode).Obj(),
			},
			wantJobSetJobs: []jobsetapi.JobSet{
				*wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Profile("p1").Mode(v1alpha1.JobSetMode).Obj(),
				*wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p2").Mode(v1alpha1.JobSetMode).Obj(),
			},
			wantOut: "jobset.jobset.x-k8s.io/j1 deleted (client dry run)\n",
		},
		"shouldn't delete job with server dry run": {
			args: []string{"j1", "--dry-run", "server"},
			objs: []runtime.Object{
				wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Profile("p1").Mode(v1alpha1.JobSetMode).Obj(),
				wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p2").Mode(v1alpha1.JobSetMode).Obj(),
			},
			wantJobSetJobs: []jobsetapi.JobSet{
				*wrappers.MakeJobSet("j1", metav1.NamespaceDefault).Profile("p1").Mode(v1alpha1.JobSetMode).Obj(),
				*wrappers.MakeJobSet("j2", metav1.NamespaceDefault).Profile("p2").Mode(v1alpha1.JobSetMode).Obj(),
			},
			wantOut: "jobset.jobset.x-k8s.io/j1 deleted (server dry run)\n",
		},
		"no args": {
			args:    []string{},
			wantErr: "requires at least 1 arg(s), only received 0",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ns := metav1.NamespaceDefault
			if tc.ns != "" {
				ns = tc.ns
			}

			streams, _, out, outErr := genericiooptions.NewTestIOStreams()

			clientset := fake.NewSimpleClientset(tc.objs...)
			clientset.PrependReactor("delete", "jobsets", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				if slices.Contains(action.(kubetesting.DeleteAction).GetDeleteOptions().DryRun, metav1.DryRunAll) {
					handled = true
				}
				return handled, ret, err
			})

			tcg := cmdtesting.NewTestClientGetter().
				WithJobSetClientset(clientset).
				WithNamespace(ns)

			cmd := NewJobSetCmd(tcg, streams)
			cmd.SetOut(out)
			cmd.SetErr(outErr)
			cmd.SetArgs(tc.args)

			gotErr := cmd.Execute()

			var gotErrStr string
			if gotErr != nil {
				gotErrStr = gotErr.Error()
			}

			if diff := cmp.Diff(tc.wantErr, gotErrStr); diff != "" {
				t.Errorf("Unexpected error (-want/+got)\n%s", diff)
			}

			if gotErr != nil {
				return
			}

			gotOut := out.String()
			if diff := cmp.Diff(tc.wantOut, gotOut); diff != "" {
				t.Errorf("Unexpected output (-want/+got)\n%s", diff)
			}

			gotOutErr := outErr.String()
			if diff := cmp.Diff(tc.wantOutErr, gotOutErr); diff != "" {
				t.Errorf("Unexpected error output (-want/+got)\n%s", diff)
			}

			gotJobSetList, err := clientset.JobsetV1alpha2().JobSets(tc.ns).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Error(err)
				return
			}

			if diff := cmp.Diff(tc.wantJobSetJobs, gotJobSetList.Items); diff != "" {
				t.Errorf("Unexpected jobsets (-want/+got)\n%s", diff)
			}
		})
	}
}
