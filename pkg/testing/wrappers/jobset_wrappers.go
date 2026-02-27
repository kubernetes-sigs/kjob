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

package wrappers

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	jobsetapi "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	kueueconstants "sigs.k8s.io/kueue/pkg/controller/constants"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	"sigs.k8s.io/kjob/pkg/constants"
)

// JobSetWrapper wraps a JobSet.
type JobSetWrapper struct{ *jobsetapi.JobSet }

// MakeJobset creates a wrapper for a JobSet
func MakeJobSet(name, ns string) *JobSetWrapper {
	return &JobSetWrapper{
		&jobsetapi.JobSet{
			TypeMeta: metav1.TypeMeta{Kind: "JobSet", APIVersion: "jobset.x-k8s.io/v1alpha2"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
		},
	}
}

// Obj returns the inner JobSet.
func (j *JobSetWrapper) Obj() *jobsetapi.JobSet {
	return j.JobSet
}

// GenerateName updates generateName.
func (j *JobSetWrapper) GenerateName(v string) *JobSetWrapper {
	j.ObjectMeta.GenerateName = v
	return j
}

// CreationTimestamp sets the .metadata.creationTimestamp
func (j *JobSetWrapper) CreationTimestamp(t time.Time) *JobSetWrapper {
	j.ObjectMeta.CreationTimestamp = metav1.NewTime(t)
	return j
}

// Profile sets the profile label.
func (j *JobSetWrapper) Profile(v string) *JobSetWrapper {
	return j.Label(constants.ProfileLabel, v)
}

// Mode sets the profile label.
func (j *JobSetWrapper) Mode(v v1alpha1.ApplicationProfileMode) *JobSetWrapper {
	return j.Label(constants.ModeLabel, string(v))
}

// LocalQueue sets the localqueue label.
func (j *JobSetWrapper) LocalQueue(v string) *JobSetWrapper {
	return j.Label(kueueconstants.QueueLabel, v)
}

// Label sets the label key and value.
func (j *JobSetWrapper) Label(key, value string) *JobSetWrapper {
	if j.Labels == nil {
		j.Labels = make(map[string]string)
	}
	j.Labels[key] = value
	return j
}

// Annotation sets the label key and value.
func (j *JobSetWrapper) Annotation(key, value string) *JobSetWrapper {
	if j.Annotations == nil {
		j.Annotations = make(map[string]string)
	}
	j.Annotations[key] = value
	return j
}

// Spec set job spec.
func (j *JobSetWrapper) Spec(spec jobsetapi.JobSetSpec) *JobSetWrapper {
	j.JobSet.Spec = spec
	return j
}

// Priority sets the workload priority class label.
func (j *JobSetWrapper) Priority(v string) *JobSetWrapper {
	return j.Label(kueueconstants.WorkloadPriorityClassLabel, v)
}

// Replicas sets the replica count for a named replicated job.
func (j *JobSetWrapper) Replicas(name string, replicas int32) *JobSetWrapper {
	for i := range j.JobSet.Spec.ReplicatedJobs {
		if j.JobSet.Spec.ReplicatedJobs[i].Name == name {
			j.JobSet.Spec.ReplicatedJobs[i].Replicas = replicas
			return j
		}
	}
	return j
}

// Command sets the command on the first container of the first replicated job.
func (j *JobSetWrapper) Command(cmd []string) *JobSetWrapper {
	if len(j.JobSet.Spec.ReplicatedJobs) > 0 {
		containers := j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Spec.Containers
		if len(containers) > 0 {
			j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Spec.Containers[0].Command = cmd
		}
	}
	return j
}

// Request sets a resource request on the first container of the first replicated job.
func (j *JobSetWrapper) Request(key corev1.ResourceName, value resource.Quantity) *JobSetWrapper {
	if len(j.JobSet.Spec.ReplicatedJobs) > 0 {
		containers := j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Spec.Containers
		if len(containers) > 0 {
			if j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Spec.Containers[0].Resources.Requests == nil {
				j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Spec.Containers[0].Resources.Requests = corev1.ResourceList{}
			}
			j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Spec.Containers[0].Resources.Requests[key] = value
		}
	}
	return j
}

// PodTemplateLabel sets a label on the pod template of the first replicated job.
func (j *JobSetWrapper) PodTemplateLabel(key, value string) *JobSetWrapper {
	if len(j.JobSet.Spec.ReplicatedJobs) > 0 {
		if j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Labels == nil {
			j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Labels = make(map[string]string)
		}
		j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Labels[key] = value
	}
	return j
}

// WithEnvVar adds an env var to all containers in all replicated jobs.
func (j *JobSetWrapper) WithEnvVar(envVar corev1.EnvVar) *JobSetWrapper {
	for i := range j.JobSet.Spec.ReplicatedJobs {
		for c := range j.JobSet.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers {
			j.JobSet.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[c].Env =
				append(j.JobSet.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[c].Env, envVar)
		}
	}
	return j
}

// PodTemplateAnnotation sets an annotation on the pod template of the first replicated job.
func (j *JobSetWrapper) PodTemplateAnnotation(key, value string) *JobSetWrapper {
	if len(j.JobSet.Spec.ReplicatedJobs) > 0 {
		if j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Annotations == nil {
			j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Annotations = make(map[string]string)
		}
		j.JobSet.Spec.ReplicatedJobs[0].Template.Spec.Template.Annotations[key] = value
	}
	return j
}
