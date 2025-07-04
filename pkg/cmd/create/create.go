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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/cmd/attach"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	"sigs.k8s.io/kjob/pkg/builder"
	"sigs.k8s.io/kjob/pkg/cmd/completion"
	"sigs.k8s.io/kjob/pkg/cmd/helpers"
	"sigs.k8s.io/kjob/pkg/parser"
)

const (
	profileFlagName                  = "profile"
	podRunningTimeoutFlagName        = "pod-running-timeout"
	removeFlagName                   = "rm"
	ignoreUnknownFlagName            = "ignore-unknown-flags"
	initImageFlagName                = "init-image"
	streamContainersFlagName         = "stream-container"
	workerContainersFlagName         = "worker-container"
	skipLocalQueueValidationFlagName = "skip-localqueue-validation"
	skipPriorityValidationFlagName   = "skip-priority-validation"
	changeDirFlagName                = "chdir"
	firstNodeIPFlagName              = "first-node-ip"
	firstNodeIPTimeoutFlagName       = "first-node-ip-timeout"
	waitFlagName                     = "wait"
	waitTimeoutFlagName              = "wait-timeout"

	commandFlagName               = string(v1alpha1.CmdFlag)
	parallelismFlagName           = string(v1alpha1.ParallelismFlag)
	completionsFlagName           = string(v1alpha1.CompletionsFlag)
	replicasFlagName              = string(v1alpha1.ReplicasFlag)
	minReplicasFlagName           = string(v1alpha1.MinReplicasFlag)
	maxReplicasFlagName           = string(v1alpha1.MaxReplicasFlag)
	requestFlagName               = string(v1alpha1.RequestFlag)
	localQueueFlagName            = string(v1alpha1.LocalQueueFlag)
	rayClusterFlagName            = string(v1alpha1.RayClusterFlag)
	arrayFlagName                 = string(v1alpha1.ArrayFlag)
	cpusPerTaskFlagName           = string(v1alpha1.CpusPerTaskFlag)
	gpusPerTaskFlagName           = string(v1alpha1.GpusPerTaskFlag)
	memPerNodeFlagName            = string(v1alpha1.MemPerNodeFlag)
	memPerTaskFlagName            = string(v1alpha1.MemPerTaskFlag)
	memPerCPUFlagName             = string(v1alpha1.MemPerCPUFlag)
	memPerGPUFlagName             = string(v1alpha1.MemPerGPUFlag)
	nodesFlagName                 = string(v1alpha1.NodesFlag)
	nTasksFlagName                = string(v1alpha1.NTasksFlag)
	nTasksPerNodeFlagName         = string(v1alpha1.NTasksPerNodeFlag)
	outputFlagName                = string(v1alpha1.OutputFlag)
	errorFlagName                 = string(v1alpha1.ErrorFlag)
	inputFlagName                 = string(v1alpha1.InputFlag)
	jobNameFlagName               = string(v1alpha1.JobNameFlag)
	partitionFlagName             = string(v1alpha1.PartitionFlag)
	priorityFlagName              = string(v1alpha1.PriorityFlag)
	timeFlagName                  = string(v1alpha1.TimeFlag)
	podTemplateLabelFlagName      = string(v1alpha1.PodTemplateLabelFlag)
	podTemplateAnnotationFlagName = string(v1alpha1.PodTemplateAnnotationFlag)
)

func withTimeFlag(f *pflag.FlagSet, p *string) {
	f.StringVarP(p, timeFlagName, "t", "",
		`Set a limit on the total run time of the job. 
A time limit of zero requests that no time limit be imposed. 
Acceptable time formats include "minutes", "minutes:seconds", 
"hours:minutes:seconds", "days-hours", "days-hours:minutes" 
and "days-hours:minutes:seconds".`)
}

var (
	createJobExample = templates.Examples(`
		# Create job 
  		kjobctl create job \
		--profile my-application-profile \
		--cmd "sleep 5" \
		--parallelism 4 \
		--completions 4 \
		--request cpu=500m,memory=4Gi \
		--localqueue my-local-queue-name
	`)
	createInteractiveExample = templates.Examples(`
		# Create interactive 
	  	kjobctl create interactive \
		--profile my-application-profile  \
		--pod-running-timeout 30s \
		--rm
	`)
	createRayJobLong = templates.LongDesc(`
		Create a rayjob.

		KubeRay operator is required for RayJob.
		How to install KubeRay operator you can find here https://ray-project.github.io/kuberay/deploy/installation/.
	`)
	createRayJobExample = templates.Examples(`
		# Create rayjob 
  		kjobctl create rayjob \
		--profile my-application-profile \
		--cmd "python /home/ray/samples/sample_code.py" \
		--replicas small-group=1 \
		--min-replicas small-group=1 \
		--max-replicas small-group=5 \
		--localqueue my-local-queue-name
	`)
	createRayClusterLong = templates.LongDesc(`
		Create a raycluster.

		KubeRay operator is required for RayCluster.
		How to install KubeRay operator you can find here https://ray-project.github.io/kuberay/deploy/installation/.
	`)
	createRayClusterExample = templates.Examples(`
		# Create raycluster 
  		kjobctl create raycluster \
		--profile my-application-profile \
		--replicas small-group=1 \
		--min-replicas small-group=1 \
		--max-replicas small-group=5 \
		--localqueue my-local-queue-name
	`)
	createSlurmExample = templates.Examples(`
		# Create slurm 
		kjobctl create slurm --profile my-application-profile -- \
		--array 0-5 --nodes 3 --ntasks 1 ./script.sh
	`)
)

