/*
Copyright 2025 Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package list

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubetesting "k8s.io/client-go/testing"
	testingclock "k8s.io/utils/clock/testing"
	jobsetapi "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	"sigs.k8s.io/jobset/client-go/clientset/versioned/fake"

	cmdtesting "sigs.k8s.io/kjob/pkg/cmd/testing"
	"sigs.k8s.io/kjob/pkg/testing/wrappers"
)

func TestJobSetCmd(t *testing.T) {
	testStartTime := time.Now().Truncate(time.Second)

	testCases := map[string]struct {
		ns         string
		objs       []runtime.Object
		args       []string
		wantOut    string
		wantOutErr string
		wantErr    error
	}{
		"should print only kjobctl jobsets": {
			args: []string{},
			objs: []runtime.Object{
				wrappers.MakeJobSet("rj1", metav1.NamespaceDefault).
					Profile("profile1").
					LocalQueue("localqueue1").
					CreationTimestamp(testStartTime.Add(-2 * time.Hour)).
					Obj(),
				wrappers.MakeJobSet("rj2", "ns1").
					LocalQueue("localqueue1").
					CreationTimestamp(testStartTime.Add(-2 * time.Hour)).
					Obj(),
			},
			wantOut: `NAME   PROFILE    LOCAL QUEUE
rj1    profile1   localqueue1
`,
		},
		"should print jobset list with namespace filter": {
			args: []string{},
			ns:   "ns1",
			objs: []runtime.Object{
				wrappers.MakeJobSet("rj1", "ns1").
					Profile("profile1").
					LocalQueue("localqueue1").
					CreationTimestamp(testStartTime.Add(-2 * time.Hour)).
					Obj(),
				wrappers.MakeJobSet("rj2", "ns2").
					Profile("profile1").
					LocalQueue("localqueue1").
					CreationTimestamp(testStartTime.Add(-2 * time.Hour)).
					Obj(),
			},
			wantOut: `NAME   PROFILE    LOCAL QUEUE
rj1    profile1   localqueue1
`,
		},
		"should print not found error": {
			args:       []string{},
			wantOutErr: fmt.Sprintf("No resources found in %s namespace.\n", metav1.NamespaceDefault),
		},
		"should print not found error with all-namespaces filter": {
			args:       []string{"-A"},
			wantOutErr: "No resources found\n",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			streams, _, out, outErr := genericiooptions.NewTestIOStreams()

			clientset := fake.NewSimpleClientset(tc.objs...)
			clientset.PrependReactor("list", "jobsets", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				listAction := action.(kubetesting.ListActionImpl)
				fieldsSelector := listAction.GetListRestrictions().Fields

				obj, err := clientset.Tracker().List(listAction.GetResource(), listAction.GetKind(), listAction.Namespace)
				jobSetList := obj.(*jobsetapi.JobSetList)

				filtered := make([]jobsetapi.JobSet, 0, len(jobSetList.Items))
				for _, item := range jobSetList.Items {
					fieldsSet := fields.Set{
						"metadata.name": item.Name,
					}
					if fieldsSelector.Matches(fieldsSet) {
						filtered = append(filtered, item)
					}
				}
				jobSetList.Items = filtered
				return true, jobSetList, err
			})

			tcg := cmdtesting.NewTestClientGetter().WithJobSetClientset(clientset)
			if len(tc.ns) > 0 {
				tcg.WithNamespace(tc.ns)
			}

			cmd := NewJobSetCmd(tcg, streams, testingclock.NewFakeClock(testStartTime))
			cmd.SetArgs(tc.args)

			gotErr := cmd.Execute()
			if diff := cmp.Diff(tc.wantErr, gotErr, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("Unexpected error (-want/+got)\n%s", diff)
			}

			gotOut := out.String()
			if diff := cmp.Diff(tc.wantOut, gotOut); diff != "" {
				t.Errorf("Unexpected output (-want/+got)\n%s", diff)
			}

			gotOutErr := outErr.String()
			if diff := cmp.Diff(tc.wantOutErr, gotOutErr); diff != "" {
				t.Errorf("Unexpected output (-want/+got)\n%s", diff)
			}
		})
	}
}
