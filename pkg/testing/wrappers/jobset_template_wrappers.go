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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	jobsetapi "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	"sigs.k8s.io/kjob/apis/v1alpha1"
)

// JobSetTemplateWrapper wraps a JobSetTemplate.
type JobSetTemplateWrapper struct{ v1alpha1.JobSetTemplate }

// MakeJobSetTemplate creates a wrapper for a JobSetTemplate
func MakeJobSetTemplate(name, ns string) *JobSetTemplateWrapper {
	return &JobSetTemplateWrapper{
		JobSetTemplate: v1alpha1.JobSetTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
		},
	}
}

// Obj returns the inner JobSetTemplate.
func (w *JobSetTemplateWrapper) Obj() *v1alpha1.JobSetTemplate {
	return &w.JobSetTemplate
}

// Clone JobSetTemplateWrapper.
func (w *JobSetTemplateWrapper) Clone() *JobSetTemplateWrapper {
	return &JobSetTemplateWrapper{
		JobSetTemplate: *w.DeepCopy(),
	}
}

// Label sets the label key and value.
func (w *JobSetTemplateWrapper) Label(key, value string) *JobSetTemplateWrapper {
	if w.Template.Labels == nil {
		w.Template.Labels = make(map[string]string)
	}
	w.Template.Labels[key] = value
	return w
}

// Annotation sets the label key and value.
func (w *JobSetTemplateWrapper) Annotation(key, value string) *JobSetTemplateWrapper {
	if w.Template.Annotations == nil {
		w.Template.Annotations = make(map[string]string)
	}
	w.Template.Annotations[key] = value
	return w
}

// Spec set entrypoint.
func (w *JobSetTemplateWrapper) Spec(spec jobsetapi.JobSetSpec) *JobSetTemplateWrapper {
	w.Template.Spec = spec
	return w
}