var (
	podRunningTimeoutDefault = 1 * time.Minute
)

type CreateOptions struct {
	exec.StreamOptions

	PrintFlags *genericclioptions.PrintFlags
	Config     *restclient.Config
	Attach     attach.RemoteAttach
	AttachFunc func(*CreateOptions, *corev1.Container, remotecommand.TerminalSizeQueue, *corev1.Pod) func() error

	DryRunStrategy helpers.DryRunStrategy

	Namespace          string
	ProfileName        string
	ModeName           v1alpha1.ApplicationProfileMode
	Script             string
	InitImage          string
	PodRunningTimeout  time.Duration
	FirstNodeIPTimeout time.Duration
	WaitTimeout        time.Duration
	FirstNodeIP        bool
	RemoveObject       bool
	ChangeDir          string
	StreamContainers   []string
	WorkerContainers   []string
	SlurmFlagSet       *pflag.FlagSet
	activeStreams      sync.Map

	Command                  []string
	Parallelism              *int32
	Completions              *int32
	Replicas                 map[string]int
	MinReplicas              map[string]int
	MaxReplicas              map[string]int
	Requests                 corev1.ResourceList
	LocalQueue               string
	RayCluster               string
	Array                    string
	CpusPerTask              *apiresource.Quantity
	GpusPerTask              map[string]*apiresource.Quantity
	MemPerNode               *apiresource.Quantity
	MemPerTask               *apiresource.Quantity
	MemPerCPU                *apiresource.Quantity
	MemPerGPU                *apiresource.Quantity
	Nodes                    *int32
	NTasks                   *int32
	NTasksPerNode            *int32
	Output                   string
	Error                    string
	Input                    string
	JobName                  string
	Partition                string
	Priority                 string
	TimeLimit                string
	IgnoreUnknown            bool
	Wait                     bool
	SkipLocalQueueValidation bool
	SkipPriorityValidation   bool
	PodTemplateLabels        map[string]string
	PodTemplateAnnotations   map[string]string

	UserSpecifiedCommand       string
	UserSpecifiedParallelism   int32
	UserSpecifiedCompletions   int32
	UserSpecifiedRequest       map[string]string
	UserSpecifiedCpusPerTask   string
	UserSpecifiedGpusPerTask   string
	UserSpecifiedMemPerNode    string
	UserSpecifiedMemPerTask    string
	UserSpecifiedMemPerCPU     string
	UserSpecifiedMemPerGPU     string
	UserSpecifiedNodes         int32
	UserSpecifiedNTasks        int32
	UserSpecifiedNTasksPerNode int32

	PrintObj printers.ResourcePrinterFunc

	genericiooptions.IOStreams
}

func NewCreateOptions(streams genericiooptions.IOStreams) *CreateOptions {
	scheme := runtime.NewScheme()
	utilruntime.Must(k8sscheme.AddToScheme(scheme))
	utilruntime.Must(rayv1.AddToScheme(scheme))

	return &CreateOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created").WithTypeSetter(scheme),
		IOStreams:  streams,
		StreamOptions: exec.StreamOptions{
			IOStreams: streams,
		},
		Attach:     &attach.DefaultRemoteAttach{},
		AttachFunc: defaultAttachFunc,
	}
}

type modeSubcommand struct {
	ModeName v1alpha1.ApplicationProfileMode
	Setup    func(clientGetter helpers.ClientGetter, subcmd *cobra.Command, o *CreateOptions)
}

