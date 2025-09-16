package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	jobsetapi "sigs.k8s.io/jobset/api/jobset/v1alpha2"
)

// JobSetTemplateSpec describes the data a jobset should have when created from a template
type JobSetTemplateSpec struct {
	// Standard object's metadata.
	//
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the jobset.
	//
	// +kubebuilder:validation:Required
	Spec jobsetapi.JobSetSpec `json:"spec"`
}

// +genclient
// +kubebuilder:object:root=true

// JobSetTemplate is the Schema for the jobsettemplate API
type JobSetTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Template defines the jobs that will be created from this jobset template.
	//
	// +kubebuilder:validation:Required
	Template JobSetTemplateSpec `json:"template,omitempty"`
}

// +kubebuilder:object:root=true

// JobSetTemplateList contains a list of JobSetTemplate
type JobSetTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []JobSetTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JobSetTemplate{}, &JobSetTemplateList{})
}
