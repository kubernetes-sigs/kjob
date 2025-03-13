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

package create

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	clocktesting "k8s.io/utils/clock/testing"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	cmdtesting "sigs.k8s.io/kjob/pkg/cmd/testing"
)

func TestCreateOptions_Complete(t *testing.T) {
	testStartTime := time.Now()

	testCases := map[string]struct {
		args        []string
		options     *CreateOptions
		wantOptions *CreateOptions
		wantErr     string
	}{
		"invalid request": {
			args: []string{"job"},
			options: &CreateOptions{
				Namespace:            metav1.NamespaceDefault,
				ModeName:             v1alpha1.JobMode,
				UserSpecifiedRequest: map[string]string{"cpu": "invalid"},
			},
			wantOptions: &CreateOptions{},
			wantErr:     "quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			streams, _, out, outErr := genericiooptions.NewTestIOStreams()

			tcg := cmdtesting.NewTestClientGetter()

			cmd := NewCreateCmd(tcg, streams, clocktesting.NewFakeClock(testStartTime))
			cmd.SetOut(out)
			cmd.SetErr(outErr)
			cmd.SetArgs(tc.args)

			gotErr := tc.options.Complete(tcg, cmd.Commands()[0], nil)

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
		})
	}
}