var createModeSubcommands = map[string]modeSubcommand{
	"job": {
		ModeName: v1alpha1.JobMode,
		Setup: func(clientGetter helpers.ClientGetter, subcmd *cobra.Command, o *CreateOptions) {
			subcmd.Use += " [--cmd COMMAND]" +
				" [--request RESOURCE_NAME=QUANTITY]" +
				" [--parallelism PARALLELISM]" +
				" [--completions COMPLETIONS]" +
				" [--time TIME_LIMIT]"
			subcmd.Short = "Create a job"
			subcmd.Example = createJobExample

			subcmd.Flags().StringVar(&o.UserSpecifiedCommand, commandFlagName, "",
				"Command which is associated with the resource.")
			subcmd.Flags().StringToStringVar(&o.UserSpecifiedRequest, requestFlagName, nil,
				"Request is a set of (resource name, quantity) pairs.")
			subcmd.Flags().Int32Var(&o.UserSpecifiedParallelism, parallelismFlagName, 0,
				"Parallelism specifies the maximum desired number of pods the job should run at any given time.")
			subcmd.Flags().Int32Var(&o.UserSpecifiedCompletions, completionsFlagName, 0,
				"Completions specifies the desired number of successfully finished pods.")

			withTimeFlag(subcmd.Flags(), &o.TimeLimit)
		},
	},
	"interactive": {
		ModeName: v1alpha1.InteractiveMode,
		Setup: func(clientGetter helpers.ClientGetter, subcmd *cobra.Command, o *CreateOptions) {
			subcmd.Use += " [--cmd COMMAND]" +
				" [--request RESOURCE_NAME=QUANTITY]" +
				" [--pod-running-timeout DURATION]" +
				" [--time TIME_LIMIT]" +
				" [--rm]"
			subcmd.Short = "Create an interactive shell"
			subcmd.Example = createInteractiveExample

			subcmd.Flags().StringVar(&o.UserSpecifiedCommand, commandFlagName, "",
				"Command which is associated with the resource.")
			subcmd.Flags().StringToStringVar(&o.UserSpecifiedRequest, requestFlagName, nil,
				"Request is a set of (resource name, quantity) pairs.")
			subcmd.Flags().DurationVar(&o.PodRunningTimeout, podRunningTimeoutFlagName, podRunningTimeoutDefault,
				"The length of time (like 5s, 2m, or 3h, higher than zero) to wait until at least one pod is running.")
			subcmd.Flags().BoolVar(&o.RemoveObject, removeFlagName, false,
				"Remove pod when interactive session exits.")

			withTimeFlag(subcmd.Flags(), &o.TimeLimit)
		},
	},
	"rayjob": {
		ModeName: v1alpha1.RayJobMode,
		Setup: func(clientGetter helpers.ClientGetter, subcmd *cobra.Command, o *CreateOptions) {
			subcmd.Use += " [--cmd COMMAND]" +
				" [--replicas [WORKER_GROUP]=REPLICAS]" +
				" [--min-replicas [WORKER_GROUP]=MIN_REPLICAS]" +
				" [--max-replicas [WORKER_GROUP]=MAX_REPLICAS]" +
				" [--time TIME_LIMIT]"
			subcmd.Short = "Create a rayjob"
			subcmd.Long = createRayJobLong
			subcmd.Example = createRayJobExample

			subcmd.Flags().StringVar(&o.UserSpecifiedCommand, commandFlagName, "",
				"Command which is associated with the resource.")
			subcmd.Flags().StringToIntVar(&o.Replicas, replicasFlagName, nil,
				"Replicas is the number of desired Pods for this worker group.")
			subcmd.Flags().StringToIntVar(&o.MinReplicas, minReplicasFlagName, nil,
				"MinReplicas denotes the minimum number of desired Pods for this worker group.")
			subcmd.Flags().StringToIntVar(&o.MaxReplicas, maxReplicasFlagName, nil,
				"MaxReplicas denotes the maximum number of desired Pods for this worker group, and the default value is maxInt32.")
			subcmd.Flags().StringVar(&o.RayCluster, rayClusterFlagName, "",
				"Existing ray cluster on which the job will be created.")

			withTimeFlag(subcmd.Flags(), &o.TimeLimit)

			subcmd.MarkFlagsMutuallyExclusive(rayClusterFlagName, replicasFlagName)
			subcmd.MarkFlagsMutuallyExclusive(rayClusterFlagName, minReplicasFlagName)
			subcmd.MarkFlagsMutuallyExclusive(rayClusterFlagName, maxReplicasFlagName)
			subcmd.MarkFlagsMutuallyExclusive(rayClusterFlagName, localQueueFlagName)
		},
	},
	"raycluster": {
		ModeName: v1alpha1.RayClusterMode,
		Setup: func(clientGetter helpers.ClientGetter, subcmd *cobra.Command, o *CreateOptions) {
			subcmd.Use += " [--replicas [WORKER_GROUP]=REPLICAS]" +
				" [--min-replicas [WORKER_GROUP]=MIN_REPLICAS]" +
				" [--max-replicas [WORKER_GROUP]=MAX_REPLICAS]" +
				" [--time TIME_LIMIT]"
			subcmd.Short = "Create a raycluster"
			subcmd.Long = createRayClusterLong
			subcmd.Example = createRayClusterExample

			subcmd.Flags().StringToIntVar(&o.Replicas, replicasFlagName, nil,
				"Replicas is the number of desired Pods for this worker group.")
			subcmd.Flags().StringToIntVar(&o.MinReplicas, minReplicasFlagName, nil,
				"MinReplicas denotes the minimum number of desired Pods for this worker group.")
			subcmd.Flags().StringToIntVar(&o.MaxReplicas, maxReplicasFlagName, nil,
				"MaxReplicas denotes the maximum number of desired Pods for this worker group, and the default value is maxInt32.")

			withTimeFlag(subcmd.Flags(), &o.TimeLimit)
		},
	},
	"slurm": {
		ModeName: v1alpha1.SlurmMode,
		Setup: func(clientGetter helpers.ClientGetter, subcmd *cobra.Command, o *CreateOptions) {
			subcmd.Use += " [--ignore-unknown-flags]" +
				" [--wait]" +
				" [--wait-timeout]" +
				" [--init-image IMAGE]" +
				" [--first-node-ip]" +
				" [--first-node-ip-timeout DURATION]" +
				" [--rm]" +
				" [--stream-container NAME]" +
				" [--worker-container NAME]" +
				" -- " +
				" [--array ARRAY]" +
				" [--cpus-per-task QUANTITY]" +
				" [--gpus-per-task QUANTITY]" +
				" [--mem QUANTITY]" +
				" [--mem-per-task QUANTITY]" +
				" [--mem-per-cpu QUANTITY]" +
				" [--mem-per-gpu QUANTITY]" +
				" [--nodes COUNT]" +
				" [--ntasks COUNT]" +
				" [--ntasks-per-node COUNT]" +
				" [--output FILENAME_PATTERN]" +
				" [--error FILENAME_PATTERN]" +
				" [--input FILENAME_PATTERN]" +
				" [--job-name NAME]" +
				" [--partition NAME]" +
				" SCRIPT"

			subcmd.Short = "Create a slurm job"
			subcmd.Example = createSlurmExample
			subcmd.Args = cobra.MinimumNArgs(1)

			subcmd.Flags().BoolVar(&o.IgnoreUnknown, ignoreUnknownFlagName, false,
				"Ignore all the unsupported flags in the bash script.")
			subcmd.Flags().BoolVar(&o.Wait, waitFlagName, false,
				"Wait for the Job to complete and stream its Pod logs.")
			subcmd.Flags().DurationVar(&o.WaitTimeout, waitTimeoutFlagName, 0,
				"Timeout for waiting for the job to complete.")
			subcmd.Flags().StringArrayVar(&o.StreamContainers, streamContainersFlagName, nil, "Specify containers from which logs will be streamed. This can be set only if --wait is used. By default, all containers are included.")
			subcmd.Flags().StringArrayVar(&o.WorkerContainers, workerContainersFlagName, nil, "Specify containers for which limits and volumes will be applied. By default, all containers are included.")
			subcmd.Flags().StringVar(&o.InitImage, initImageFlagName, "registry.k8s.io/busybox:1.27.2",
				"The image used for the init container.")
			subcmd.Flags().BoolVar(&o.FirstNodeIP, firstNodeIPFlagName, false,
				"Enable the retrieval of the first node's IP address.")
			subcmd.Flags().DurationVar(&o.FirstNodeIPTimeout, firstNodeIPTimeoutFlagName, time.Minute,
				"The timeout for the retrieval of the first node's IP address.")
			subcmd.Flags().BoolVar(&o.RemoveObject, removeFlagName, false,
				"Remove the job when it is canceled while running with the wait flag.")

			o.SlurmFlagSet = pflag.NewFlagSet("slurm", pflag.ExitOnError)
			o.SlurmFlagSet.StringVarP(&o.Array, arrayFlagName, "a", "",
				`Submit a job array, multiple jobs to be executed with identical parameters. 
The indexes specification identifies what array index values should be used. 
Multiple values may be specified using a comma separated list and/or a range of values with a "-" separator. For example, "--array=0-15" or "--array=0,6,16-32". 
A maximum number of simultaneously running tasks from the job array may be specified using a "%" separator. For example "--array=0-15%4" will limit the number of simultaneously running tasks from this job array to 4. 
The minimum index value is 0. The maximum index value is 2147483647.`)
			o.SlurmFlagSet.StringVarP(&o.UserSpecifiedCpusPerTask, cpusPerTaskFlagName, "c", "",
				"How much cpus a container inside a pod requires.")
			o.SlurmFlagSet.StringVar(&o.UserSpecifiedGpusPerTask, gpusPerTaskFlagName, "",
				"How much gpus a container inside a pod requires.")
			o.SlurmFlagSet.StringVar(&o.UserSpecifiedMemPerNode, memPerNodeFlagName, "",
				"How much memory a pod requires.")
			o.SlurmFlagSet.StringVar(&o.UserSpecifiedMemPerTask, memPerTaskFlagName, "",
				"How much memory a container requires.")
			o.SlurmFlagSet.StringVar(&o.UserSpecifiedMemPerCPU, memPerCPUFlagName, "",
				"How much memory a container requires, it multiplies the number of requested cpus per task by mem-per-cpu.")
			o.SlurmFlagSet.StringVar(&o.UserSpecifiedMemPerGPU, memPerGPUFlagName, "",
				"How much memory a container requires, it multiplies the number of requested gpus per task by mem-per-gpu.")
			o.SlurmFlagSet.Int32VarP(&o.UserSpecifiedNodes, nodesFlagName, "N", builder.DefaultNodes,
				"Number of pods to be used at a time.")
			o.SlurmFlagSet.Int32VarP(&o.UserSpecifiedNTasks, nTasksFlagName, "n", builder.DefaultNTasks,
				"Number of identical containers inside of a pod, usually 1.")
			o.SlurmFlagSet.Int32Var(&o.UserSpecifiedNTasksPerNode, nTasksPerNodeFlagName, builder.DefaultNTasksPerNode,
				"Request that ntasks be invoked on each node.")
			o.SlurmFlagSet.StringVarP(&o.Output, outputFlagName, "o", "",
				"Where to redirect the standard output stream of a task. If not passed it proceeds to stdout, and is available via kubectl logs.")
			o.SlurmFlagSet.StringVarP(&o.Error, errorFlagName, "e", "",
				"Where to redirect std error stream of a task. If not passed it proceeds to stdout, and is available via kubectl logs.")
			o.SlurmFlagSet.StringVar(&o.Input, inputFlagName, "",
				"What to pipe into the script.")
			o.SlurmFlagSet.StringVarP(&o.JobName, jobNameFlagName, "J", "",
				"What is the job name.")
			o.SlurmFlagSet.StringVar(&o.Partition, partitionFlagName, "",
				"Local queue name.")
			o.SlurmFlagSet.StringVarP(&o.ChangeDir, changeDirFlagName, "D", "",
				"Change directory before executing the script.")

			withTimeFlag(o.SlurmFlagSet, &o.TimeLimit)
		},
	},
}

