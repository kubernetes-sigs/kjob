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
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/template"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	kjobctlconstants "sigs.k8s.io/kjob/pkg/constants"
	"sigs.k8s.io/kjob/pkg/parser"
	utilslices "sigs.k8s.io/kjob/pkg/util/slices"
	"sigs.k8s.io/kjob/pkg/util/validate"
)

//go:embed templates/slurm_*
var slurmTemplates embed.FS

const SlurmInitContainerName = "slurm-init-env"

const (
	// Note that the first job ID will always be 1.
	slurmFirstJobID = 1

	slurmScriptsPath            = "/slurm/scripts"
	slurmInitEntrypointFilename = "init-entrypoint.sh"
	slurmEntrypointFilename     = "entrypoint.sh"
	slurmScriptFilename         = "script"

	slurmEnvsPath         = "/slurm/env"
	slurmSlurmEnvFilename = "slurm.env"
)

var (
	errNoScriptSpecified                             = errors.New("no script specified")
	errNTasksOrNTasksPerNodeMustBeSpecifiedWithNodes = errors.New("ntasks or ntasks-per-node must be specified with nodes")
	errInvalidNodesNTasksOrNTasksPerNodeValue        = errors.New("invalid nodes, ntasks or ntasks-per-node value")
	errInvalidNTasksOrNTasksPerNodeValue             = errors.New("invalid ntasks or ntasks-per-node value")
	errInvalidNodesOrNTasksValue                     = errors.New("invalid nodes or ntasks value")
)

var (
	slurmInitEntrypointFilenamePath = fmt.Sprintf("%s/%s", slurmScriptsPath, slurmInitEntrypointFilename)
	slurmEntrypointFilenamePath     = fmt.Sprintf("%s/%s", slurmScriptsPath, slurmEntrypointFilename)
	slurmScriptFilenamePath         = fmt.Sprintf("%s/%s", slurmScriptsPath, slurmScriptFilename)

	unmaskReplacer = strings.NewReplacer(
		"%%", "%",
		"%A", "${SLURM_ARRAY_JOB_ID}",
		"%a", "${SLURM_ARRAY_TASK_ID}",
		"%j", "${SLURM_JOB_ID}",
		"%N", "${HOSTNAME}",
		"%n", "${JOB_COMPLETION_INDEX}",
		"%t", "${SLURM_ARRAY_TASK_ID}",
		"%u", "${USER_ID}",
		"%x", "${SBATCH_JOB_NAME}",
	)
)

type slurmBuilder struct {
	*Builder

	scriptContent   string
	template        *template.Template
	arrayIndexes    *parser.ArrayIndexes
	cpusOnNode      *resource.Quantity
	cpusPerGpu      *resource.Quantity
	totalMemPerNode *resource.Quantity
	totalGpus       *resource.Quantity
}

var _ builder = (*slurmBuilder)(nil)

