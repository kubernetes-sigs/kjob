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
	"slices"
	"strings"
	"time"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	kueueversioned "sigs.k8s.io/kueue/client-go/clientset/versioned"
	kueueconstants "sigs.k8s.io/kueue/pkg/controller/constants"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	"sigs.k8s.io/kjob/client-go/clientset/versioned"
	"sigs.k8s.io/kjob/pkg/cmd/helpers"
	"sigs.k8s.io/kjob/pkg/constants"
	"sigs.k8s.io/kjob/pkg/parser"
)

const (
	DefaultNodes                 = int32(1)
	DefaultNTasks                = int32(1)
	DefaultNTasksPerNode         = int32(1)
	DefaultArrayIndexParallelism = int32(1)
	DefaultArrayIndexStep        = int32(1)
)

var (
	errNoNamespaceSpecified                = errors.New("no namespace specified")
	errNoApplicationProfileSpecified       = errors.New("no application profile specified")
	errNoApplicationProfileModeSpecified   = errors.New("no application profile mode specified")
	errInvalidApplicationProfileMode       = errors.New("invalid application profile mode")
	errApplicationProfileModeNotConfigured = errors.New("application profile mode not configured")
	errNoCommandSpecified                  = errors.New("no command specified")
	errNoParallelismSpecified              = errors.New("no parallelism specified")
	errNoCompletionsSpecified              = errors.New("no completions specified")
	errNoReplicasSpecified                 = errors.New("no replicas specified")
	errNoMinReplicasSpecified              = errors.New("no min-replicas specified")
	errNoMaxReplicasSpecified              = errors.New("no max-replicas specified")
	errNoRequestsSpecified                 = errors.New("no requests specified")
	errNoLocalQueueSpecified               = errors.New("no local queue specified")
	errNoRayClusterSpecified               = errors.New("no raycluster specified")
	errNoArraySpecified                    = errors.New("no array specified")
	errNoCpusPerTaskSpecified              = errors.New("no cpus-per-task specified")
	errNoErrorSpecified                    = errors.New("no error specified")
	errNoGpusPerTaskSpecified              = errors.New("no gpus-per-task specified")
	errNoInputSpecified                    = errors.New("no input specified")
	errNoJobNameSpecified                  = errors.New("no job-name specified")
	errNoMemPerCPUSpecified                = errors.New("no mem-per-cpu specified")
	errNoMemPerGPUSpecified                = errors.New("no mem-per-gpu specified")
	errNoMemPerTaskSpecified               = errors.New("no mem-per-task specified")
	errNoNodesSpecified                    = errors.New("no nodes specified")
	errNoNTasksSpecified                   = errors.New("no ntasks specified")
	errNoNTasksPerNodeSpecified            = errors.New("no ntasks-per-node specified")
	errNoOutputSpecified                   = errors.New("no output specified")
	errNoPartitionSpecified                = errors.New("no partition specified")
	errNoPrioritySpecified                 = errors.New("no priority specified")
	errNoTimeSpecified                     = errors.New("no time specified")
	errNoPodTemplateLabelSpecified         = errors.New("no pod template label specified")
	errNoPodTemplateAnnotationSpecified    = errors.New("no pod template annotation specified")
)

type builder interface {
	build(ctx context.Context) (rootObj runtime.Object, childObjs []runtime.Object, err error)
}

type Builder struct {
	clientGetter     helpers.ClientGetter
	kjobctlClientset versioned.Interface
	k8sClientset     k8s.Interface
	kueueClientset   kueueversioned.Interface

	namespace   string
	profileName string
	modeName    v1alpha1.ApplicationProfileMode

	command                  []string
	parallelism              *int32
	completions              *int32
	replicas                 map[string]int
	minReplicas              map[string]int
	maxReplicas              map[string]int
	requests                 corev1.ResourceList
	localQueue               string
	rayCluster               string
	script                   string
	array                    string
	cpusPerTask              *resource.Quantity
	error                    string
	gpusPerTask              map[string]*resource.Quantity
	input                    string
	jobName                  string
	memPerNode               *resource.Quantity
	memPerCPU                *resource.Quantity
	memPerGPU                *resource.Quantity
	memPerTask               *resource.Quantity
	nodes                    *int32
	nTasks                   *int32
	nTasksPerNode            *int32
	output                   string
	partition                string
	priority                 string
	initImage                string
	ignoreUnknown            bool
	skipLocalQueueValidation bool
	skipPriorityValidation   bool
	firstNodeIP              bool
	firstNodeIPTimeout       time.Duration
	changeDir                string
	timeLimit                string
	podTemplateLabels        map[string]string
	podTemplateAnnotations   map[string]string
	workerContainers         []string

	profile       *v1alpha1.ApplicationProfile
	mode          *v1alpha1.SupportedMode
	volumeBundles []v1alpha1.VolumeBundle

	buildTime time.Time
}