func NewCreateCmd(clientGetter helpers.ClientGetter, streams genericiooptions.IOStreams, clock clock.Clock) *cobra.Command {
	o := NewCreateOptions(streams)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a task",
		Example: fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s",
			createJobExample,
			createInteractiveExample,
			createRayJobExample,
			createRayClusterExample,
			createSlurmExample,
		),
	}

	for modeName, modeSubcommand := range createModeSubcommands {
		subcmd := &cobra.Command{
			Use: modeName +
				" --profile APPLICATION_PROFILE_NAME" +
				" [--localqueue LOCAL_QUEUE_NAME]" +
				" [--skip-localqueue-validation]" +
				" [--priority NAME]" +
				" [--skip-priority-validation]",
			DisableFlagsInUseLine: true,
			Args:                  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.SilenceUsage = true

				err := o.Complete(clientGetter, cmd, args)
				if err != nil {
					return err
				}

				return o.Run(cmd.Context(), clientGetter, clock.Now())
			},
		}

		o.PrintFlags.AddFlags(subcmd)

		subcmd.Flags().StringVarP(&o.ProfileName, profileFlagName, "p", "",
			"Application profile contains a template (with defaults set) for running a specific type of application.")
		subcmd.Flags().StringVar(&o.LocalQueue, localQueueFlagName, "",
			"Kueue localqueue name which is associated with the resource.")
		subcmd.Flags().BoolVar(&o.SkipLocalQueueValidation, skipLocalQueueValidationFlagName, false,
			"Skip local queue validation. Add local queue even if the queue does not exist.")
		subcmd.Flags().StringVar(&o.Priority, priorityFlagName, "",
			"Apply priority for the entire workload.")
		subcmd.Flags().BoolVar(&o.SkipPriorityValidation, skipPriorityValidationFlagName, false,
			"Skip workload priority class validation. Add priority class label even if the class does not exist.")
		subcmd.Flags().StringToStringVar(&o.PodTemplateLabels, podTemplateLabelFlagName, make(map[string]string),
			"Specifies one or more labels for the Pod template.")
		subcmd.Flags().StringToStringVar(&o.PodTemplateAnnotations, podTemplateAnnotationFlagName, make(map[string]string),
			"Specifies one or more annotations for the Pod template.")

		modeSubcommand.Setup(clientGetter, subcmd, o)

		helpers.AddDryRunFlag(subcmd)

		_ = subcmd.MarkFlagRequired(profileFlagName)

		cobra.CheckErr(subcmd.RegisterFlagCompletionFunc(profileFlagName, completion.ApplicationProfileNameFunc(clientGetter)))
		cobra.CheckErr(subcmd.RegisterFlagCompletionFunc(localQueueFlagName, completion.LocalQueueNameFunc(clientGetter)))

		cmd.AddCommand(subcmd)
	}

	return cmd
}

