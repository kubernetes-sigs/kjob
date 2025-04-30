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
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
	"k8s.io/utils/ptr"
	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	kueuefake "sigs.k8s.io/kueue/client-go/clientset/versioned/fake"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	kjobctlfake "sigs.k8s.io/kjob/client-go/clientset/versioned/fake"
	cmdtesting "sigs.k8s.io/kjob/pkg/cmd/testing"
	"sigs.k8s.io/kjob/pkg/constants"
	"sigs.k8s.io/kjob/pkg/testing/wrappers"
)

type createSlurmCmdTestCase struct {
	beforeTest     func(t *testing.T, tc *createSlurmCmdTestCase)
	tempFile       string
	args           func(tc *createSlurmCmdTestCase) []string
	kjobctlObjs    []runtime.Object
	kueueObjs      []runtime.Object
	wantJobs       []batchv1.Job
	wantConfigMaps []corev1.ConfigMap
	wantServices   []corev1.Service
	cmpOpts        []cmp.Option
	wantOut        string
	wantOutPattern string
	wantOutErr     string
	wantErr        string
}

func beforeSlurmTest(t *testing.T, tc *createSlurmCmdTestCase) {
	file, err := os.CreateTemp("", "slurm")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	t.Cleanup(func() {
		if err := os.Remove(tc.tempFile); err != nil {
			t.Fatal(err)
		}
	})

	if _, err := file.WriteString("#!/bin/bash\nsrun sleep 300'"); err != nil {
		t.Fatal(err)
	}

	tc.tempFile = file.Name()
}