func NewBuilder(clientGetter helpers.ClientGetter, buildTime time.Time) *Builder {
	return &Builder{clientGetter: clientGetter, buildTime: buildTime}
}

func (b *Builder) WithNamespace(namespace string) *Builder {
	b.namespace = namespace
	return b
}

func (b *Builder) WithProfileName(profileName string) *Builder {
	b.profileName = profileName
	return b
}

func (b *Builder) WithModeName(modeName v1alpha1.ApplicationProfileMode) *Builder {
	b.modeName = modeName
	return b
}

func (b *Builder) WithCommand(command []string) *Builder {
	b.command = command
	return b
}

func (b *Builder) WithParallelism(parallelism *int32) *Builder {
	b.parallelism = parallelism
	return b
}

func (b *Builder) WithCompletions(completions *int32) *Builder {
	b.completions = completions
	return b
}

func (b *Builder) WithReplicas(replicas map[string]int) *Builder {
	b.replicas = replicas
	return b
}

func (b *Builder) WithMinReplicas(minReplicas map[string]int) *Builder {
	b.minReplicas = minReplicas
	return b
}

func (b *Builder) WithMaxReplicas(maxReplicas map[string]int) *Builder {
	b.maxReplicas = maxReplicas
	return b
}

func (b *Builder) WithRequests(requests corev1.ResourceList) *Builder {
	b.requests = requests
	return b
}

func (b *Builder) WithLocalQueue(localQueue string) *Builder {
	b.localQueue = localQueue
	return b
}

func (b *Builder) WithRayCluster(rayCluster string) *Builder {
	b.rayCluster = rayCluster
	return b
}

func (b *Builder) WithScript(script string) *Builder {
	b.script = script
	return b
}

func (b *Builder) WithArray(array string) *Builder {
	b.array = array
	return b
}

func (b *Builder) WithCpusPerTask(cpusPerTask *resource.Quantity) *Builder {
	b.cpusPerTask = cpusPerTask
	return b
}

func (b *Builder) WithError(error string) *Builder {
	b.error = error
	return b
}

func (b *Builder) WithGpusPerTask(gpusPerTask map[string]*resource.Quantity) *Builder {
	b.gpusPerTask = gpusPerTask
	return b
}

func (b *Builder) WithInput(input string) *Builder {
	b.input = input
	return b
}

func (b *Builder) WithJobName(jobName string) *Builder {
	b.jobName = jobName
	return b
}

func (b *Builder) WithMemPerNode(memPerNode *resource.Quantity) *Builder {
	b.memPerNode = memPerNode
	return b
}

func (b *Builder) WithMemPerCPU(memPerCPU *resource.Quantity) *Builder {
	b.memPerCPU = memPerCPU
	return b
}

func (b *Builder) WithMemPerGPU(memPerGPU *resource.Quantity) *Builder {
	b.memPerGPU = memPerGPU
	return b
}

func (b *Builder) WithMemPerTask(memPerTask *resource.Quantity) *Builder {
	b.memPerTask = memPerTask
	return b
}

func (b *Builder) WithNodes(nodes *int32) *Builder {
	b.nodes = nodes
	return b
}

func (b *Builder) WithNTasks(nTasks *int32) *Builder {
	b.nTasks = nTasks
	return b
}

func (b *Builder) WithNTasksPerNode(nTasksPerNode *int32) *Builder {
	b.nTasksPerNode = nTasksPerNode
	return b
}

func (b *Builder) WithOutput(output string) *Builder {
	b.output = output
	return b
}

func (b *Builder) WithPartition(partition string) *Builder {
	b.partition = partition
	return b
}

func (b *Builder) WithPriority(priority string) *Builder {
	b.priority = priority
	return b
}

func (b *Builder) WithInitImage(initImage string) *Builder {
	b.initImage = initImage
	return b
}

func (b *Builder) WithIgnoreUnknown(ignoreUnknown bool) *Builder {
	b.ignoreUnknown = ignoreUnknown
	return b
}

func (b *Builder) WithChangeDir(chdir string) *Builder {
	b.changeDir = chdir
	return b
}

func (b *Builder) WithSkipLocalQueueValidation(skip bool) *Builder {
	b.skipLocalQueueValidation = skip
	return b
}

