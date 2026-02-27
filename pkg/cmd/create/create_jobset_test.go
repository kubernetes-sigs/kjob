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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic/fake"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	clocktesting "k8s.io/utils/clock/testing"
	jobsetapi "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	kueuefake "sigs.k8s.io/kueue/client-go/clientset/versioned/fake"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	kjobctlfake "sigs.k8s.io/kjob/client-go/clientset/versioned/fake"
	cmdtesting "sigs.k8s.io/kjob/pkg/cmd/testing"
	"sigs.k8s.io/kjob/pkg/constants"
	"sigs.k8s.io/kjob/pkg/testing/wrappers"
)

func makeExampleJobSetSpec() jobsetapi.JobSetSpec {
	return jobsetapi.JobSetSpec{
		ReplicatedJobs: []jobsetapi.ReplicatedJob{
			{
				Name: "rj",
				Template: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "c1", Image: "busybox"},
									{Name: "c2", Image: "busybox"},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestCreateJobSetCmd(t *testing.T) {
	testStartTime := time.Now()
	userID := os.Getenv(constants.SystemEnvVarNameUser)

	jobSetGVK := schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"}

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
		"should create jobSet": {
			args: []string{"jobset", "--profile", "profile"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobSetTemplate("jobSet", metav1.NamespaceDefault).Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobSetMode, "jobSet").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{jobSetGVK},
			wantLists: []runtime.Object{
				&jobsetapi.JobSetList{
					TypeMeta: metav1.TypeMeta{Kind: "JobSetList", APIVersion: "jobset.x-k8s.io/v1alpha2"},
					Items: []jobsetapi.JobSet{
						*wrappers.MakeJobSet("", metav1.NamespaceDefault).
							GenerateName("profile-jobset-").
							Profile("profile").
							Mode(v1alpha1.JobSetMode).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "jobset.jobset.x-k8s.io/<unknown> created\n",
		},
		"should create jobSet with localqueue": {
			args: []string{"jobset", "--profile", "profile", "--localqueue", "lq1"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobSetTemplate("jobSet", metav1.NamespaceDefault).Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobSetMode, "jobSet").Obj()).
					Obj(),
			},
			kueueObjs: []runtime.Object{
				&kueue.LocalQueue{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: metav1.NamespaceDefault,
						Name:      "lq1",
					},
				},
			},
			gvks: []schema.GroupVersionKind{jobSetGVK},
			wantLists: []runtime.Object{
				&jobsetapi.JobSetList{
					TypeMeta: metav1.TypeMeta{Kind: "JobSetList", APIVersion: "jobset.x-k8s.io/v1alpha2"},
					Items: []jobsetapi.JobSet{
						*wrappers.MakeJobSet("", metav1.NamespaceDefault).
							GenerateName("profile-jobset-").
							Profile("profile").
							Mode(v1alpha1.JobSetMode).
							LocalQueue("lq1").
							Obj(),
					},
				},
			},
			wantOut: "jobset.jobset.x-k8s.io/<unknown> created\n",
		},
		"should create jobSet with priority": {
			args: []string{"jobset", "--profile", "profile", "--priority", "sample-priority"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobSetTemplate("jobSet", metav1.NamespaceDefault).Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobSetMode, "jobSet").Obj()).
					Obj(),
			},
			kueueObjs: []runtime.Object{
				&kueue.WorkloadPriorityClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sample-priority",
					},
				},
			},
			gvks: []schema.GroupVersionKind{jobSetGVK},
			wantLists: []runtime.Object{
				&jobsetapi.JobSetList{
					TypeMeta: metav1.TypeMeta{Kind: "JobSetList", APIVersion: "jobset.x-k8s.io/v1alpha2"},
					Items: []jobsetapi.JobSet{
						*wrappers.MakeJobSet("", metav1.NamespaceDefault).
							GenerateName("profile-jobset-").
							Profile("profile").
							Mode(v1alpha1.JobSetMode).
							Priority("sample-priority").
							Obj(),
					},
				},
			},
			wantOut: "jobset.jobset.x-k8s.io/<unknown> created\n",
		},
		"should create jobSet with replicas": {
			args: []string{"jobset", "--profile", "profile", "--replicas", "rj=5"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobSetTemplate("jobSet", metav1.NamespaceDefault).
					Spec(makeExampleJobSetSpec()).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobSetMode, "jobSet").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{jobSetGVK},
			wantLists: []runtime.Object{
				&jobsetapi.JobSetList{
					TypeMeta: metav1.TypeMeta{Kind: "JobSetList", APIVersion: "jobset.x-k8s.io/v1alpha2"},
					Items: []jobsetapi.JobSet{
						*wrappers.MakeJobSet("", metav1.NamespaceDefault).
							GenerateName("profile-jobset-").
							Profile("profile").
							Mode(v1alpha1.JobSetMode).
							Spec(makeExampleJobSetSpec()).
							Replicas("rj", 5).
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarNameUserID, Value: userID}).
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarTaskName, Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{
								Name:  constants.EnvVarTaskID,
								Value: fmt.Sprintf("%s_%s_default_profile", userID, testStartTime.Format(time.RFC3339)),
							}).
							WithEnvVar(corev1.EnvVar{Name: "PROFILE", Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{Name: "TIMESTAMP", Value: testStartTime.Format(time.RFC3339)}).
							Obj(),
					},
				},
			},
			wantOut: "jobset.jobset.x-k8s.io/<unknown> created\n",
		},
		"should create jobSet with command": {
			args: []string{"jobset", "--profile", "profile", "--cmd", "sleep 15s"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobSetTemplate("jobSet", metav1.NamespaceDefault).
					Spec(makeExampleJobSetSpec()).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobSetMode, "jobSet").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{jobSetGVK},
			wantLists: []runtime.Object{
				&jobsetapi.JobSetList{
					TypeMeta: metav1.TypeMeta{Kind: "JobSetList", APIVersion: "jobset.x-k8s.io/v1alpha2"},
					Items: []jobsetapi.JobSet{
						*wrappers.MakeJobSet("", metav1.NamespaceDefault).
							GenerateName("profile-jobset-").
							Profile("profile").
							Mode(v1alpha1.JobSetMode).
							Spec(makeExampleJobSetSpec()).
							Command([]string{"sleep", "15s"}).
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarNameUserID, Value: userID}).
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarTaskName, Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{
								Name:  constants.EnvVarTaskID,
								Value: fmt.Sprintf("%s_%s_default_profile", userID, testStartTime.Format(time.RFC3339)),
							}).
							WithEnvVar(corev1.EnvVar{Name: "PROFILE", Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{Name: "TIMESTAMP", Value: testStartTime.Format(time.RFC3339)}).
							Obj(),
					},
				},
			},
			wantOut: "jobset.jobset.x-k8s.io/<unknown> created\n",
		},
		"should create jobSet with request": {
			args: []string{"jobset", "--profile", "profile", "--request", "cpu=100m"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobSetTemplate("jobSet", metav1.NamespaceDefault).
					Spec(makeExampleJobSetSpec()).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobSetMode, "jobSet").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{jobSetGVK},
			wantLists: []runtime.Object{
				&jobsetapi.JobSetList{
					TypeMeta: metav1.TypeMeta{Kind: "JobSetList", APIVersion: "jobset.x-k8s.io/v1alpha2"},
					Items: []jobsetapi.JobSet{
						*wrappers.MakeJobSet("", metav1.NamespaceDefault).
							GenerateName("profile-jobset-").
							Profile("profile").
							Mode(v1alpha1.JobSetMode).
							Spec(makeExampleJobSetSpec()).
							Request(corev1.ResourceCPU, resource.MustParse("100m")).
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarNameUserID, Value: userID}).
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarTaskName, Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{
								Name:  constants.EnvVarTaskID,
								Value: fmt.Sprintf("%s_%s_default_profile", userID, testStartTime.Format(time.RFC3339)),
							}).
							WithEnvVar(corev1.EnvVar{Name: "PROFILE", Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{Name: "TIMESTAMP", Value: testStartTime.Format(time.RFC3339)}).
							Obj(),
					},
				},
			},
			wantOut: "jobset.jobset.x-k8s.io/<unknown> created\n",
		},
		"should create jobSet with pod template labels and annotations": {
			args: []string{
				"jobset",
				"--profile", "profile",
				"--pod-template-label", "foo=bar",
				"--pod-template-annotation", "foo=baz",
			},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobSetTemplate("jobSet", metav1.NamespaceDefault).
					Spec(makeExampleJobSetSpec()).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobSetMode, "jobSet").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{jobSetGVK},
			wantLists: []runtime.Object{
				&jobsetapi.JobSetList{
					TypeMeta: metav1.TypeMeta{Kind: "JobSetList", APIVersion: "jobset.x-k8s.io/v1alpha2"},
					Items: []jobsetapi.JobSet{
						*wrappers.MakeJobSet("", metav1.NamespaceDefault).
							GenerateName("profile-jobset-").
							Profile("profile").
							Mode(v1alpha1.JobSetMode).
							Spec(makeExampleJobSetSpec()).
							PodTemplateLabel("foo", "bar").
							PodTemplateAnnotation("foo", "baz").
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarNameUserID, Value: userID}).
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarTaskName, Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{
								Name:  constants.EnvVarTaskID,
								Value: fmt.Sprintf("%s_%s_default_profile", userID, testStartTime.Format(time.RFC3339)),
							}).
							WithEnvVar(corev1.EnvVar{Name: "PROFILE", Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{Name: "TIMESTAMP", Value: testStartTime.Format(time.RFC3339)}).
							Obj(),
					},
				},
			},
			wantOut: "jobset.jobset.x-k8s.io/<unknown> created\n",
		},
		"should create jobSet with client dry run": {
			args: []string{"jobset", "--profile", "profile", "--dry-run", "client"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobSetTemplate("jobSet", metav1.NamespaceDefault).Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobSetMode, "jobSet").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{jobSetGVK},
			wantLists: []runtime.Object{
				&jobsetapi.JobSetList{
					TypeMeta: metav1.TypeMeta{Kind: "JobSetList", APIVersion: "jobset.x-k8s.io/v1alpha2"},
					Items:    []jobsetapi.JobSet{},
				},
			},
			wantOut: "jobset.jobset.x-k8s.io/<unknown> created (client dry run)\n",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			streams, _, out, outErr := genericiooptions.NewTestIOStreams()

			scheme := runtime.NewScheme()
			utilruntime.Must(k8sscheme.AddToScheme(scheme))
			utilruntime.Must(jobsetapi.AddToScheme(scheme))

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
