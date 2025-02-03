// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=template.fn.crossplane.io
// +versionName=v1beta1
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// This isn't a custom resource, in the sense that we never install its CRD.
// It is a KRM-like object, so we generate a CRD to describe its schema.

// Input can be used to provide input to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type ResourceGraph struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The status of the ResourceGraph.
	Status runtime.RawExtension `json:"status,omitempty"`

	// The resources that are part of the ResourceGraph.
	//
	// +kubebuilder:validation:Optional
	Resources []*Resource `json:"resources,omitempty"`
}

type Resource struct {
	// +kubebuilder:validation:Required
	ID string `json:"id,omitempty"`
	// +kubebuilder:validation:Required
	Template runtime.RawExtension `json:"template,omitempty"`
	// +kubebuilder:validation:Optional
	ReadyWhen []string `json:"readyWhen,omitempty"`
	// +kubebuilder:validation:Optional
	IncludeWhen []string `json:"includeWhen,omitempty"`
}