func TestCreateSlurmCmd(t *testing.T) {
	const (
		baseImage              = "bash:4.4"
		applicationProfileName = "profile"
	)

	testStartTime := time.Now()
	userID := os.Getenv(constants.SystemEnvVarNameUser)

	wantOwnerReference := metav1.OwnerReference{
		Name:       "profile-slurm",
		APIVersion: "batch/v1",
		Kind:       "Job",
	}

	baseContainerWrapper := *wrappers.MakeContainer("c1", baseImage)

	baseContainerWrapperWithEnv := baseContainerWrapper.Clone().
		WithEnvVar(corev1.EnvVar{Name: constants.EnvVarNameUserID, Value: userID}).
		WithEnvVar(corev1.EnvVar{Name: constants.EnvVarTaskName, Value: "default_profile"}).
		WithEnvVar(corev1.EnvVar{
			Name:  constants.EnvVarTaskID,
			Value: fmt.Sprintf("%s_%s_default_profile", userID, testStartTime.Format(time.RFC3339)),
		}).
		WithEnvVar(corev1.EnvVar{Name: "PROFILE", Value: "default_profile"}).
		WithEnvVar(corev1.EnvVar{Name: "TIMESTAMP", Value: testStartTime.Format(time.RFC3339)})

	baseJobTemplateWrapper := wrappers.MakeJobTemplate("slurm-job-template", metav1.NamespaceDefault).
		WithContainer(*baseContainerWrapper.DeepCopy())

	baseJobTemplateWrapperWithTwoContainers := baseJobTemplateWrapper.Clone().
		WithContainer(*baseContainerWrapper.Clone().Name("c2").Obj())

	baseApplicationProfileWrapper := wrappers.MakeApplicationProfile(applicationProfileName, metav1.NamespaceDefault).
		WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.SlurmMode, "slurm-job-template").Obj()).
		Obj()

	baseKjobctlObjs := []runtime.Object{
		baseJobTemplateWrapper.DeepCopy(),
		baseApplicationProfileWrapper.DeepCopy(),
	}

	baseJobInitContainerWrapper := *wrappers.MakeContainer("slurm-init-env", "registry.k8s.io/busybox:1.27.2").
		Command("sh", "/slurm/scripts/init-entrypoint.sh").
		WithVolumeMount(corev1.VolumeMount{Name: "slurm-scripts", MountPath: "/slurm/scripts"}).
		WithVolumeMount(corev1.VolumeMount{Name: "slurm-env", MountPath: "/slurm/env"}).
		WithEnvVar(corev1.EnvVar{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
		}})

	baseJobContainerWrapper := baseContainerWrapperWithEnv.Clone().
		Command("bash", "/slurm/scripts/entrypoint.sh").
		WithVolumeMount(corev1.VolumeMount{Name: "slurm-scripts", MountPath: "/slurm/scripts"}).
		WithVolumeMount(corev1.VolumeMount{Name: "slurm-env", MountPath: "/slurm/env"})

	baseJobWrapper := wrappers.MakeJob("profile-slurm", metav1.NamespaceDefault).
		Completions(1).
		CompletionMode(batchv1.IndexedCompletion).
		Profile(applicationProfileName).
		Mode(v1alpha1.SlurmMode).
		Subdomain("profile-slurm").
		WithInitContainer(*baseJobInitContainerWrapper.DeepCopy()).
		WithContainer(*baseJobContainerWrapper.DeepCopy()).
		WithEnvVarIndexValue("JOB_CONTAINER_INDEX").
		WithVolume(corev1.Volume{
			Name: "slurm-scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "profile-slurm"},
					Items: []corev1.KeyToPath{
						{Key: "init-entrypoint.sh", Path: "init-entrypoint.sh"},
						{Key: "entrypoint.sh", Path: "entrypoint.sh"},
						{Key: "script", Path: "script", Mode: ptr.To[int32](0755)},
					},
				},
			},
		}).
		WithVolume(corev1.Volume{
			Name: "slurm-env",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

	baseConfigMapWrapper := wrappers.MakeConfigMap("profile-slurm", metav1.NamespaceDefault).
		WithOwnerReference(wantOwnerReference).
		Profile(applicationProfileName).
		Mode(v1alpha1.SlurmMode).
		Data(map[string]string{
			"script": "#!/bin/bash\nsleep 300'",
			"init-entrypoint.sh": `#!/bin/sh

set -o errexit
set -o nounset
set -o pipefail
set -x

# External variables
# JOB_COMPLETION_INDEX - completion index of the job.
# POD_IP               - current pod IP

array_indexes="0"
container_indexes=$(echo "$array_indexes" | awk -F';' -v idx="$JOB_COMPLETION_INDEX" '{print $((idx + 1))}')

for i in $(seq 0 1)
do
  container_index=$(echo "$container_indexes" | awk -F',' -v idx="$i" '{print $((idx + 1))}')

	if [ -z "$container_index" ]; then
		break
	fi

	mkdir -p /slurm/env/$i


	cat << EOF > /slurm/env/$i/slurm.env
SLURM_ARRAY_JOB_ID=1
SLURM_ARRAY_TASK_COUNT=1
SLURM_ARRAY_TASK_MAX=0
SLURM_ARRAY_TASK_MIN=0
SLURM_TASKS_PER_NODE=1
SLURM_CPUS_PER_TASK=
SLURM_CPUS_ON_NODE=
SLURM_JOB_CPUS_PER_NODE=
SLURM_CPUS_PER_GPU=
SLURM_MEM_PER_CPU=
SLURM_MEM_PER_GPU=
SLURM_MEM_PER_NODE=
SLURM_GPUS=
SLURM_NTASKS=1
SLURM_NTASKS_PER_NODE=1
SLURM_NPROCS=1
SLURM_NNODES=1
SLURM_SUBMIT_DIR=/slurm/scripts
SLURM_SUBMIT_HOST=$HOSTNAME
SLURM_JOB_NODELIST=profile-slurm-0.profile-slurm
SLURM_JOB_FIRST_NODE=profile-slurm-0.profile-slurm
SLURM_JOB_ID=$(expr $JOB_COMPLETION_INDEX \* 1 + $i + 1)
SLURM_JOBID=$(expr $JOB_COMPLETION_INDEX \* 1 + $i + 1)
SLURM_ARRAY_TASK_ID=$container_index
SLURM_JOB_FIRST_NODE_IP=${SLURM_JOB_FIRST_NODE_IP:-""}
EOF

done
`,
			"entrypoint.sh": `#!/usr/local/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# External variables
# JOB_CONTAINER_INDEX 	- container index in the container template.

if [ ! -d "/slurm/env/$JOB_CONTAINER_INDEX" ]; then
	exit 0
fi

SBATCH_JOB_NAME=

export $(cat /slurm/env/$JOB_CONTAINER_INDEX/slurm.env | xargs)

/slurm/scripts/script
`,
		})

	baseServiceWrapper := wrappers.MakeService("profile-slurm", metav1.NamespaceDefault).
		Profile(applicationProfileName).
		Mode(v1alpha1.SlurmMode).
		ClusterIP("None").
		Selector("job-name", "profile-slurm").
		WithOwnerReference(wantOwnerReference)

	cmpOpts := []cmp.Option{
		cmpopts.AcyclicTransformer("RemoveGeneratedNameSuffixInString", func(val string) string {
			return regexp.MustCompile("(profile-slurm)(-.{5})").ReplaceAllString(val, "$1")
		}),
		cmpopts.AcyclicTransformer("RemoveGeneratedNameSuffixInMap", func(m map[string]string) map[string]string {
			for key, val := range m {
				m[key] = regexp.MustCompile("(profile-slurm)(-.{5})").ReplaceAllString(val, "$1")
			}
			return m
		}),
	}

	cmpOptsIgnoreConfigMapAndService := append(
		cmpOpts,
		cmpopts.IgnoreTypes(corev1.ConfigMapList{}),
		cmpopts.IgnoreTypes(corev1.ServiceList{}),
	)

	wantOutPatternSuccess := "^job\\.batch\\/.+ created\\\\nconfigmap\\/.+ created\\\\nservice\\/.+ created\\\\n$"

	testCases := map[string]createSlurmCmdTestCase{
		"shouldn't create slurm because slurm args must be specified": {
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName}
			},
			wantErr: "requires at least 1 arg(s), only received 0",
		},
		"shouldn't create slurm because script must be specified on slurm args": {
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "./script.sh"}
			},
			wantErr: "unknown command \"./script.sh\" for \"create slurm\"",
		},
		"shouldn't create slurm because script must be specified": {
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--", "--array", "0-5"}
			},
			wantErr: "must specify script",
		},
		"shouldn't create slurm because script only one script must be specified": {
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--", "./script.sh", "./script.sh"}
			},
			wantErr: "must specify only one script",
		},
		"shouldn't create slurm because the wait-timeout flag requires the wait flag to be set": {
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--wait-timeout", "5s", "--", tc.tempFile}
			},
			wantErr: "the --wait-timeout flag is required when --wait is set",
		},
		"shouldn't create slurm because the stream-container is used without wait": {
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--stream-container", "foo", "--", tc.tempFile}
			},
			wantErr: "the --stream-container can only be specified for streaming output",
		},
		"should create slurm": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--", tc.tempFile}
			},
			kjobctlObjs:    baseKjobctlObjs,
			wantJobs:       []batchv1.Job{*baseJobWrapper.DeepCopy()},
			wantConfigMaps: []corev1.ConfigMap{*baseConfigMapWrapper.DeepCopy()},
			wantServices:   []corev1.Service{*baseServiceWrapper.DeepCopy()},
			cmpOpts:        cmpOpts,
			wantOutPattern: wantOutPatternSuccess,
		},
		"should create slurm with flags": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{
					"slurm",
					"--profile", applicationProfileName,
					"--localqueue", "lq1",
					"--init-image", "bash:latest",
					"--first-node-ip",
					"--first-node-ip-timeout", "29s",
					"--pod-template-label", "foo=bar",
					"--pod-template-annotation", "foo=baz",
					"--",
					"--array", "0-25",
					"--nodes", "2",
					"--ntasks", "3",
					"--input", "\\\\/home/%u/%x/stderr%%-%A-%a-%j-%N-%n-%t.out",
					"--output", "/home/%u/%x/stdout%%-%A-%a-%j-%N-%n-%t.out",
					"--error", "/home/%u/%x/stderr%%-%A-%a-%j-%N-%n-%t.out",
					"--job-name", "job-name",
					"--partition", "lq1",
					"--chdir", "/mydir",
					"--cpus-per-task", "2",
					tc.tempFile,
				}
			},
			kjobctlObjs: []runtime.Object{
				baseJobTemplateWrapperWithTwoContainers.DeepCopy(),
				baseApplicationProfileWrapper.DeepCopy(),
			},
			kueueObjs: []runtime.Object{
				&kueue.LocalQueue{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: metav1.NamespaceDefault,
						Name:      "lq1",
					},
				},
			},
			wantJobs: []batchv1.Job{
				*baseJobWrapper.Clone().
					Parallelism(2).
					Completions(9).
					LocalQueue("lq1").
					PodTemplateLabel("foo", "bar").
					PodTemplateAnnotation("foo", "baz").
					InitContainers(*baseJobInitContainerWrapper.Clone().Image("bash:latest").Obj()).
					Containers(
						*baseJobContainerWrapper.Clone().Name("c1-0").
							WithRequest(corev1.ResourceCPU, resource.MustParse("2")).
							Obj(),
						*baseJobContainerWrapper.Clone().Name("c2").
							WithRequest(corev1.ResourceCPU, resource.MustParse("2")).
							Obj(),
						*baseJobContainerWrapper.Clone().Name("c1-1").
							WithRequest(corev1.ResourceCPU, resource.MustParse("2")).
							Obj(),
						*baseJobContainerWrapper.Clone().Name("c1-2").
							WithRequest(corev1.ResourceCPU, resource.MustParse("2")).
							Obj(),
					).
					WithEnvVarIndexValue("JOB_CONTAINER_INDEX").
					Obj(),
			},
			wantConfigMaps: []corev1.ConfigMap{
				*baseConfigMapWrapper.Clone().
					DataValue("init-entrypoint.sh", `#!/bin/sh

set -o errexit
set -o nounset
set -o pipefail
set -x

# External variables
# JOB_COMPLETION_INDEX - completion index of the job.
# POD_IP               - current pod IP

array_indexes="0,1,2;3,4,5;6,7,8;9,10,11;12,13,14;15,16,17;18,19,20;21,22,23;24,25"
container_indexes=$(echo "$array_indexes" | awk -F';' -v idx="$JOB_COMPLETION_INDEX" '{print $((idx + 1))}')

for i in $(seq 0 3)
do
  container_index=$(echo "$container_indexes" | awk -F',' -v idx="$i" '{print $((idx + 1))}')

	if [ -z "$container_index" ]; then
		break
	fi

	mkdir -p /slurm/env/$i


  if [[ "$JOB_COMPLETION_INDEX" -eq 0 ]]; then
    SLURM_JOB_FIRST_NODE_IP=${POD_IP}
  else
    timeout=29
    start_time=$(date +%s)
    while true; do
      ip=$(nslookup "profile-slurm-r8njg-0.profile-slurm-r8njg" | grep "Address 1" | awk 'NR==2 {print $3}') || true
      if [[ -n "$ip" ]]; then
        SLURM_JOB_FIRST_NODE_IP=$ip
        break
      else
        current_time=$(date +%s)
        elapsed_time=$((current_time - start_time))
        if [ "$elapsed_time" -ge "$timeout" ]; then
          echo "Timeout reached, IP address for the first node (profile-slurm-r8njg-0.profile-slurm-r8njg) not found."
          break
        fi
        echo "IP Address for the first node (profile-slurm-r8njg-0.profile-slurm-r8njg) not found, retrying..."
        sleep 1
      fi
    done
  fi

	cat << EOF > /slurm/env/$i/slurm.env
SLURM_ARRAY_JOB_ID=1
SLURM_ARRAY_TASK_COUNT=26
SLURM_ARRAY_TASK_MAX=25
SLURM_ARRAY_TASK_MIN=0
SLURM_TASKS_PER_NODE=3
SLURM_CPUS_PER_TASK=2
SLURM_CPUS_ON_NODE=8
SLURM_JOB_CPUS_PER_NODE=8
SLURM_CPUS_PER_GPU=
SLURM_MEM_PER_CPU=
SLURM_MEM_PER_GPU=
SLURM_MEM_PER_NODE=
SLURM_GPUS=
SLURM_NTASKS=3
SLURM_NTASKS_PER_NODE=3
SLURM_NPROCS=3
SLURM_NNODES=2
SLURM_SUBMIT_DIR=/slurm/scripts
SLURM_SUBMIT_HOST=$HOSTNAME
SLURM_JOB_NODELIST=profile-slurm-fpxnj-0.profile-slurm-fpxnj,profile-slurm-fpxnj-1.profile-slurm-fpxnj
SLURM_JOB_FIRST_NODE=profile-slurm-fpxnj-0.profile-slurm-fpxnj
SLURM_JOB_ID=$(expr $JOB_COMPLETION_INDEX \* 3 + $i + 1)
SLURM_JOBID=$(expr $JOB_COMPLETION_INDEX \* 3 + $i + 1)
SLURM_ARRAY_TASK_ID=$container_index
SLURM_JOB_FIRST_NODE_IP=${SLURM_JOB_FIRST_NODE_IP:-""}
EOF

done
`).
					DataValue("entrypoint.sh", `#!/usr/local/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# External variables
# JOB_CONTAINER_INDEX 	- container index in the container template.

if [ ! -d "/slurm/env/$JOB_CONTAINER_INDEX" ]; then
	exit 0
fi

SBATCH_JOB_NAME=job-name

export $(cat /slurm/env/$JOB_CONTAINER_INDEX/slurm.env | xargs)cd /mydir

/slurm/scripts/script </home/%u/%x/stderr%%-%A-%a-%j-%N-%n-%t.out 1> >(tee /home/${USER_ID}/${SBATCH_JOB_NAME}/stdout%-${SLURM_ARRAY_JOB_ID}-${SLURM_ARRAY_TASK_ID}-${SLURM_JOB_ID}-${HOSTNAME}-${JOB_COMPLETION_INDEX}-${SLURM_ARRAY_TASK_ID}.out) 2> >(tee /home/${USER_ID}/${SBATCH_JOB_NAME}/stderr%-${SLURM_ARRAY_JOB_ID}-${SLURM_ARRAY_TASK_ID}-${SLURM_JOB_ID}-${HOSTNAME}-${JOB_COMPLETION_INDEX}-${SLURM_ARRAY_TASK_ID}.out >&2)
`).
					Obj(),
			},
			wantServices:   []corev1.Service{*baseServiceWrapper.DeepCopy()},
			cmpOpts:        cmpOpts,
			wantOutPattern: wantOutPatternSuccess,
		},
		"should create slurm with --ntasks flag": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--", "--ntasks", "3", tc.tempFile}
			},
			kjobctlObjs: baseKjobctlObjs,
			wantJobs: []batchv1.Job{
				*baseJobWrapper.Clone().
					Containers(
						*baseJobContainerWrapper.Clone().Name("c1-0").Obj(),
						*baseJobContainerWrapper.Clone().Name("c1-1").Obj(),
						*baseJobContainerWrapper.Clone().Name("c1-2").Obj(),
					).
					WithEnvVarIndexValue("JOB_CONTAINER_INDEX").
					Obj(),
			},
			cmpOpts:        cmpOptsIgnoreConfigMapAndService,
			wantOutPattern: wantOutPatternSuccess,
		},
		"should divide --mem exactly across containers": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--", "--mem", "2G", tc.tempFile}
			},
			kjobctlObjs: []runtime.Object{
				baseJobTemplateWrapperWithTwoContainers.DeepCopy(),
				baseApplicationProfileWrapper.DeepCopy(),
			},
			wantJobs: []batchv1.Job{
				*baseJobWrapper.Clone().
					Containers(
						*baseJobContainerWrapper.Clone().Name("c1").
							WithResources(corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1G"),
								},
							}).
							Obj(),
						*baseJobContainerWrapper.Clone().Name("c2").
							WithResources(corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1G"),
								},
							}).
							Obj(),
					).
					WithEnvVarIndexValue("JOB_CONTAINER_INDEX").
					Obj(),
			},
			cmpOpts:        cmpOptsIgnoreConfigMapAndService,
			wantOutPattern: wantOutPatternSuccess,
		},
		"should handle non-exact --mem division across containers": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--", "--mem", "1G", tc.tempFile}
			},
			kjobctlObjs: []runtime.Object{
				baseJobTemplateWrapperWithTwoContainers.DeepCopy(),
				baseApplicationProfileWrapper.DeepCopy(),
			},
			wantJobs: []batchv1.Job{
				*baseJobWrapper.Clone().
					Containers(
						*baseJobContainerWrapper.Clone().Name("c1").
							WithResources(corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("500M"),
								},
							}).
							Obj(),
						*baseJobContainerWrapper.Clone().Name("c2").
							WithResources(corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("500M"),
								},
							}).
							Obj(),
					).
					WithEnvVarIndexValue("JOB_CONTAINER_INDEX").
					Obj(),
			},
			cmpOpts:        cmpOptsIgnoreConfigMapAndService,
			wantOutPattern: wantOutPatternSuccess,
		},
		"should create slurm with --mem-per-cpu flag": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--", "--cpus-per-task", "2", "--mem-per-cpu", "500M", tc.tempFile}
			},
			kjobctlObjs: []runtime.Object{
				baseJobTemplateWrapperWithTwoContainers.DeepCopy(),
				baseApplicationProfileWrapper.DeepCopy(),
			},
			wantJobs: []batchv1.Job{
				*baseJobWrapper.Clone().
					Containers(
						*baseJobContainerWrapper.Clone().Name("c1").
							WithResources(corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("1G"),
								},
							}).
							Obj(),
						*baseJobContainerWrapper.Clone().Name("c2").
							WithResources(corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("1G"),
								},
							}).
							Obj(),
					).
					WithEnvVarIndexValue("JOB_CONTAINER_INDEX").
					Obj(),
			},
			cmpOpts:        cmpOptsIgnoreConfigMapAndService,
			wantOutPattern: wantOutPatternSuccess,
		},
		"shouldn't create slurm with --mem-per-cpu flag because --cpus-per-task flag not specified": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--", "--mem-per-cpu", "500M", tc.tempFile}
			},
			kjobctlObjs: baseKjobctlObjs,
			wantErr:     "no cpus-per-task specified",
		},
		"should create slurm with --mem-per-gpu flag": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{
					"slurm",
					"--profile", applicationProfileName,
					"--",
					"--gpus-per-task", "volta:3,kepler:1",
					"--mem-per-gpu", "500M",
					tc.tempFile,
				}
			},
			kjobctlObjs: []runtime.Object{
				baseJobTemplateWrapperWithTwoContainers.DeepCopy(),
				baseApplicationProfileWrapper.DeepCopy(),
			},
			wantJobs: []batchv1.Job{
				*baseJobWrapper.Clone().
					Containers(
						*baseJobContainerWrapper.Clone().Name("c1").
							WithResources(corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("2G"),
								},
								Limits: corev1.ResourceList{
									"volta":  resource.MustParse("3"),
									"kepler": resource.MustParse("1"),
								},
							}).
							Obj(),
						*baseJobContainerWrapper.Clone().Name("c2").
							WithResources(corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("2G"),
								},
								Limits: corev1.ResourceList{
									"volta":  resource.MustParse("3"),
									"kepler": resource.MustParse("1"),
								},
							}).
							Obj(),
					).
					WithEnvVarIndexValue("JOB_CONTAINER_INDEX").
					Obj(),
			},
			cmpOpts:        cmpOptsIgnoreConfigMapAndService,
			wantOutPattern: wantOutPatternSuccess,
		},
		"shouldn't create slurm with --mem-per-gpu flag because --gpus-per-task flag not specified": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--", "--mem-per-gpu", "500M", tc.tempFile}
			},
			kjobctlObjs: baseKjobctlObjs,
			wantErr:     "no gpus-per-task specified",
		},
		"should create slurm with --priority flag": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--priority", "sample-priority", "--", tc.tempFile}
			},
			kjobctlObjs: baseKjobctlObjs,
			kueueObjs: []runtime.Object{
				&kueue.WorkloadPriorityClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sample-priority",
					},
				},
			},
			wantJobs:       []batchv1.Job{*baseJobWrapper.Clone().Priority("sample-priority").Obj()},
			wantConfigMaps: []corev1.ConfigMap{*baseConfigMapWrapper.DeepCopy()},
			wantServices:   []corev1.Service{*baseServiceWrapper.DeepCopy()},
			cmpOpts:        cmpOpts,
			wantOutPattern: wantOutPatternSuccess,
		},
		"should create slurm with --priority flag and skip workload priority class validation": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{
					"slurm",
					"--profile", applicationProfileName,
					"--priority", "sample-priority",
					"--skip-priority-validation",
					"--",
					tc.tempFile,
				}
			},
			kjobctlObjs:    baseKjobctlObjs,
			wantJobs:       []batchv1.Job{*baseJobWrapper.Clone().Priority("sample-priority").Obj()},
			wantConfigMaps: []corev1.ConfigMap{*baseConfigMapWrapper.DeepCopy()},
			wantServices:   []corev1.Service{*baseServiceWrapper.DeepCopy()},
			cmpOpts:        cmpOpts,
			wantOutPattern: wantOutPatternSuccess,
		},
		"should create slurm with --time flag": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--", tc.tempFile, "--time", "1"}
			},
			kjobctlObjs:    baseKjobctlObjs,
			wantJobs:       []batchv1.Job{*baseJobWrapper.Clone().MaxExecTimeSecondsLabel("60").Obj()},
			wantConfigMaps: []corev1.ConfigMap{*baseConfigMapWrapper.DeepCopy()},
			wantServices:   []corev1.Service{*baseServiceWrapper.DeepCopy()},
			cmpOpts:        cmpOpts,
			wantOutPattern: wantOutPatternSuccess,
		},
		"should create slurm with -t flag": {
			beforeTest: beforeSlurmTest,
			args: func(tc *createSlurmCmdTestCase) []string {
				return []string{"slurm", "--profile", applicationProfileName, "--", tc.tempFile, "-t", "2-12:05:23"}
			},
			kjobctlObjs:    baseKjobctlObjs,
			wantJobs:       []batchv1.Job{*baseJobWrapper.Clone().MaxExecTimeSecondsLabel("216323").Obj()},
			wantConfigMaps: []corev1.ConfigMap{*baseConfigMapWrapper.DeepCopy()},
			wantServices:   []corev1.Service{*baseServiceWrapper.DeepCopy()},
			cmpOpts:        cmpOpts,
			wantOutPattern: wantOutPatternSuccess,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if tc.beforeTest != nil {
				tc.beforeTest(t, &tc)
			}

			streams, _, out, outErr := genericiooptions.NewTestIOStreams()

			scheme := runtime.NewScheme()
			utilruntime.Must(k8sscheme.AddToScheme(scheme))
			utilruntime.Must(rayv1.AddToScheme(scheme))

			clientset := kjobctlfake.NewSimpleClientset(tc.kjobctlObjs...)
			dynamicClient := fake.NewSimpleDynamicClient(scheme)
			kueueClientset := kueuefake.NewSimpleClientset(tc.kueueObjs...)
			restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})

			gvks := []schema.GroupVersionKind{
				{Group: "batch", Version: "v1", Kind: "Job"},
				{Group: "", Version: "v1", Kind: "ConfigMap"},
				{Group: "", Version: "v1", Kind: "Service"},
			}

			for _, gvk := range gvks {
				restMapper.Add(gvk, meta.RESTScopeNamespace)
			}

			tcg := cmdtesting.NewTestClientGetter().
				WithKjobctlClientset(clientset).
				WithDynamicClient(dynamicClient).
				WithKueueClientset(kueueClientset).
				WithRESTMapper(restMapper)

			cmd := NewCreateCmd(tcg, streams, clocktesting.NewFakeClock(testStartTime))
			cmd.SetOut(out)
			cmd.SetErr(outErr)
			cmd.SetArgs(tc.args(&tc))

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
			if tc.wantOutPattern != "" {
				gotOut = strings.ReplaceAll(gotOut, "\n", "\\n")
				match, err := regexp.MatchString(tc.wantOutPattern, gotOut)
				if err != nil {
					t.Error(err)
					return
				}
				if !match {
					t.Errorf("Unexpected output. Not match pattern \"%s\":\n%s", tc.wantOutPattern, gotOut)
				}
			} else if diff := cmp.Diff(tc.wantOut, gotOut); diff != "" {
				t.Errorf("Unexpected output (-want/+got)\n%s", diff)
			}

			gotOutErr := outErr.String()
			if diff := cmp.Diff(tc.wantOutErr, gotOutErr); diff != "" {
				t.Errorf("Unexpected output (-want/+got)\n%s", diff)
			}

			wantLists := []runtime.Object{
				&batchv1.JobList{
					TypeMeta: metav1.TypeMeta{Kind: "JobList", APIVersion: "batch/v1"},
					Items:    tc.wantJobs,
				},
				&corev1.ConfigMapList{
					TypeMeta: metav1.TypeMeta{Kind: "ConfigMapList", APIVersion: "v1"},
					Items:    tc.wantConfigMaps,
				},
				&corev1.ServiceList{
					TypeMeta: metav1.TypeMeta{Kind: "ServiceList", APIVersion: "v1"},
					Items:    tc.wantServices,
				},
			}

			for index, gvk := range gvks {
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

				gotList := wantLists[index].DeepCopyObject()

				err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), gotList)
				if err != nil {
					t.Error(err)
					return
				}

				if job, ok := wantLists[index].(*batchv1.JobList); ok && len(job.Items) > 0 {
					if tc.tempFile != "" {
						if job.Items[0].Annotations == nil {
							job.Items[0].Annotations = make(map[string]string)
						}
						job.Items[0].Annotations[constants.ScriptAnnotation] = tc.tempFile
					}
				}

				if diff := cmp.Diff(wantLists[index], gotList, tc.cmpOpts...); diff != "" {
					t.Errorf("Unexpected list for %s (-want/+got)\n%s", gvk.String(), diff)
				}
			}
		})
	}
}