func (b *slurmBuilder) build(ctx context.Context) (runtime.Object, []runtime.Object, error) {
	if len(b.script) == 0 {
		return nil, nil, errNoScriptSpecified
	}

	err := b.parse()
	if err != nil {
		return nil, nil, err
	}

	err = b.complete()
	if err != nil {
		return nil, nil, err
	}

	template, err := b.kjobctlClientset.KjobctlV1alpha1().JobTemplates(b.profile.Namespace).
		Get(ctx, string(b.mode.Template), metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	objectMeta, err := b.buildObjectMeta(template.Template.ObjectMeta, true)
	if err != nil {
		return nil, nil, err
	}

	if b.script != "" {
		if objectMeta.Annotations == nil {
			objectMeta.Annotations = make(map[string]string, 1)
		}
		objectMeta.Annotations[kjobctlconstants.ScriptAnnotation] = b.script
	}

	job := &batchv1.Job{
		TypeMeta:   metav1.TypeMeta{Kind: "Job", APIVersion: "batch/v1"},
		ObjectMeta: objectMeta,
		Spec:       template.Template.Spec,
	}

	job.Spec.Completions = ptr.To(b.completion())
	job.Spec.Parallelism = ptr.To(b.parallelism())
	job.Spec.CompletionMode = ptr.To(batchv1.IndexedCompletion)
	job.Spec.Template.Spec.Subdomain = job.Name

	b.buildPodObjectMeta(&job.Spec.Template.ObjectMeta)
	b.buildPodSpecVolumesAndEnv(&job.Spec.Template.Spec)
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: "slurm-scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: job.Name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  slurmInitEntrypointFilename,
							Path: slurmInitEntrypointFilename,
						},
						{
							Key:  slurmEntrypointFilename,
							Path: slurmEntrypointFilename,
						},
						{
							Key:  slurmScriptFilename,
							Path: slurmScriptFilename,
							Mode: ptr.To[int32](0755),
						},
					},
				},
			},
		},
		corev1.Volume{
			Name: "slurm-env",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	)

	job.Spec.Template.Spec.InitContainers = append(job.Spec.Template.Spec.InitContainers, corev1.Container{
		Name:    SlurmInitContainerName,
		Image:   b.initImage,
		Command: []string{"sh", slurmInitEntrypointFilenamePath},
		Env: []corev1.EnvVar{
			{
				Name: "POD_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "slurm-scripts",
				MountPath: slurmScriptsPath,
			},
			{
				Name:      "slurm-env",
				MountPath: slurmEnvsPath,
			},
		},
	})

	var totalGPUsPerTask resource.Quantity
	for _, number := range b.gpusPerTask {
		totalGPUsPerTask.Add(*number)
	}

	var memPerCPU, memPerGPU, memPerContainer resource.Quantity
	if b.memPerCPU != nil && b.cpusPerTask != nil {
		memPerCPU = *b.memPerCPU
		memPerCPU.Mul(b.cpusPerTask.Value())
	}

	if b.memPerGPU != nil && b.gpusPerTask != nil {
		memPerGPU = *b.memPerGPU
		memPerGPU.Mul(totalGPUsPerTask.Value())
	}

	if b.memPerNode != nil {
		mem := b.memPerNode.MilliValue() / int64(len(job.Spec.Template.Spec.Containers))
		memPerContainer = *resource.NewMilliQuantity(mem, b.memPerNode.Format)
	}

	var totalCpus, totalGpus, totalMem resource.Quantity
	for i := range job.Spec.Template.Spec.Containers {
		container := &job.Spec.Template.Spec.Containers[i]
		if len(b.workerContainers) > 0 && !slices.Contains(b.workerContainers, container.Name) {
			continue
		}
		container.Command = []string{"bash", slurmEntrypointFilenamePath}

		var requests corev1.ResourceList
		if b.requests != nil {
			requests = b.requests
		} else {
			requests = corev1.ResourceList{}
		}

		if b.cpusPerTask != nil {
			requests[corev1.ResourceCPU] = *b.cpusPerTask
			totalCpus.Add(*b.cpusPerTask)
		}

		if b.memPerTask != nil {
			requests[corev1.ResourceMemory] = *b.memPerTask
			totalMem.Add(*b.memPerTask)
		}

		if !memPerCPU.IsZero() {
			requests[corev1.ResourceMemory] = memPerCPU
			totalMem.Add(memPerCPU)
		}

		if !memPerGPU.IsZero() {
			requests[corev1.ResourceMemory] = memPerGPU
			totalMem.Add(memPerGPU)
		}

		if len(requests) > 0 {
			container.Resources.Requests = requests
		}

		limits := corev1.ResourceList{}
		if !memPerContainer.IsZero() {
			limits[corev1.ResourceMemory] = memPerContainer
		}

		if b.gpusPerTask != nil {
			for name, number := range b.gpusPerTask {
				limits[corev1.ResourceName(name)] = *number
			}
			totalGpus.Add(totalGPUsPerTask)
		}

		if len(limits) > 0 {
			container.Resources.Limits = limits
		}

		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name:      "slurm-scripts",
				MountPath: slurmScriptsPath,
			},
			corev1.VolumeMount{
				Name:      "slurm-env",
				MountPath: slurmEnvsPath,
			},
		)
	}

	if b.containers() > 1 {
		for i := 1; i < b.containers(); i++ {
			replica := job.Spec.Template.Spec.Containers[0].DeepCopy()
			replica.Name = fmt.Sprintf("%s-%d", job.Spec.Template.Spec.Containers[0].Name, i)
			job.Spec.Template.Spec.Containers = append(job.Spec.Template.Spec.Containers, *replica)

			if b.cpusPerTask != nil {
				totalCpus.Add(*b.cpusPerTask)
			}

			if !memPerCPU.IsZero() {
				totalMem.Add(memPerCPU)
			}

			if !memPerGPU.IsZero() {
				totalMem.Add(memPerGPU)
			}
		}

		job.Spec.Template.Spec.Containers[0].Name = fmt.Sprintf("%s-0", job.Spec.Template.Spec.Containers[0].Name)
	}

	if !totalCpus.IsZero() {
		b.cpusOnNode = &totalCpus
	}

	if !totalGpus.IsZero() {
		b.totalGpus = &totalGpus
	}

	if b.memPerNode != nil {
		b.totalMemPerNode = b.memPerNode
	} else if !totalMem.IsZero() {
		b.totalMemPerNode = &totalMem
	}

	if b.cpusOnNode != nil && b.totalGpus != nil && !totalGpus.IsZero() {
		cpusPerGpu := totalCpus.MilliValue() / totalGpus.MilliValue()
		b.cpusPerGpu = resource.NewQuantity(cpusPerGpu, b.cpusOnNode.Format)
	}

	initEntrypointScript, err := b.buildInitEntrypointScript(job.Name)
	if err != nil {
		return nil, nil, err
	}

	entrypointScript, err := b.buildEntrypointScript()
	if err != nil {
		return nil, nil, err
	}

	configMap := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
		ObjectMeta: b.buildChildObjectMeta(job.Name),
		Data: map[string]string{
			slurmInitEntrypointFilename: initEntrypointScript,
			slurmEntrypointFilename:     entrypointScript,
			slurmScriptFilename:         b.scriptContent,
		},
	}

	service := &corev1.Service{
		TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
		ObjectMeta: b.buildChildObjectMeta(job.Name),
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector: map[string]string{
				"job-name": job.Name,
			},
		},
	}

	return job, []runtime.Object{configMap, service}, nil
}

