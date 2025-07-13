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

package builder

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	jobsetapi "sigs.k8s.io/jobset/api/jobset/v1alpha2"
)

type jobSetBuilder struct {
	*Builder
}

var _ builder = (*jobSetBuilder)(nil)

func (b *jobSetBuilder) build(ctx context.Context) (runtime.Object, []runtime.Object, error) {
	template, err := b.kjobctlClientset.KjobctlV1alpha1().JobSetTemplates(b.profile.Namespace).
		Get(ctx, string(b.mode.Template), metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	objectMeta, err := b.buildObjectMeta(template.Template.ObjectMeta, false)
	if err != nil {
		return nil, nil, err
	}

	jobSet := &jobsetapi.JobSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "JobSet",
			APIVersion: "jobset.x-k8s.io/v1alpha2",
		},
		ObjectMeta: objectMeta,
		Spec:       template.Template.Spec,
	}

	b.buildJobSetSpec(&jobSet.Spec)

	return jobSet, nil, nil
}

func newJobSetBuilder(b *Builder) *jobSetBuilder {
	return &jobSetBuilder{Builder: b}
}