func (o *CreateOptions) Complete(clientGetter helpers.ClientGetter, cmd *cobra.Command, args []string) error {
	currentSubcommand := createModeSubcommands[cmd.Name()]
	o.ModeName = currentSubcommand.ModeName

	var err error

	if o.ModeName == v1alpha1.SlurmMode {
		argsLenAtDash := cmd.ArgsLenAtDash()

		if argsLenAtDash == -1 || argsLenAtDash > 0 {
			return fmt.Errorf("unknown command \"%s\" for \"%s\"", args[0], cmd.CommandPath())
		}

		if err := o.SlurmFlagSet.Parse(args[argsLenAtDash:]); err != nil {
			return err
		}

		slurmArgs := o.SlurmFlagSet.Args()

		if len(slurmArgs) == 0 {
			return errors.New("must specify script")
		}

		if len(slurmArgs) > 1 {
			return errors.New("must specify only one script")
		}

		if o.WaitTimeout != 0 && !o.Wait {
			return errors.New("the --wait-timeout flag is required when --wait is set")
		}
		if len(o.StreamContainers) != 0 && !o.Wait {
			return errors.New("the --stream-container can only be specified for streaming output")
		}
		o.Script = slurmArgs[0]
	}

	o.Namespace, _, err = clientGetter.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.UserSpecifiedCommand != "" {
		o.Command = strings.Fields(o.UserSpecifiedCommand)
	}

	if cmd.Flags().Changed(parallelismFlagName) {
		o.Parallelism = ptr.To(o.UserSpecifiedParallelism)
	}

	if cmd.Flags().Changed(completionsFlagName) {
		o.Completions = ptr.To(o.UserSpecifiedCompletions)
	}

	if len(o.UserSpecifiedRequest) > 0 {
		o.Requests = make(corev1.ResourceList)
		for key, value := range o.UserSpecifiedRequest {
			quantity, err := apiresource.ParseQuantity(value)
			if err != nil {
				return err
			}
			o.Requests[corev1.ResourceName(key)] = quantity
		}
	}

	if cmd.Flags().Changed(podRunningTimeoutFlagName) && o.PodRunningTimeout <= 0 {
		return errors.New("--pod-running-timeout must be higher than zero")
	}

	if o.SlurmFlagSet.Changed(cpusPerTaskFlagName) {
		quantity, err := apiresource.ParseQuantity(o.UserSpecifiedCpusPerTask)
		if err != nil {
			return fmt.Errorf("cannot parse '%s': %w", o.UserSpecifiedCpusPerTask, err)
		}
		o.CpusPerTask = &quantity
	}

	if o.SlurmFlagSet.Changed(gpusPerTaskFlagName) {
		gpusPerTask, err := parser.GpusFlag(o.UserSpecifiedGpusPerTask)
		if err != nil {
			return fmt.Errorf("cannot parse '%s': %w", o.UserSpecifiedGpusPerTask, err)
		}
		o.GpusPerTask = gpusPerTask
	}

	if o.SlurmFlagSet.Changed(memPerNodeFlagName) {
		quantity, err := apiresource.ParseQuantity(o.UserSpecifiedMemPerNode)
		if err != nil {
			return fmt.Errorf("cannot parse '%s': %w", o.UserSpecifiedMemPerNode, err)
		}
		o.MemPerNode = &quantity
	}

	if o.SlurmFlagSet.Changed(memPerTaskFlagName) {
		quantity, err := apiresource.ParseQuantity(o.UserSpecifiedMemPerTask)
		if err != nil {
			return fmt.Errorf("cannot parse '%s': %w", o.UserSpecifiedMemPerTask, err)
		}
		o.MemPerTask = &quantity
	}

	if o.SlurmFlagSet.Changed(memPerCPUFlagName) {
		quantity, err := apiresource.ParseQuantity(o.UserSpecifiedMemPerCPU)
		if err != nil {
			return fmt.Errorf("cannot parse '%s': %w", o.UserSpecifiedMemPerCPU, err)
		}
		o.MemPerCPU = &quantity
	}

	if o.SlurmFlagSet.Changed(memPerGPUFlagName) {
		quantity, err := apiresource.ParseQuantity(o.UserSpecifiedMemPerGPU)
		if err != nil {
			return fmt.Errorf("cannot parse '%s': %w", o.UserSpecifiedMemPerGPU, err)
		}
		o.MemPerGPU = &quantity
	}

	if o.SlurmFlagSet.Changed(nodesFlagName) {
		if o.UserSpecifiedNodes <= 0 {
			return errors.New("--nodes must be greater than 0")
		}

		o.Nodes = &o.UserSpecifiedNodes
	}

	if o.SlurmFlagSet.Changed(nTasksFlagName) {
		if o.UserSpecifiedNTasks <= 0 {
			return errors.New("--ntasks must be greater than 0")
		}

		o.NTasks = &o.UserSpecifiedNTasks
	}

	if o.SlurmFlagSet.Changed(nTasksPerNodeFlagName) {
		if o.UserSpecifiedNTasksPerNode <= 0 {
			return errors.New("--ntasks-per-node must be greater than 0")
		}

		o.NTasksPerNode = &o.UserSpecifiedNTasksPerNode
	}

	o.DryRunStrategy, err = helpers.GetDryRunStrategy(cmd)
	if err != nil {
		return err
	}

	err = helpers.PrintFlagsWithDryRunStrategy(o.PrintFlags, o.DryRunStrategy)
	if err != nil {
		return err
	}

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	o.PrintObj = printer.PrintObj

	o.Config, err = clientGetter.ToRESTConfig()
	if err != nil {
		return err
	}

	err = setKubernetesDefaults(o.Config)
	if err != nil {
		return err
	}

	return nil
}

