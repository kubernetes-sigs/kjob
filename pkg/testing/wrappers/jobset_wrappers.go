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

package wrappers

import (
	"time"

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
	j.JobSet.ObjectMeta.CreationTimestamp = metav1.NewTime(t)
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
	j.ObjectMeta.Labels[key] = value
	return j
}

// Annotation sets the label key and value.
func (j *JobSetWrapper) Annotation(key, value string) *JobSetWrapper {
	if j.Annotations == nil {
		j.Annotations = make(map[string]string)
	}
	j.ObjectMeta.Annotations[key] = value
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
