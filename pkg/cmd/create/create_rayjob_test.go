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

package create

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic/fake"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	clocktesting "k8s.io/utils/clock/testing"
	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	kueuefake "sigs.k8s.io/kueue/client-go/clientset/versioned/fake"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	kjobctlfake "sigs.k8s.io/kjob/client-go/clientset/versioned/fake"
	cmdtesting "sigs.k8s.io/kjob/pkg/cmd/testing"
	"sigs.k8s.io/kjob/pkg/testing/wrappers"
)

func TestCreateRayJobCmd(t *testing.T) {
	testStartTime := time.Now()
	testCases := map[string]struct {
		ns          string
		args        []string
		kjobctlObjs []runtime.Object
		kueueObjs   []runtime.Object
		gvks        []schema.GroupVersionKind
		wantLists   []runtime.Object
		wantOut     string
		wantOutErr  string
		wantErr     string
	}{
		"should create ray job": {
			args: []string{"rayjob", "--profile", "profile"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeRayJobTemplate("ray-job-template", metav1.NamespaceDefault).Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.RayJobMode, "ray-job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "ray.io", Version: "v1", Kind: "RayJob"}},
			wantLists: []runtime.Object{
				&rayv1.RayJobList{
					TypeMeta: metav1.TypeMeta{Kind: "RayJobList", APIVersion: "ray.io/v1"},
					Items: []rayv1.RayJob{
						*wrappers.MakeRayJob("", metav1.NamespaceDefault).
							GenerateName("profile-rayjob-").
							Profile("profile").
							Mode(v1alpha1.RayJobMode).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "rayjob.ray.io/<unknown> created\n",
		},
		"should create ray job with replicas replacement": {
			args: []string{"rayjob", "--profile", "profile", "--replicas", "g1=5"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeRayJobTemplate("ray-job-template", metav1.NamespaceDefault).
					WithRayClusterSpec(
						wrappers.MakeRayClusterSpec().
							WithWorkerGroupSpec(*wrappers.MakeWorkerGroupSpec("g1").Obj()).
							Obj(),
					).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.RayJobMode, "ray-job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "ray.io", Version: "v1", Kind: "RayJob"}},
			wantLists: []runtime.Object{
				&rayv1.RayJobList{
					TypeMeta: metav1.TypeMeta{Kind: "RayJobList", APIVersion: "ray.io/v1"},
					Items: []rayv1.RayJob{
						*wrappers.MakeRayJob("", metav1.NamespaceDefault).
							GenerateName("profile-rayjob-").
							Profile("profile").
							Mode(v1alpha1.RayJobMode).
							WithWorkerGroupSpec(*wrappers.MakeWorkerGroupSpec("g1").Replicas(5).Obj()).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "rayjob.ray.io/<unknown> created\n",
		},
		"should create ray job with cmd replacement": {
			args: []string{"rayjob", "--profile", "profile", "--cmd", "sleep   3s"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeRayJobTemplate("ray-job-template", metav1.NamespaceDefault).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.RayJobMode, "ray-job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "ray.io", Version: "v1", Kind: "RayJob"}},
			wantLists: []runtime.Object{
				&rayv1.RayJobList{
					TypeMeta: metav1.TypeMeta{Kind: "RayJobList", APIVersion: "ray.io/v1"},
					Items: []rayv1.RayJob{
						*wrappers.MakeRayJob("", metav1.NamespaceDefault).
							GenerateName("profile-rayjob-").
							Profile("profile").
							Mode(v1alpha1.RayJobMode).
							Entrypoint("sleep 3s").
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "rayjob.ray.io/<unknown> created\n",
		},
		"should create ray job with min-replicas replacement": {
			args: []string{"rayjob", "--profile", "profile", "--min-replicas", "g1=5"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeRayJobTemplate("ray-job-template", metav1.NamespaceDefault).
					WithRayClusterSpec(
						wrappers.MakeRayClusterSpec().
							WithWorkerGroupSpec(*wrappers.MakeWorkerGroupSpec("g1").Obj()).
							Obj(),
					).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.RayJobMode, "ray-job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "ray.io", Version: "v1", Kind: "RayJob"}},
			wantLists: []runtime.Object{
				&rayv1.RayJobList{
					TypeMeta: metav1.TypeMeta{Kind: "RayJobList", APIVersion: "ray.io/v1"},
					Items: []rayv1.RayJob{
						*wrappers.MakeRayJob("", metav1.NamespaceDefault).
							GenerateName("profile-rayjob-").
							Profile("profile").
							Mode(v1alpha1.RayJobMode).
							WithWorkerGroupSpec(*wrappers.MakeWorkerGroupSpec("g1").MinReplicas(5).Obj()).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "rayjob.ray.io/<unknown> created\n",
		},
		"should create ray job with max-replicas replacement": {
			args: []string{"rayjob", "--profile", "profile", "--max-replicas", "g1=5"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeRayJobTemplate("ray-job-template", metav1.NamespaceDefault).
					WithRayClusterSpec(
						wrappers.MakeRayClusterSpec().
							WithWorkerGroupSpec(*wrappers.MakeWorkerGroupSpec("g1").Obj()).
							Obj(),
					).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.RayJobMode, "ray-job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "ray.io", Version: "v1", Kind: "RayJob"}},
			wantLists: []runtime.Object{
				&rayv1.RayJobList{
					TypeMeta: metav1.TypeMeta{Kind: "RayJobList", APIVersion: "ray.io/v1"},
					Items: []rayv1.RayJob{
						*wrappers.MakeRayJob("", metav1.NamespaceDefault).
							GenerateName("profile-rayjob-").
							Profile("profile").
							Mode(v1alpha1.RayJobMode).
							WithWorkerGroupSpec(*wrappers.MakeWorkerGroupSpec("g1").MaxReplicas(5).Obj()).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "rayjob.ray.io/<unknown> created\n",
		},
		"should create ray job with ray cluster replacement": {
			args: []string{"rayjob", "--profile", "profile", "--raycluster", "rc1"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeRayJobTemplate("ray-job-template", metav1.NamespaceDefault).
					WithRayClusterSpec(
						wrappers.MakeRayClusterSpec().
							WithWorkerGroupSpec(*wrappers.MakeWorkerGroupSpec("g1").Obj()).
							Obj(),
					).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.RayJobMode, "ray-job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "ray.io", Version: "v1", Kind: "RayJob"}},
			wantLists: []runtime.Object{
				&rayv1.RayJobList{
					TypeMeta: metav1.TypeMeta{Kind: "RayJobList", APIVersion: "ray.io/v1"},
					Items: []rayv1.RayJob{
						*wrappers.MakeRayJob("", metav1.NamespaceDefault).
							GenerateName("profile-rayjob-").
							Profile("profile").
							Mode(v1alpha1.RayJobMode).
							WithRayClusterLabelSelector("rc1").
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "rayjob.ray.io/<unknown> created\n",
		},
		"should create ray job with --priority flag": {
			args: []string{"rayjob", "--profile", "profile", "--priority", "sample-priority"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeRayJobTemplate("ray-job-template", metav1.NamespaceDefault).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.RayJobMode, "ray-job-template").Obj()).
					Obj(),
			},
			kueueObjs: []runtime.Object{
				&kueue.WorkloadPriorityClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sample-priority",
					},
				},
			},
			gvks: []schema.GroupVersionKind{{Group: "ray.io", Version: "v1", Kind: "RayJob"}},
			wantLists: []runtime.Object{
				&rayv1.RayJobList{
					TypeMeta: metav1.TypeMeta{Kind: "RayJobList", APIVersion: "ray.io/v1"},
					Items: []rayv1.RayJob{
						*wrappers.MakeRayJob("", metav1.NamespaceDefault).
							Priority("sample-priority").
							GenerateName("profile-rayjob-").
							Profile("profile").
							Mode(v1alpha1.RayJobMode).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "rayjob.ray.io/<unknown> created\n",
		},
		"should create ray job with --priority flag and skip workload priority class validation": {
			args: []string{"rayjob", "--profile", "profile", "--skip-priority-validation", "--priority", "sample-priority"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeRayJobTemplate("ray-job-template", metav1.NamespaceDefault).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.RayJobMode, "ray-job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "ray.io", Version: "v1", Kind: "RayJob"}},
			wantLists: []runtime.Object{
				&rayv1.RayJobList{
					TypeMeta: metav1.TypeMeta{Kind: "RayJobList", APIVersion: "ray.io/v1"},
					Items: []rayv1.RayJob{
						*wrappers.MakeRayJob("", metav1.NamespaceDefault).
							Priority("sample-priority").
							GenerateName("profile-rayjob-").
							Profile("profile").
							Mode(v1alpha1.RayJobMode).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "rayjob.ray.io/<unknown> created\n",
		},
		"shouldn't create ray job with ray cluster and localqueue replacements because mutually exclusive": {
			args:    []string{"rayjob", "--profile", "profile", "--raycluster", "rc1", "--localqueue", "lq1"},
			wantErr: "if any flags in the group [raycluster localqueue] are set none of the others can be; [localqueue raycluster] were all set",
		},
		"shouldn't create ray job with ray cluster and replicas replacements because mutually exclusive": {
			args:    []string{"rayjob", "--profile", "profile", "--raycluster", "rc1", "--replicas", "g1=5"},
			wantErr: "if any flags in the group [raycluster replicas] are set none of the others can be; [raycluster replicas] were all set",
		},
		"shouldn't create ray job with ray cluster and min-replicas replacements because mutually exclusive": {
			args:    []string{"rayjob", "--profile", "profile", "--raycluster", "rc1", "--min-replicas", "g1=5"},
			wantErr: "if any flags in the group [raycluster min-replicas] are set none of the others can be; [min-replicas raycluster] were all set",
		},
		"shouldn't create ray job with ray cluster and max-replicas replacements because mutually exclusive": {
			args:    []string{"rayjob", "--profile", "profile", "--raycluster", "rc1", "--max-replicas", "g1=5"},
			wantErr: "if any flags in the group [raycluster max-replicas] are set none of the others can be; [max-replicas raycluster] were all set",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			streams, _, out, outErr := genericiooptions.NewTestIOStreams()

			scheme := runtime.NewScheme()
			utilruntime.Must(k8sscheme.AddToScheme(scheme))
			utilruntime.Must(rayv1.AddToScheme(scheme))

			clientset := kjobctlfake.NewSimpleClientset(tc.kjobctlObjs...)
			dynamicClient := fake.NewSimpleDynamicClient(scheme)
			kueueClientset := kueuefake.NewSimpleClientset(tc.kueueObjs...)
			restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})

			for _, gvk := range tc.gvks {
				restMapper.Add(gvk, meta.RESTScopeNamespace)
			}

			tcg := cmdtesting.NewTestClientGetter().
				WithKjobctlClientset(clientset).
				WithDynamicClient(dynamicClient).
				WithKueueClientset(kueueClientset).
				WithRESTMapper(restMapper)
			if tc.ns != "" {
				tcg.WithNamespace(tc.ns)
			}

			cmd := NewCreateCmd(tcg, streams, clocktesting.NewFakeClock(testStartTime))
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
				t.Errorf("Unexpected output (-want/+got)\n%s", diff)
			}

			for index, gvk := range tc.gvks {
				mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
				if err != nil {
					t.Error(err)
					return
				}

				unstructured, err := dynamicClient.Resource(mapping.Resource).Namespace(metav1.NamespaceDefault).
					List(context.Background(), metav1.ListOptions{})
				if err != nil {
					t.Error(err)
					return
				}

				gotList := tc.wantLists[index].DeepCopyObject()

				err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), gotList)
				if err != nil {
					t.Error(err)
					return
				}

				if diff := cmp.Diff(tc.wantLists[index], gotList); diff != "" {
					t.Errorf("Unexpected list for %s (-want/+got)\n%s", gvk.String(), diff)
				}
			}
		})
	}
}
