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

package builder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	kjobctlfake "sigs.k8s.io/kjob/client-go/clientset/versioned/fake"
	cmdtesting "sigs.k8s.io/kjob/pkg/cmd/testing"
	"sigs.k8s.io/kjob/pkg/constants"
	"sigs.k8s.io/kjob/pkg/testing/wrappers"
	"sigs.k8s.io/kjob/pkg/util/validate"
)

type slurmBuilderTestCase struct {
	beforeTest       func(t *testing.T, tc *slurmBuilderTestCase)
	tempFile         string
	profile          string
	array            string
	cpusPerTask      *resource.Quantity
	gpusPerTask      map[string]*resource.Quantity
	memPerTask       *resource.Quantity
	memPerCPU        *resource.Quantity
	memPerGPU        *resource.Quantity
	nodes            *int32
	nTasks           *int32
	output           string
	err              string
	input            string
	jobName          string
	partition        string
	workerContainers []string
	firstNodeTimeout time.Duration
	kjobctlObjs      []runtime.Object
	wantRootObj      runtime.Object
	wantChildObjs    []runtime.Object
	wantErr          error
	cmpopts          []cmp.Option
}

func beforeSlurmTest(t *testing.T, tc *slurmBuilderTestCase) {
	file, err := os.CreateTemp("", "slurm")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	t.Cleanup(func() {
		if err := os.Remove(file.Name()); err != nil {
			t.Fatal(err)
		}
	})

	if _, err := file.WriteString("#!/bin/bash\nsrun sleep 300'"); err != nil {
		t.Fatal(err)
	}

	tc.tempFile = file.Name()
}