func (o *CreateOptions) Run(ctx context.Context, clientGetter helpers.ClientGetter, runTime time.Time) error {
	rootObj, childObjs, err := builder.NewBuilder(clientGetter, runTime).
		WithNamespace(o.Namespace).
		WithProfileName(o.ProfileName).
		WithModeName(o.ModeName).
		WithCommand(o.Command).
		WithParallelism(o.Parallelism).
		WithCompletions(o.Completions).
		WithReplicas(o.Replicas).
		WithMinReplicas(o.MinReplicas).
		WithMaxReplicas(o.MaxReplicas).
		WithRequests(o.Requests).
		WithLocalQueue(o.LocalQueue).
		WithRayCluster(o.RayCluster).
		WithScript(o.Script).
		WithArray(o.Array).
		WithCpusPerTask(o.CpusPerTask).
		WithGpusPerTask(o.GpusPerTask).
		WithMemPerNode(o.MemPerNode).
		WithMemPerTask(o.MemPerTask).
		WithMemPerCPU(o.MemPerCPU).
		WithMemPerGPU(o.MemPerGPU).
		WithNodes(o.Nodes).
		WithNTasks(o.NTasks).
		WithNTasksPerNode(o.NTasksPerNode).
		WithOutput(o.Output).
		WithError(o.Error).
		WithInput(o.Input).
		WithJobName(o.JobName).
		WithPartition(o.Partition).
		WithPriority(o.Priority).
		WithInitImage(o.InitImage).
		WithIgnoreUnknown(o.IgnoreUnknown).
		WithSkipLocalQueueValidation(o.SkipLocalQueueValidation).
		WithSkipPriorityValidation(o.SkipPriorityValidation).
		WithChangeDir(o.ChangeDir).
		WithFirstNodeIP(o.FirstNodeIP).
		WithFirstNodeIPTimeout(o.FirstNodeIPTimeout).
		WithTimeLimit(o.TimeLimit).
		WithPodTemplateLabels(o.PodTemplateLabels).
		WithPodTemplateAnnotations(o.PodTemplateAnnotations).
		WithWorkerContainers(o.WorkerContainers).
		Do(ctx)
	if err != nil {
		return err
	}

	if o.DryRunStrategy != helpers.DryRunClient {
		rootObj, err = o.createObject(ctx, clientGetter, rootObj, nil)
		if err != nil {
			return err
		}
	}

	err = o.PrintObj(rootObj, o.Out)
	if err != nil {
		return err
	}

	for i := range childObjs {
		if o.DryRunStrategy != helpers.DryRunClient {
			childObjs[i], err = o.createObject(ctx, clientGetter, childObjs[i], rootObj)
			if err != nil {
				return err
			}
		}

		err = o.PrintObj(childObjs[i], o.Out)
		if err != nil {
			return err
		}
	}

	if o.DryRunStrategy == helpers.DryRunNone && o.ModeName == v1alpha1.InteractiveMode {
		pod := rootObj.(*corev1.Pod)
		return o.RunInteractivePod(ctx, clientGetter, pod.Name)
	}

	if o.ModeName == v1alpha1.SlurmMode && o.Wait {
		job := rootObj.(*batchv1.Job)
		return o.watchJobAndStreamLogs(ctx, clientGetter, job.GetName())
	}

	return nil
}

