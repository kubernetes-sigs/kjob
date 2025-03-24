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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
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
	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	kueuefake "sigs.k8s.io/kueue/client-go/clientset/versioned/fake"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	kjobctlfake "sigs.k8s.io/kjob/client-go/clientset/versioned/fake"
	cmdtesting "sigs.k8s.io/kjob/pkg/cmd/testing"
	"sigs.k8s.io/kjob/pkg/constants"
	"sigs.k8s.io/kjob/pkg/testing/wrappers"
)

func TestCreateJobCmd(t *testing.T) {
	testStartTime := time.Now()
	userID := os.Getenv(constants.SystemEnvVarNameUser)

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
		"should create job": {
			args: []string{"job", "--profile", "profile"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items: []batchv1.Job{
						*wrappers.MakeJob("", metav1.NamespaceDefault).
							GenerateName("profile-job-").
							Profile("profile").
							Mode(v1alpha1.JobMode).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created\n",
		},
		"should create job with pod template label and annotation": {
			args: []string{
				"job",
				"--profile", "profile",
				"--pod-template-label", "foo=bar",
				"--pod-template-annotation", "foo=baz",
			},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items: []batchv1.Job{
						*wrappers.MakeJob("", metav1.NamespaceDefault).
							GenerateName("profile-job-").
							Profile("profile").
							Mode(v1alpha1.JobMode).
							PodTemplateLabel("foo", "bar").
							PodTemplateAnnotation("foo", "baz").
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created\n",
		},
		"should create job with client dry run": {
			args: []string{"job", "--profile", "profile", "--dry-run", "client"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items:    []batchv1.Job{},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created (client dry run)\n",
		},
		"should create job with short profile flag": {
			args: []string{"job", "-p", "profile"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items: []batchv1.Job{
						*wrappers.MakeJob("", metav1.NamespaceDefault).
							GenerateName("profile-job-").
							Profile("profile").
							Mode(v1alpha1.JobMode).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created\n",
		},
		"should create job with localqueue replacement": {
			args: []string{"job", "--profile", "profile", "--localqueue", "lq1"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
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
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items: []batchv1.Job{
						*wrappers.MakeJob("", metav1.NamespaceDefault).
							GenerateName("profile-job-").
							Profile("profile").
							Mode(v1alpha1.JobMode).
							LocalQueue("lq1").
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created\n",
		},
		"should create job with localqueue and skip local queue validation": {
			args: []string{"job", "--profile", "profile", "--localqueue", "lq1", "--skip-localqueue-validation"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items: []batchv1.Job{
						*wrappers.MakeJob("", metav1.NamespaceDefault).
							GenerateName("profile-job-").
							Profile("profile").
							Mode(v1alpha1.JobMode).
							LocalQueue("lq1").
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created\n",
		},
		"should create job with parallelism replacement": {
			args: []string{"job", "--profile", "profile", "--parallelism", "5"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).
					Parallelism(1).
					Completions(1).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items: []batchv1.Job{
						*wrappers.MakeJob("", metav1.NamespaceDefault).
							GenerateName("profile-job-").
							Profile("profile").
							Mode(v1alpha1.JobMode).
							Parallelism(5).
							Completions(1).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created\n",
		},
		"should create job with --priority flag": {
			args: []string{"job", "--profile", "profile", "--priority", "sample-priority"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
					Obj(),
			},
			kueueObjs: []runtime.Object{
				&kueue.WorkloadPriorityClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sample-priority",
					},
				},
			},
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items: []batchv1.Job{
						*wrappers.MakeJob("", metav1.NamespaceDefault).
							Priority("sample-priority").
							GenerateName("profile-job-").
							Profile("profile").
							Mode(v1alpha1.JobMode).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created\n",
		},
		"should create job with --priority flag and skip workload priority class validation": {
			args: []string{"job", "--profile", "profile", "--skip-priority-validation", "--priority", "sample-priority"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items: []batchv1.Job{
						*wrappers.MakeJob("", metav1.NamespaceDefault).
							Priority("sample-priority").
							GenerateName("profile-job-").
							Profile("profile").
							Mode(v1alpha1.JobMode).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created\n",
		},
		"should create job with completions replacement": {
			args: []string{"job", "--profile", "profile", "--completions", "5"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).
					Parallelism(1).
					Completions(1).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items: []batchv1.Job{
						*wrappers.MakeJob("", metav1.NamespaceDefault).
							GenerateName("profile-job-").
							Profile("profile").
							Mode(v1alpha1.JobMode).
							Parallelism(1).
							Completions(5).
							Obj(),
					},
				},
			},
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created\n",
		},
		"should create job with command replacement": {
			args: []string{"job", "--profile", "profile", "--cmd", "sleep 15s"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).
					Parallelism(1).
					Completions(1).
					WithContainer(*wrappers.MakeContainer("c1", "sleep").Obj()).
					WithContainer(*wrappers.MakeContainer("c2", "sleep").Obj()).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items: []batchv1.Job{
						*wrappers.MakeJob("", metav1.NamespaceDefault).
							GenerateName("profile-job-").
							Profile("profile").
							Mode(v1alpha1.JobMode).
							Parallelism(1).
							Completions(1).
							WithContainer(*wrappers.MakeContainer("c1", "sleep").Command("sleep", "15s").Obj()).
							WithContainer(*wrappers.MakeContainer("c2", "sleep").Obj()).
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
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created\n",
		},
		"should create job with request replacement": {
			args: []string{"job", "--profile", "profile", "--request", "cpu=100m,ram=3Gi"},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeJobTemplate("job-template", metav1.NamespaceDefault).
					Parallelism(1).
					Completions(1).
					WithContainer(*wrappers.MakeContainer("c1", "sleep").Obj()).
					WithContainer(*wrappers.MakeContainer("c2", "sleep").Obj()).
					Obj(),
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobMode, "job-template").Obj()).
					Obj(),
			},
			gvks: []schema.GroupVersionKind{{Group: "batch", Version: "v1", Kind: "Job"}},
			wantLists: []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items: []batchv1.Job{
						*wrappers.MakeJob("", metav1.NamespaceDefault).
							GenerateName("profile-job-").
							Profile("profile").
							Mode(v1alpha1.JobMode).
							Parallelism(1).
							Completions(1).
							WithContainer(
								*wrappers.MakeContainer("c1", "sleep").
									WithRequest("cpu", resource.MustParse("100m")).
									WithRequest("ram", resource.MustParse("3Gi")).
									Obj(),
							).
							WithContainer(*wrappers.MakeContainer("c2", "sleep").Obj()).
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
			// Fake dynamic client not generating name. That's why we have <unknown>.
			wantOut: "job.batch/<unknown> created\n",
		},
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
					List(t.Context(), metav1.ListOptions{})
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