func TestSlurmBuilderDo(t *testing.T) {
	const (
		baseImage              = "bash:4.4"
		baseInitContainerImage = "bash:latest"
		applicationProfileName = "profile"
	)

	testStartTime := time.Now()
	userID := os.Getenv(constants.SystemEnvVarNameUser)

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

	baseApplicationProfileWrapper := wrappers.MakeApplicationProfile(applicationProfileName, metav1.NamespaceDefault).
		WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.SlurmMode, "slurm-job-template").Obj())

	baseJobContainerWrapper := baseContainerWrapperWithEnv.Clone().
		Command("bash", "/slurm/scripts/entrypoint.sh").
		WithVolumeMount(corev1.VolumeMount{Name: "slurm-scripts", MountPath: "/slurm/scripts"}).
		WithVolumeMount(corev1.VolumeMount{Name: "slurm-env", MountPath: "/slurm/env"})

	baseJobWrapper := wrappers.MakeJob("", metav1.NamespaceDefault).
		Completions(1).
		CompletionMode(batchv1.IndexedCompletion).
		Profile(applicationProfileName).
		Mode(v1alpha1.SlurmMode).
		Subdomain("profile-slurm").
		WithInitContainer(*wrappers.MakeContainer("slurm-init-env", baseInitContainerImage).
			Command("sh", "/slurm/scripts/init-entrypoint.sh").
			WithVolumeMount(corev1.VolumeMount{Name: "slurm-scripts", MountPath: "/slurm/scripts"}).
			WithVolumeMount(corev1.VolumeMount{Name: "slurm-env", MountPath: "/slurm/env"}).
			WithEnvVar(corev1.EnvVar{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
			}}).
			Obj()).
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

	baseConfigMapWrapper := wrappers.MakeConfigMap("", metav1.NamespaceDefault).
		Profile(applicationProfileName).
		Mode(v1alpha1.SlurmMode).
		Data(map[string]string{
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
SLURM_JOB_NODELIST=profile-slurm-q85rk-0.profile-slurm-q85rk
SLURM_JOB_FIRST_NODE=profile-slurm-q85rk-0.profile-slurm-q85rk
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
			"script": "#!/bin/bash\nsleep 300'",
		})

	baseServiceWrapper := wrappers.MakeService("profile-slurm", metav1.NamespaceDefault).
		Profile(applicationProfileName).
		Mode(v1alpha1.SlurmMode).
		ClusterIP("None").
		Selector("job-name", "profile-slurm")

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

	testCases := map[string]slurmBuilderTestCase{
		"shouldn't build slurm job because script not specified": {
			profile:     applicationProfileName,
			kjobctlObjs: []runtime.Object{baseApplicationProfileWrapper.DeepCopy()},
			wantErr:     errNoScriptSpecified,
		},
		"shouldn't build slurm job with --mem-per-cpu because no --cpus-per-task specified": {
			beforeTest:  beforeSlurmTest,
			profile:     applicationProfileName,
			memPerCPU:   ptr.To(resource.MustParse("1")),
			kjobctlObjs: []runtime.Object{baseApplicationProfileWrapper.DeepCopy()},
			wantErr:     errNoCpusPerTaskSpecified,
		},
		"shouldn't build slurm job with --mem-per-cpu because no --gpus-per-task specified": {
			beforeTest:  beforeSlurmTest,
			profile:     applicationProfileName,
			memPerGPU:   ptr.To(resource.MustParse("1")),
			kjobctlObjs: []runtime.Object{baseApplicationProfileWrapper.DeepCopy()},
			wantErr:     errNoGpusPerTaskSpecified,
		},
		"shouldn't build slurm job because --mem-per-cpu and --mem-per-gpu flags mutually exclusive": {
			beforeTest:  beforeSlurmTest,
			profile:     applicationProfileName,
			cpusPerTask: ptr.To(resource.MustParse("1")),
			memPerCPU:   ptr.To(resource.MustParse("1")),
			gpusPerTask: map[string]*resource.Quantity{
				"test": ptr.To(resource.MustParse("1")),
			},
			memPerGPU:   ptr.To(resource.MustParse("1")),
			kjobctlObjs: []runtime.Object{baseApplicationProfileWrapper.DeepCopy()},
			wantErr: validate.NewMutuallyExclusiveError(
				[]string{
					string(v1alpha1.MemPerNodeFlag),
					string(v1alpha1.MemPerCPUFlag),
					string(v1alpha1.MemPerGPUFlag),
					string(v1alpha1.MemPerTaskFlag),
				},
				[]string{string(v1alpha1.MemPerCPUFlag), string(v1alpha1.MemPerGPUFlag)},
			),
		},
		"shouldn't build slurm job because template not found": {
			beforeTest:  beforeSlurmTest,
			profile:     applicationProfileName,
			kjobctlObjs: []runtime.Object{baseApplicationProfileWrapper.DeepCopy()},
			wantErr:     apierrors.NewNotFound(schema.GroupResource{Group: "kjobctl.x-k8s.io", Resource: "jobtemplates"}, "slurm-job-template"),
		},
		"should build slurm job": {
			beforeTest:    beforeSlurmTest,
			profile:       applicationProfileName,
			kjobctlObjs:   []runtime.Object{baseJobTemplateWrapper.DeepCopy(), baseApplicationProfileWrapper.DeepCopy()},
			wantRootObj:   baseJobWrapper.DeepCopy(),
			wantChildObjs: []runtime.Object{baseConfigMapWrapper.DeepCopy(), baseServiceWrapper.DeepCopy()},
			cmpopts:       cmpOpts,
		},
		"should build slurm job with --worker-container": {
			beforeTest:       beforeSlurmTest,
			profile:          applicationProfileName,
			workerContainers: []string{"c1"},
			kjobctlObjs: []runtime.Object{
				baseJobTemplateWrapper.Clone().
					WithContainer(*wrappers.MakeContainer("c2", baseImage).Obj()).
					Obj(),
				baseApplicationProfileWrapper.DeepCopy(),
			},
			wantRootObj: baseJobWrapper.Clone().
				WithContainer(
					*baseContainerWrapperWithEnv.Clone().
						Name("c2").
						WithEnvVar(corev1.EnvVar{Name: "JOB_CONTAINER_INDEX", Value: "1"}).
						Obj(),
				).
				Obj(),
			wantChildObjs: []runtime.Object{baseConfigMapWrapper.DeepCopy(), baseServiceWrapper.DeepCopy()},
			cmpopts:       cmpOpts,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if tc.beforeTest != nil {
				tc.beforeTest(t, &tc)
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			tcg := cmdtesting.NewTestClientGetter().
				WithKjobctlClientset(kjobctlfake.NewSimpleClientset(tc.kjobctlObjs...))

			builder := NewBuilder(tcg, testStartTime).
				WithNamespace(metav1.NamespaceDefault).
				WithProfileName(tc.profile).
				WithModeName(v1alpha1.SlurmMode).
				WithScript(tc.tempFile).
				WithArray(tc.array).
				WithCpusPerTask(tc.cpusPerTask).
				WithGpusPerTask(tc.gpusPerTask).
				WithMemPerTask(tc.memPerTask).
				WithMemPerCPU(tc.memPerCPU).
				WithMemPerGPU(tc.memPerGPU).
				WithNodes(tc.nodes).
				WithNTasks(tc.nTasks).
				WithOutput(tc.output).
				WithError(tc.err).
				WithInput(tc.input).
				WithJobName(tc.jobName).
				WithPartition(tc.partition).
				WithInitImage(baseInitContainerImage).
				WithFirstNodeIPTimeout(tc.firstNodeTimeout).
				WithWorkerContainers(tc.workerContainers)

			gotRootObj, gotChildObjs, gotErr := builder.Do(ctx)

			var opts []cmp.Option
			var statusError *apierrors.StatusError
			if !errors.As(tc.wantErr, &statusError) {
				opts = append(opts, cmpopts.EquateErrors())
			}
			if diff := cmp.Diff(tc.wantErr, gotErr, opts...); diff != "" {
				t.Errorf("Unexpected error (-want/+got)\n%s", diff)
			}

			defaultCmpOpts := []cmp.Option{cmpopts.IgnoreFields(metav1.ObjectMeta{}, "Name")}
			opts = append(defaultCmpOpts, tc.cmpopts...)

			if job, ok := tc.wantRootObj.(*batchv1.Job); ok {
				if job.Annotations == nil {
					job.Annotations = make(map[string]string, 1)
				}
				job.Annotations[constants.ScriptAnnotation] = tc.tempFile
			}

			if diff := cmp.Diff(tc.wantRootObj, gotRootObj, opts...); diff != "" {
				t.Errorf("Root object after build (-want,+got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantChildObjs, gotChildObjs, opts...); diff != "" {
				t.Errorf("Child objects after build (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestSlurmBuilderBuildEntrypointCommand(t *testing.T) {
	testStartTime := time.Now()

	testCases := map[string]struct {
		input                 string
		output                string
		error                 string
		wantEntrypointCommand string
	}{
		"should build entrypoint command": {
			wantEntrypointCommand: "/slurm/scripts/script",
		},
		"should build entrypoint command with output": {
			output:                "/home/test/stdout.out",
			wantEntrypointCommand: "/slurm/scripts/script 1> >(tee /home/test/stdout.out)",
		},
		"should build entrypoint command with error": {
			error:                 "/home/test/stderr.out",
			wantEntrypointCommand: "/slurm/scripts/script 2> >(tee /home/test/stderr.out >&2)",
		},
		"should build entrypoint command with output and error": {
			output:                "/home/test/stdout.out",
			error:                 "/home/test/stderr.out",
			wantEntrypointCommand: "/slurm/scripts/script 1> >(tee /home/test/stdout.out) 2> >(tee /home/test/stderr.out >&2)",
		},
		"should build entrypoint command with input": {
			input:                 "/home/test/script.sh",
			wantEntrypointCommand: "/slurm/scripts/script </home/test/script.sh",
		},
		"should build entrypoint command with input and output": {
			input:                 "/home/test/script.sh",
			output:                "/home/test/stdout.out",
			wantEntrypointCommand: "/slurm/scripts/script </home/test/script.sh 1> >(tee /home/test/stdout.out)",
		},
		"should build entrypoint command with input and error": {
			input:                 "/home/test/script.sh",
			error:                 "/home/test/stderr.out",
			wantEntrypointCommand: "/slurm/scripts/script </home/test/script.sh 2> >(tee /home/test/stderr.out >&2)",
		},
		"should build entrypoint command with input, output and error": {
			input:                 "/home/test/script.sh",
			output:                "/home/test/stdout.out",
			error:                 "/home/test/stderr.out",
			wantEntrypointCommand: "/slurm/scripts/script </home/test/script.sh 1> >(tee /home/test/stdout.out) 2> >(tee /home/test/stderr.out >&2)",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tcg := cmdtesting.NewTestClientGetter()

			newBuilder := NewBuilder(tcg, testStartTime).
				WithInput(tc.input).
				WithOutput(tc.output).
				WithError(tc.error)
			gotEntrypointCommand := newSlurmBuilder(newBuilder).buildEntrypointCommand()
			if diff := cmp.Diff(tc.wantEntrypointCommand, gotEntrypointCommand); diff != "" {
				t.Errorf("Unexpected entrypoint command (-want/+got)\n%s", diff)
				return
			}
		})
	}
}