func (o *CreateOptions) createObject(ctx context.Context, clientGetter helpers.ClientGetter, obj runtime.Object, owner runtime.Object) (runtime.Object, error) {
	options := metav1.CreateOptions{}
	if o.DryRunStrategy == helpers.DryRunServer {
		options.DryRun = []string{metav1.DryRunAll}
	}

	dynamicClient, err := clientGetter.DynamicClient()
	if err != nil {
		return nil, err
	}

	restMapper, err := clientGetter.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	gvr := mapping.Resource

	unstructuredObj := &unstructured.Unstructured{}
	unstructuredObj.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	if owner != nil {
		unstructuredOwner := &unstructured.Unstructured{}
		unstructuredOwner.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(owner)
		if err != nil {
			return nil, err
		}
		unstructuredObj.SetOwnerReferences(append(unstructuredOwner.GetOwnerReferences(), metav1.OwnerReference{
			APIVersion: unstructuredOwner.GetAPIVersion(),
			Kind:       unstructuredOwner.GetKind(),
			Name:       unstructuredOwner.GetName(),
			UID:        unstructuredOwner.GetUID(),
		}))
	}

	unstructuredObj, err = dynamicClient.Resource(gvr).Namespace(o.Namespace).Create(ctx, unstructuredObj, options)
	if err != nil {
		return nil, err
	}

	createdObj := obj.DeepCopyObject()

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), createdObj)
	if err != nil {
		return nil, err
	}

	return createdObj, nil
}