func (b *slurmBuilder) parse() error {
	content, err := os.ReadFile(b.script)
	if err != nil {
		return err
	}
	b.scriptContent = string(content)

	t, err := template.ParseFS(slurmTemplates, "templates/*")
	if err != nil {
		return err
	}
	b.template = t

	if err := b.getSbatchEnvs(); err != nil {
		return err
	}

	if err := b.replaceScriptFlags(); err != nil {
		return err
	}

	if b.isArrayJob() {
		b.arrayIndexes, err = parser.ParseArrayIndexes(b.array)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *slurmBuilder) isArrayJob() bool {
	return b.array != ""
}

func (b *slurmBuilder) complete() error {
	nodes := ptr.Deref(b.nodes, DefaultNodes)
	nTasks := ptr.Deref(b.nTasks, DefaultNTasks)
	nTasksPerNode := ptr.Deref(b.nTasksPerNode, DefaultNTasksPerNode)

	if nodes != DefaultNodes && nTasks == DefaultNTasks && nTasksPerNode == DefaultNTasksPerNode {
		return errNTasksOrNTasksPerNodeMustBeSpecifiedWithNodes
	}

	if nodes != DefaultNodes && nTasks != DefaultNTasks && nTasksPerNode != DefaultNTasksPerNode && nTasks != nodes*nTasksPerNode {
		return errInvalidNodesNTasksOrNTasksPerNodeValue
	}

	switch {
	case nodes == DefaultNodes && nTasks != DefaultNTasks && nTasksPerNode != DefaultNTasksPerNode:
		nodesFloat := float64(nTasks) / float64(nTasksPerNode)
		nodes = int32(nodesFloat)
		if nodesFloat > float64(nodes) {
			return errInvalidNTasksOrNTasksPerNodeValue
		}
		b.nodes = ptr.To(nodes)
	case nTasks != DefaultNTasks && nTasksPerNode == DefaultNTasksPerNode:
		nTasksPerNodeFloat := float64(nTasks) / float64(nodes)
		nTasksPerNode = int32(nTasksPerNodeFloat)
		if nTasksPerNodeFloat > float64(nTasksPerNode) {
			return errInvalidNodesOrNTasksValue
		}
		b.nTasksPerNode = ptr.To(nTasksPerNode)
	case nTasks == DefaultNTasks && nTasksPerNode != DefaultNTasksPerNode:
		nTasks = nodes * nTasksPerNode
		b.nTasks = ptr.To(nTasks)
	}

	// Only one of --nodes or --array can be specified unless --nodes is set to 1.
	if err := validate.ValidateMutuallyExclusiveFlags(map[string]bool{
		string(v1alpha1.NodesFlag): nodes != DefaultNodes,
		string(v1alpha1.ArrayFlag): b.array != "",
	}); err != nil {
		return err
	}

	if b.memPerCPU != nil && b.cpusPerTask == nil {
		return errNoCpusPerTaskSpecified
	}

	if b.memPerGPU != nil && b.gpusPerTask == nil {
		return errNoGpusPerTaskSpecified
	}

	if err := validate.ValidateMutuallyExclusiveFlags(map[string]bool{
		string(v1alpha1.MemPerNodeFlag): b.memPerNode != nil,
		string(v1alpha1.MemPerTaskFlag): b.memPerTask != nil,
		string(v1alpha1.MemPerCPUFlag):  b.memPerCPU != nil,
		string(v1alpha1.MemPerGPUFlag):  b.memPerGPU != nil,
	}); err != nil {
		return err
	}

	replacedScriptContent, err := parser.SlurmValidateAndReplaceScript(b.scriptContent, nTasks)
	if err != nil {
		return err
	}
	b.scriptContent = replacedScriptContent

	return nil
}

func (b *slurmBuilder) parallelism() int32 {
	if b.isArrayJob() {
		return ptr.Deref(b.arrayIndexes.Parallelism, DefaultArrayIndexParallelism)
	}
	return ptr.Deref(b.nodes, DefaultNodes)
}

func (b *slurmBuilder) completion() int32 {
	if b.isArrayJob() {
		return int32(b.arrayIndexes.Count())
	}
	return ptr.Deref(b.nodes, DefaultNodes)
}

func (b *slurmBuilder) containers() int {
	return int(ptr.Deref(b.nTasksPerNode, DefaultNTasksPerNode))
}

func (b *slurmBuilder) buildArrayIndexes() string {
	arrayIndexes := utilslices.Map(b.arrayIndexes.Indexes, func(from *int32) string {
		return strconv.FormatInt(int64(*from), 10)
	})
	return strings.Join(arrayIndexes, ",")
}

type slurmInitEntrypointScript struct {
	JobName   string
	Namespace string

	EnvsPath         string
	SlurmEnvFilename string

	SlurmCPUsPerTask    string
	SlurmCPUsOnNode     string
	SlurmJobCPUsPerNode string
	SlurmCPUsPerGPU     string
	SlurmMemPerCPU      string
	SlurmMemPerGPU      string
	SlurmMemPerNode     string
	SlurmGPUs           string

	SlurmFirstJobID    int32
	SlurmNTasks        int32
	SlurmNTasksPerNode int32
	SlurmJobNumNodes   int32
	SlurmJobNodeList   string
	SlurmJobFirstNode  string
	SlurmTasksPerNode  string

	SlurmSubmitDir string

	ArrayJob            bool
	ArrayIndexes        string
	SlurmArrayTaskCount int32
	SlurmArrayTaskMax   int32
	SlurmArrayTaskMin   int32
	SlurmArrayTaskStep  int32

	FirstNodeIP               bool
	FirstNodeIPTimeoutSeconds int32
}

func (b *slurmBuilder) buildInitEntrypointScript(jobName string) (string, error) {
	nTasks := ptr.Deref(b.nTasks, DefaultNTasks)
	nTasksPerNode := ptr.Deref(b.nTasksPerNode, DefaultNTasksPerNode)

	nNodes := b.completion()
	nodeList := make([]string, nNodes)
	for i := int32(0); i < nNodes; i++ {
		nodeList[i] = fmt.Sprintf("%s-%d.%s", jobName, i, jobName)
	}

	scriptValues := slurmInitEntrypointScript{
		JobName:   jobName,
		Namespace: b.namespace,

		EnvsPath:         slurmEnvsPath,
		SlurmEnvFilename: slurmSlurmEnvFilename,

		SlurmCPUsPerTask:    getValueOrEmpty(b.cpusPerTask),
		SlurmCPUsOnNode:     getValueOrEmpty(b.cpusOnNode),
		SlurmJobCPUsPerNode: getValueOrEmpty(b.cpusOnNode),
		SlurmCPUsPerGPU:     getValueOrEmpty(b.cpusPerGpu),
		SlurmMemPerCPU:      getValueOrEmpty(b.memPerCPU),
		SlurmMemPerGPU:      getValueOrEmpty(b.memPerGPU),
		SlurmMemPerNode:     getValueOrEmpty(b.totalMemPerNode),
		SlurmGPUs:           getValueOrEmpty(b.totalGpus),

		SlurmNTasks:        nTasks,
		SlurmNTasksPerNode: nTasksPerNode,
		SlurmJobNumNodes:   nNodes,
		SlurmJobNodeList:   strings.Join(nodeList, ","),
		SlurmJobFirstNode:  nodeList[0],
		SlurmFirstJobID:    slurmFirstJobID,
		// SlurmTasksPerNode:  nTasks, // TODO: Implement buildTasksPerNode() builder according to https://slurm.schedmd.com/sbatch.html#OPT_SLURM_TASKS_PER_NODE.

		SlurmSubmitDir: slurmScriptsPath,
	}

	if b.isArrayJob() {
		scriptValues.ArrayJob = true
		scriptValues.ArrayIndexes = b.buildArrayIndexes()
		scriptValues.JobName = jobName
		scriptValues.SlurmFirstJobID = slurmFirstJobID
		scriptValues.SlurmArrayTaskCount = int32(b.arrayIndexes.Count())
		scriptValues.SlurmArrayTaskMax = b.arrayIndexes.Max()
		scriptValues.SlurmArrayTaskMin = b.arrayIndexes.Min()
		scriptValues.SlurmArrayTaskStep = ptr.Deref(b.arrayIndexes.Step, DefaultArrayIndexStep)
	}

	if b.firstNodeIP {
		scriptValues.FirstNodeIP = true
		scriptValues.FirstNodeIPTimeoutSeconds = int32(b.firstNodeIPTimeout.Seconds())
	}

	var script bytes.Buffer

	if err := b.template.ExecuteTemplate(&script, "slurm_init_entrypoint_script.sh.tmpl", scriptValues); err != nil {
		return "", err
	}

	return script.String(), nil
}

type slurmEntrypointScript struct {
	EnvsPath               string
	SbatchJobName          string
	SlurmEnvFilename       string
	ChangeDir              string
	BuildEntrypointCommand string
}

func (b *slurmBuilder) buildEntrypointScript() (string, error) {
	scriptValues := slurmEntrypointScript{
		EnvsPath:               slurmEnvsPath,
		SbatchJobName:          b.jobName,
		SlurmEnvFilename:       slurmSlurmEnvFilename,
		ChangeDir:              b.changeDir,
		BuildEntrypointCommand: b.buildEntrypointCommand(),
	}

	var script bytes.Buffer

	if err := b.template.ExecuteTemplate(&script, "slurm_entrypoint_script.sh.tmpl", scriptValues); err != nil {
		return "", err
	}

	return script.String(), nil
}

func getValueOrEmpty(ptr *resource.Quantity) string {
	if ptr != nil {
		return ptr.String()
	}

	return ""
}

// unmaskFilename unmasks a filename based on the filename pattern.
// For more details, see https://slurm.schedmd.com/sbatch.html#SECTION_FILENAME-PATTERN.
func unmaskFilename(filename string) string {
	if strings.Contains(filename, "\\\\") {
		return strings.ReplaceAll(filename, "\\\\", "")
	}
	return unmaskReplacer.Replace(filename)
}

func (b *slurmBuilder) buildEntrypointCommand() string {
	strBuilder := strings.Builder{}

	strBuilder.WriteString(slurmScriptFilenamePath)

	if b.input != "" {
		strBuilder.WriteString(" <")
		strBuilder.WriteString(unmaskFilename(b.input))
	}

	if b.output != "" {
		strBuilder.WriteString(" 1> >(tee ")
		strBuilder.WriteString(unmaskFilename(b.output))
		strBuilder.WriteByte(')')
	}

	if b.error != "" {
		strBuilder.WriteString(" 2> >(tee ")
		strBuilder.WriteString(unmaskFilename(b.error))
		strBuilder.WriteString(" >&2)")
	}

	return strBuilder.String()
}

func (b *slurmBuilder) getSbatchEnvs() error {
	if len(b.array) == 0 {
		b.array = os.Getenv("SBATCH_ARRAY_INX")
	}

	if b.gpusPerTask == nil {
		if env, ok := os.LookupEnv("SBATCH_GPUS_PER_TASK"); ok {
			val, err := parser.GpusFlag(env)
			if err != nil {
				return fmt.Errorf("cannot parse '%s': %w", env, err)
			}
			b.gpusPerTask = val
		}
	}

	if b.memPerNode == nil {
		if env, ok := os.LookupEnv("SBATCH_MEM_PER_NODE"); ok {
			val, err := resource.ParseQuantity(env)
			if err != nil {
				return fmt.Errorf("cannot parse '%s': %w", env, err)
			}
			b.memPerNode = ptr.To(val)
		}
	}

	if b.memPerTask == nil {
		if env, ok := os.LookupEnv("SBATCH_MEM_PER_CPU"); ok {
			val, err := resource.ParseQuantity(env)
			if err != nil {
				return fmt.Errorf("cannot parse '%s': %w", env, err)
			}
			b.memPerTask = ptr.To(val)
		}
	}

	if b.memPerGPU == nil {
		if env, ok := os.LookupEnv("SBATCH_MEM_PER_GPU"); ok {
			val, err := resource.ParseQuantity(env)
			if err != nil {
				return fmt.Errorf("cannot parse '%s': %w", env, err)
			}
			b.memPerGPU = ptr.To(val)
		}
	}

	if len(b.output) == 0 {
		b.output = os.Getenv("SBATCH_OUTPUT")
	}

	if len(b.error) == 0 {
		b.error = os.Getenv("SBATCH_ERROR")
	}

	if len(b.input) == 0 {
		b.input = os.Getenv("SBATCH_INPUT")
	}

	if len(b.jobName) == 0 {
		b.jobName = os.Getenv("SBATCH_JOB_NAME")
	}

	if len(b.partition) == 0 {
		b.partition = os.Getenv("SBATCH_PARTITION")
	}

	if b.timeLimit == "" {
		b.timeLimit = os.Getenv("SBATCH_TIMELIMIT")
	}

	return nil
}

func (b *slurmBuilder) replaceScriptFlags() error {
	scriptFlags, err := parser.SlurmFlags(b.scriptContent, b.ignoreUnknown)
	if err != nil {
		return err
	}

	if len(b.array) == 0 {
		b.array = scriptFlags.Array
	}

	if b.cpusPerTask == nil {
		b.cpusPerTask = scriptFlags.CpusPerTask
	}

	if b.gpusPerTask == nil {
		b.gpusPerTask = scriptFlags.GpusPerTask
	}

	if b.memPerNode == nil {
		b.memPerNode = scriptFlags.MemPerNode
	}

	if b.memPerTask == nil {
		b.memPerTask = scriptFlags.MemPerTask
	}

	if b.memPerCPU == nil {
		b.memPerCPU = scriptFlags.MemPerCPU
	}

	if b.memPerGPU == nil {
		b.memPerGPU = scriptFlags.MemPerGPU
	}

	if b.nodes == nil {
		b.nodes = scriptFlags.Nodes
	}

	if b.nTasks == nil {
		b.nTasks = scriptFlags.NTasks
	}

	if len(b.output) == 0 {
		b.output = scriptFlags.Output
	}

	if len(b.error) == 0 {
		b.error = scriptFlags.Error
	}

	if len(b.input) == 0 {
		b.input = scriptFlags.Input
	}

	if len(b.jobName) == 0 {
		b.jobName = scriptFlags.JobName
	}

	if len(b.partition) == 0 {
		b.partition = scriptFlags.Partition
	}

	if b.timeLimit == "" {
		b.timeLimit = scriptFlags.TimeLimit
	}

	return nil
}

func newSlurmBuilder(b *Builder) *slurmBuilder {
	return &slurmBuilder{Builder: b}
}