func (b *Builder) WithSkipPriorityValidation(skip bool) *Builder {
	b.skipPriorityValidation = skip
	return b
}

func (b *Builder) WithFirstNodeIP(firstNodeIP bool) *Builder {
	b.firstNodeIP = firstNodeIP
	return b
}

func (b *Builder) WithFirstNodeIPTimeout(timeout time.Duration) *Builder {
	b.firstNodeIPTimeout = timeout
	return b
}

func (b *Builder) WithTimeLimit(timeLimit string) *Builder {
	b.timeLimit = timeLimit
	return b
}

func (b *Builder) WithPodTemplateLabels(podTemplateLabels map[string]string) *Builder {
	b.podTemplateLabels = podTemplateLabels
	return b
}

func (b *Builder) WithPodTemplateAnnotations(podTemplateAnnotations map[string]string) *Builder {
	b.podTemplateAnnotations = podTemplateAnnotations
	return b
}

func (b *Builder) WithWorkerContainers(workerContainers []string) *Builder {
	b.workerContainers = workerContainers
	return b
}

func (b *Builder) validateGeneral(ctx context.Context) error {
	if b.namespace == "" {
		return errNoNamespaceSpecified
	}

	if b.profileName == "" {
		return errNoApplicationProfileSpecified
	}

	if b.modeName == "" {
		return errNoApplicationProfileModeSpecified
	}

	// check that local queue exists
	if len(b.localQueue) != 0 && !b.skipLocalQueueValidation {
		_, err := b.kueueClientset.KueueV1beta1().LocalQueues(b.namespace).Get(ctx, b.localQueue, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}

	// check that priority class exists
	if len(b.priority) != 0 && !b.skipPriorityValidation {
		_, err := b.kueueClientset.KueueV1beta1().WorkloadPriorityClasses().Get(ctx, b.priority, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Builder) complete(ctx context.Context) error {
	var err error

	b.profile, err = b.kjobctlClientset.KjobctlV1alpha1().ApplicationProfiles(b.namespace).Get(ctx, b.profileName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for i, mode := range b.profile.Spec.SupportedModes {
		if mode.Name == b.modeName {
			b.mode = &b.profile.Spec.SupportedModes[i]
		}
	}

	if b.mode == nil {
		return errApplicationProfileModeNotConfigured
	}

	for _, name := range b.profile.Spec.VolumeBundles {
		volumeBundle, err := b.kjobctlClientset.KjobctlV1alpha1().VolumeBundles(b.profile.Namespace).Get(ctx, string(name), metav1.GetOptions{})
		if err != nil {
			return err
		}
		b.volumeBundles = append(b.volumeBundles, *volumeBundle)
	}

	return nil
}

func (b *Builder) validateFlags() error {
	if slices.Contains(b.mode.RequiredFlags, v1alpha1.CmdFlag) && len(b.command) == 0 {
		return errNoCommandSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.ParallelismFlag) && b.parallelism == nil {
		return errNoParallelismSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.CompletionsFlag) && b.completions == nil {
		return errNoCompletionsSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.ReplicasFlag) && b.replicas == nil {
		return errNoReplicasSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.MinReplicasFlag) && b.minReplicas == nil {
		return errNoMinReplicasSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.MaxReplicasFlag) && b.maxReplicas == nil {
		return errNoMaxReplicasSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.RequestFlag) && b.requests == nil {
		return errNoRequestsSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.LocalQueueFlag) && b.localQueue == "" {
		return errNoLocalQueueSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.RayClusterFlag) && b.rayCluster == "" {
		return errNoRayClusterSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.ArrayFlag) && b.array == "" {
		return errNoArraySpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.CpusPerTaskFlag) && b.cpusPerTask == nil {
		return errNoCpusPerTaskSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.ErrorFlag) && b.error == "" {
		return errNoErrorSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.GpusPerTaskFlag) && b.gpusPerTask == nil {
		return errNoGpusPerTaskSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.InputFlag) && b.input == "" {
		return errNoInputSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.JobNameFlag) && b.jobName == "" {
		return errNoJobNameSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.MemPerCPUFlag) && b.memPerCPU == nil {
		return errNoMemPerCPUSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.MemPerGPUFlag) && b.memPerGPU == nil {
		return errNoMemPerGPUSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.MemPerTaskFlag) && b.memPerTask == nil {
		return errNoMemPerTaskSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.NodesFlag) && b.nodes == nil {
		return errNoNodesSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.NTasksFlag) && b.nTasks == nil {
		return errNoNTasksSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.NTasksPerNodeFlag) && b.nTasksPerNode == nil {
		return errNoNTasksPerNodeSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.OutputFlag) && b.output == "" {
		return errNoOutputSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.PartitionFlag) && b.partition == "" {
		return errNoPartitionSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.PriorityFlag) && b.priority == "" {
		return errNoPrioritySpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.TimeFlag) && b.timeLimit == "" {
		return errNoTimeSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.PodTemplateLabelFlag) && b.podTemplateLabels == nil {
		return errNoPodTemplateLabelSpecified
	}

	if slices.Contains(b.mode.RequiredFlags, v1alpha1.PodTemplateAnnotationFlag) && b.podTemplateAnnotations == nil {
		return errNoPodTemplateAnnotationSpecified
	}

	return nil
}

func (b *Builder) Do(ctx context.Context) (runtime.Object, []runtime.Object, error) {
	if err := b.setClients(); err != nil {
		return nil, nil, err
	}

	if err := b.validateGeneral(ctx); err != nil {
		return nil, nil, err
	}

	var bImpl builder

	switch b.modeName {
	case v1alpha1.JobMode:
		bImpl = newJobBuilder(b)
	case v1alpha1.InteractiveMode:
		bImpl = newInteractiveBuilder(b)
	case v1alpha1.RayJobMode:
		bImpl = newRayJobBuilder(b)
	case v1alpha1.RayClusterMode:
		bImpl = newRayClusterBuilder(b)
	case v1alpha1.SlurmMode:
		bImpl = newSlurmBuilder(b)
	}

	if bImpl == nil {
		return nil, nil, errInvalidApplicationProfileMode
	}

	if err := b.complete(ctx); err != nil {
		return nil, nil, err
	}

	if err := b.validateFlags(); err != nil {
		return nil, nil, err
	}

	return bImpl.build(ctx)
}

func (b *Builder) setClients() error {
	var err error

	b.kjobctlClientset, err = b.clientGetter.KjobctlClientset()
	if err != nil {
		return err
	}

	b.k8sClientset, err = b.clientGetter.K8sClientset()
	if err != nil {
		return err
	}

	b.kueueClientset, err = b.clientGetter.KueueClientset()
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) buildObjectMeta(templateObjectMeta metav1.ObjectMeta, strictNaming bool) (metav1.ObjectMeta, error) {
	objectMeta := metav1.ObjectMeta{
		Namespace:   b.profile.Namespace,
		Labels:      templateObjectMeta.Labels,
		Annotations: templateObjectMeta.Annotations,
	}

	if strictNaming {
		objectMeta.Name = b.generatePrefixName() + utilrand.String(5)
	} else {
		objectMeta.GenerateName = b.generatePrefixName()
	}

	b.withKjobLabels(&objectMeta)
	if err := b.withKueueLabels(&objectMeta); err != nil {
		return metav1.ObjectMeta{}, err
	}

	return objectMeta, nil
}

func (b *Builder) buildChildObjectMeta(name string) metav1.ObjectMeta {
	objectMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: b.profile.Namespace,
	}
	b.withKjobLabels(&objectMeta)
	return objectMeta
}

func (b *Builder) withKjobLabels(objectMeta *metav1.ObjectMeta) {
	if objectMeta.Labels == nil {
		objectMeta.Labels = map[string]string{}
	}

	if b.profile != nil {
		objectMeta.Labels[constants.ProfileLabel] = b.profile.Name
	}

	if b.mode != nil {
		objectMeta.Labels[constants.ModeLabel] = string(b.mode.Name)
	}
}

func (b *Builder) withKueueLabels(objectMeta *metav1.ObjectMeta) error {
	if objectMeta.Labels == nil {
		objectMeta.Labels = map[string]string{}
	}

	if len(b.localQueue) > 0 {
		objectMeta.Labels[kueueconstants.QueueLabel] = b.localQueue
	}

	if len(b.priority) != 0 {
		objectMeta.Labels[kueueconstants.WorkloadPriorityClassLabel] = b.priority
	}

	if b.timeLimit != "" {
		maxExecutionTimeSeconds, err := parser.TimeLimitToSeconds(b.timeLimit)
		if err != nil {
			return fmt.Errorf("cannot parse '%s': %w", b.timeLimit, err)
		}

		if ptr.Deref(maxExecutionTimeSeconds, 0) > 0 {
			objectMeta.Labels[kueueconstants.MaxExecTimeSecondsLabel] = fmt.Sprint(*maxExecutionTimeSeconds)
		}
	}

	return nil
}

// buildPodObjectMeta sets user specified pod template labels and annotations
func (b *Builder) buildPodObjectMeta(templateObjectMeta *metav1.ObjectMeta) {
	templateObjectMeta.Labels = b.podTemplateLabels
	templateObjectMeta.Annotations = b.podTemplateAnnotations
}

func (b *Builder) buildPodTemplateSpec(podTemplateSpec *corev1.PodTemplateSpec) {
	b.buildPodObjectMeta(&podTemplateSpec.ObjectMeta)
	b.buildPodSpec(&podTemplateSpec.Spec)
}

func (b *Builder) buildPodSpec(podSpec *corev1.PodSpec) {
	b.buildPodSpecVolumesAndEnv(podSpec)

	for i := range podSpec.Containers {
		container := &podSpec.Containers[i]

		if i == 0 && len(b.command) > 0 {
			container.Command = b.command
		}

		if i == 0 && len(b.requests) > 0 {
			container.Resources.Requests = b.requests
		}
	}
}

func (b *Builder) buildPodSpecVolumesAndEnv(templateSpec *corev1.PodSpec) {
	bundle := mergeBundles(b.volumeBundles)

	templateSpec.Volumes = append(templateSpec.Volumes, bundle.Spec.Volumes...)
	for i := range templateSpec.Containers {
		container := &templateSpec.Containers[i]

		container.VolumeMounts = append(container.VolumeMounts, bundle.Spec.ContainerVolumeMounts...)
		container.Env = append(container.Env, bundle.Spec.EnvVars...)
		container.Env = append(container.Env, b.additionalEnvironmentVariables()...)
	}

	for i := range templateSpec.InitContainers {
		initContainer := &templateSpec.InitContainers[i]

		initContainer.VolumeMounts = append(initContainer.VolumeMounts, bundle.Spec.ContainerVolumeMounts...)
		initContainer.Env = append(initContainer.Env, bundle.Spec.EnvVars...)
		initContainer.Env = append(initContainer.Env, b.additionalEnvironmentVariables()...)
	}
}

func (b *Builder) buildRayClusterSpec(spec *rayv1.RayClusterSpec) {
	b.buildPodSpecVolumesAndEnv(&spec.HeadGroupSpec.Template.Spec)

	for index := range spec.WorkerGroupSpecs {
		workerGroupSpec := &spec.WorkerGroupSpecs[index]

		if replicas, ok := b.replicas[workerGroupSpec.GroupName]; ok {
			workerGroupSpec.Replicas = ptr.To(int32(replicas))
		}
		if minReplicas, ok := b.minReplicas[workerGroupSpec.GroupName]; ok {
			workerGroupSpec.MinReplicas = ptr.To(int32(minReplicas))
		}
		if maxReplicas, ok := b.maxReplicas[workerGroupSpec.GroupName]; ok {
			workerGroupSpec.MaxReplicas = ptr.To(int32(maxReplicas))
		}

		b.buildPodSpecVolumesAndEnv(&workerGroupSpec.Template.Spec)
	}
}

func (b *Builder) additionalEnvironmentVariables() []corev1.EnvVar {
	userID := os.Getenv(constants.SystemEnvVarNameUser)
	timestamp := b.buildTime.Format(time.RFC3339)
	taskName := fmt.Sprintf("%s_%s", b.namespace, b.profileName)

	envVars := []corev1.EnvVar{
		{Name: constants.EnvVarNameUserID, Value: userID},
		{Name: constants.EnvVarTaskName, Value: taskName},
		{Name: constants.EnvVarTaskID, Value: fmt.Sprintf("%s_%s_%s", userID, timestamp, taskName)},
		{Name: constants.EnvVarNameProfile, Value: fmt.Sprintf("%s_%s", b.namespace, b.profileName)},
		{Name: constants.EnvVarNameTimestamp, Value: timestamp},
	}

	return envVars
}

func mergeBundles(bundles []v1alpha1.VolumeBundle) v1alpha1.VolumeBundle {
	var volumeBundle v1alpha1.VolumeBundle
	for _, b := range bundles {
		volumeBundle.Spec.Volumes = append(volumeBundle.Spec.Volumes, b.Spec.Volumes...)
		volumeBundle.Spec.ContainerVolumeMounts = append(volumeBundle.Spec.ContainerVolumeMounts, b.Spec.ContainerVolumeMounts...)
		volumeBundle.Spec.EnvVars = append(volumeBundle.Spec.EnvVars, b.Spec.EnvVars...)
	}

	return volumeBundle
}

func (b *Builder) generatePrefixName() string {
	return strings.ToLower(fmt.Sprintf("%s-%s-", b.profile.Name, b.modeName))
}