func (o *CreateOptions) RunInteractivePod(ctx context.Context, clientGetter helpers.ClientGetter, podName string) error {
	k8sClient, err := clientGetter.K8sClientset()
	if err != nil {
		return err
	}

	if o.RemoveObject {
		defer func() {
			err = k8sClient.CoreV1().Pods(o.Namespace).Delete(ctx, podName, metav1.DeleteOptions{})
			if err != nil {
				fmt.Fprintln(o.ErrOut, err.Error())
			}
			fmt.Fprintf(o.Out, "pod \"%s\" deleted\n", podName)
		}()
	}

	fmt.Fprintf(o.Out, "waiting for pod \"%s\" to be running...\n", podName)
	err = waitForPodRunning(ctx, k8sClient, o.Namespace, podName, o.PodRunningTimeout)
	if err != nil {
		return err
	}

	pod, err := k8sClient.CoreV1().Pods(o.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = attachTTY(o, pod)
	if err != nil {
		return err
	}

	return nil
}

func attachTTY(o *CreateOptions, pod *corev1.Pod) error {
	o.Stdin = true
	o.TTY = true
	containerToAttach := &pod.Spec.Containers[0]
	if !containerToAttach.TTY {
		return fmt.Errorf("error: Unable to use a TTY - container %s did not allocate one", containerToAttach.Name)
	}

	tty := o.SetupTTY()

	var sizeQueue remotecommand.TerminalSizeQueue
	if tty.Raw {
		// this call spawns a goroutine to monitor/update the terminal size
		sizeQueue = tty.MonitorSize(tty.GetSize())

		// unset p.Err if it was previously set because both stdout and stderr go over p.Out when tty is true
		o.ErrOut = nil
	}

	return tty.Safe(o.AttachFunc(o, containerToAttach, sizeQueue, pod))
}

func defaultAttachFunc(o *CreateOptions, containerToAttach *corev1.Container, sizeQueue remotecommand.TerminalSizeQueue, pod *corev1.Pod) func() error {
	return func() error {
		restClient, err := restclient.RESTClientFor(o.Config)
		if err != nil {
			return err
		}

		req := restClient.Post().
			Resource("pods").
			Name(pod.Name).
			Namespace(pod.Namespace).
			SubResource("attach")
		req.VersionedParams(&corev1.PodAttachOptions{
			Container: containerToAttach.Name,
			Stdin:     o.Stdin,
			Stdout:    o.Out != nil,
			Stderr:    o.ErrOut != nil,
			TTY:       o.TTY,
		}, k8sscheme.ParameterCodec)

		return o.Attach.Attach(req.URL(), o.Config, o.In, o.Out, o.ErrOut, o.TTY, sizeQueue)
	}
}

func (o *CreateOptions) watchJobAndStreamLogs(ctx context.Context, clientGetter helpers.ClientGetter, jobName string) error {
	k8sClient, err := clientGetter.K8sClientset()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	defer close(sigChan)

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	waitTimeoutDone := make(chan struct{}, 1)
	if o.WaitTimeout > 0 {
		go func() {
			defer close(waitTimeoutDone)
			select {
			case <-ctx.Done():
			case <-time.After(o.WaitTimeout):
			}
		}()
	}

	watcherErrChan := make(chan error, 1)
	go func() {
		defer close(watcherErrChan)
		err := o.watchJobPods(ctx, clientGetter, jobName)
		watcherErrChan <- err
	}()

	jobCompletionErrChan := make(chan error, 1)
	go func() {
		defer close(jobCompletionErrChan)
		err := o.verifyJobFinished(ctx, k8sClient, jobName)
		jobCompletionErrChan <- err
	}()

	select {
	case sig := <-sigChan:
		fmt.Fprintf(o.Out, "Received signal: %v\n", sig)
		return o.removeObject(ctx, k8sClient, jobName)
	case <-waitTimeoutDone:
		err := o.removeObject(ctx, k8sClient, jobName)
		return errors.Join(errors.New("timeout deadline exceeded"), err)
	case err := <-watcherErrChan:
		if err != nil {
			return fmt.Errorf("error watching job pods: %w", err)
		}
	case err := <-jobCompletionErrChan:
		if err != nil {
			return fmt.Errorf("checking job completion: %w", err)
		}
		fmt.Fprintln(o.Out, "Job logs streaming finished.")
	}

	return nil
}

func (o *CreateOptions) removeObject(ctx context.Context, clientset kubernetes.Interface, jobName string) error {
	if o.RemoveObject {
		fmt.Fprintf(o.Out, "Stopping the job and cleaning up...\n")
		return clientset.BatchV1().Jobs(o.Namespace).Delete(ctx, jobName, metav1.DeleteOptions{
			PropagationPolicy: ptr.To(metav1.DeletePropagationBackground),
		})
	}
	return nil
}

func (o *CreateOptions) watchJobPods(ctx context.Context, clientGetter helpers.ClientGetter, jobName string) error {
	k8sClient, err := clientGetter.K8sClientset()
	if err != nil {
		return err
	}

	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	}

	watcher, err := k8sClient.CoreV1().Pods(o.Namespace).Watch(ctx, listOptions)
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			continue
		}

		if event.Type == watch.Added || event.Type == watch.Modified {
			switch pod.Status.Phase {
			case corev1.PodRunning, corev1.PodSucceeded, corev1.PodFailed:
				go o.streamLogsFromPod(ctx, clientGetter, pod)
			}
		}
	}
	return nil
}

func (o *CreateOptions) streamLogsFromPod(ctx context.Context, clientGetter helpers.ClientGetter, pod *corev1.Pod) {
	_, loaded := o.activeStreams.LoadOrStore(pod.Name, true)
	if loaded {
		// pod is already streaming logs
		return
	}

	for index := range pod.Spec.Containers {
		if pod.Spec.Containers[index].Name == builder.SlurmInitContainerName {
			continue
		}
		if len(o.StreamContainers) > 0 && !slices.Contains(o.StreamContainers, pod.Spec.Containers[index].Name) {
			continue
		}
		go func() {
			err := o.streamLogsFromPodContainer(ctx, clientGetter, pod, index)
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				fmt.Fprintf(o.Out, "Error streaming logs from pod %q container %q: %v\n", pod.Name, pod.Spec.Containers[index].Name, err)
			}
		}()
	}
}

func (o *CreateOptions) streamLogsFromPodContainer(ctx context.Context, clientGetter helpers.ClientGetter, pod *corev1.Pod, containerIndex int) error {
	k8sClient, err := clientGetter.K8sClientset()
	if err != nil {
		return err
	}

	container := pod.Spec.Containers[containerIndex]

	fmt.Fprintf(o.Out, "Starting log streaming for pod %q container %q...\n", pod.Name, container.Name)

	logOptions := &corev1.PodLogOptions{
		Container: container.Name,
		Follow:    pod.Status.Phase == corev1.PodRunning,
	}
	req := k8sClient.CoreV1().Pods(o.Namespace).GetLogs(pod.Name, logOptions)

	logStream, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer logStream.Close()

	_, err = io.Copy(o.Out, logStream)
	if err != nil {
		return err
	}

	return nil
}

func (o *CreateOptions) verifyJobFinished(ctx context.Context, clientset kubernetes.Interface, jobName string) error {
	watchOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", jobName),
	}

	watcher, err := clientset.BatchV1().Jobs(o.Namespace).Watch(ctx, watchOptions)
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		job, ok := event.Object.(*batchv1.Job)
		if !ok {
			continue
		}
		switch event.Type {
		case watch.Modified:
			for _, c := range job.Status.Conditions {
				if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
					return nil
				}
			}
		case watch.Deleted:
			return nil
		}
	}
	return nil
}
