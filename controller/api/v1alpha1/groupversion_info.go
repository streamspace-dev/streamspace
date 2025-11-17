// Package v1alpha1 contains API Schema definitions for the stream v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=stream.space
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	// IMPORTANT: Must match the API group in CRD manifests (stream.space)
	GroupVersion = schema.GroupVersion{Group: "stream.space", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
