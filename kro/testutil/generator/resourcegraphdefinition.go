// Copyright 2025 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generator

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane-contrib/function-kro/input/v1beta1"
)

// ResourceGraphDefinitionOption is a functional option for ResourceGraph
type ResourceGraphDefinitionOption func(*v1beta1.ResourceGraph)

// SchemaOption is a functional option for Schema configuration.
// In function-kro, schema is handled externally, so this is kept for
// test compatibility but the schema data is stored on the ResourceGraph
// as annotations or status for the test to extract.
type SchemaOption func(map[string]interface{})

// NewResourceGraphDefinition creates a new ResourceGraph with the given name and options.
// This is adapted from upstream KRO's generator to work with function-kro's
// v1beta1.ResourceGraph type instead of v1alpha1.ResourceGraphDefinition.
func NewResourceGraphDefinition(name string, opts ...ResourceGraphDefinitionOption) *v1beta1.ResourceGraph {
	rg := &v1beta1.ResourceGraph{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	for _, opt := range opts {
		opt(rg)
	}
	return rg
}

// WithSchema sets the status schema of the ResourceGraph.
// In function-kro, the XR schema is resolved externally (from Crossplane),
// so this only sets the status expressions. The spec/status maps from
// upstream are adapted: status becomes the ResourceGraph.Status field,
// and spec information is stored as annotations for test extraction.
func WithSchema(kind, version string, spec, status map[string]interface{}, opts ...SchemaOption) ResourceGraphDefinitionOption {
	return func(rg *v1beta1.ResourceGraph) {
		if status != nil {
			rawStatus, err := json.Marshal(status)
			if err != nil {
				panic(err)
			}
			rg.Status = runtime.RawExtension{
				Object: &unstructured.Unstructured{Object: status},
				Raw:    rawStatus,
			}
		}

		// Store schema metadata as annotations so tests can extract it
		if rg.Annotations == nil {
			rg.Annotations = make(map[string]string)
		}
		rg.Annotations["test-schema-kind"] = kind
		rg.Annotations["test-schema-version"] = version
		if spec != nil {
			rawSpec, err := json.Marshal(spec)
			if err != nil {
				panic(err)
			}
			rg.Annotations["test-schema-spec"] = string(rawSpec)
		}

		// Apply schema options
		for _, opt := range opts {
			opt(status)
		}
	}
}

// WithExternalRef adds an external reference to the ResourceGraph.
func WithExternalRef(
	id string,
	externalRef *v1beta1.ExternalRef,
	readyWhen []string,
	includeWhen []string,
) ResourceGraphDefinitionOption {
	return func(rg *v1beta1.ResourceGraph) {
		rg.Resources = append(rg.Resources, &v1beta1.Resource{
			ID:          id,
			ReadyWhen:   readyWhen,
			IncludeWhen: includeWhen,
			ExternalRef: externalRef,
		})
	}
}

// WithExternalRefAndForEach adds an external reference with forEach iterators.
// This is an invalid combination and should fail validation - used for testing.
func WithExternalRefAndForEach(
	id string,
	externalRef *v1beta1.ExternalRef,
	forEach []v1beta1.ForEachDimension,
) ResourceGraphDefinitionOption {
	return func(rg *v1beta1.ResourceGraph) {
		rg.Resources = append(rg.Resources, &v1beta1.Resource{
			ID:          id,
			ExternalRef: externalRef,
			ForEach:     forEach,
		})
	}
}

// WithResource adds a resource to the ResourceGraph with the given name and definition.
func WithResource(
	id string,
	template map[string]interface{},
	readyWhen []string,
	includeWhen []string,
) ResourceGraphDefinitionOption {
	return func(rg *v1beta1.ResourceGraph) {
		raw, err := json.Marshal(template)
		if err != nil {
			panic(err)
		}
		rg.Resources = append(rg.Resources, &v1beta1.Resource{
			ID:          id,
			ReadyWhen:   readyWhen,
			IncludeWhen: includeWhen,
			Template: runtime.RawExtension{
				Object: &unstructured.Unstructured{Object: template},
				Raw:    raw,
			},
		})
	}
}

// WithTypes returns a SchemaOption that stores types metadata.
// In function-kro, custom types are not directly supported in the same way
// as upstream KRO, but this is preserved for test compatibility.
func WithTypes(types map[string]interface{}) SchemaOption {
	return func(_ map[string]interface{}) {
		// Types are not directly used in function-kro's ResourceGraph
		// but may be needed for test scenarios
	}
}

// WithScope returns a SchemaOption that stores scope metadata.
// In function-kro, scope is always namespace-scoped from our perspective.
func WithScope(scope string) SchemaOption {
	return func(_ map[string]interface{}) {
		// Scope is always namespace-scoped in function-kro
	}
}

// WithResourceCollection adds a collection resource with forEach iterators.
func WithResourceCollection(
	id string,
	template map[string]interface{},
	forEach []v1beta1.ForEachDimension,
	readyWhen []string,
	includeWhen []string,
) ResourceGraphDefinitionOption {
	return func(rg *v1beta1.ResourceGraph) {
		raw, err := json.Marshal(template)
		if err != nil {
			panic(err)
		}
		rg.Resources = append(rg.Resources, &v1beta1.Resource{
			ID:          id,
			ReadyWhen:   readyWhen,
			IncludeWhen: includeWhen,
			ForEach:     forEach,
			Template: runtime.RawExtension{
				Object: &unstructured.Unstructured{Object: template},
				Raw:    raw,
			},
		})
	}
}
