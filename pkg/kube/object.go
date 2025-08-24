package kube

import (
	"strings"
)

type ResourceMetadata struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
}

type Object map[string]any

func (o Object) GetMetadata() ResourceMetadata {
	return ResourceMetadata{
		APIVersion: o.GetAPIVersion(),
		Kind:       o.GetKind(),
		Namespace:  o.GetNamespace(),
		Name:       o.GetName(),
	}
}

// GetAPIVersion returns the apiVersion of the object.
// If apiVersion is not set, it returns an empty string.
func (o Object) GetAPIVersion() string {
	if apiVersion, ok := o["apiVersion"]; ok {
		if version, ok := apiVersion.(string); ok {
			return version
		}
	}

	return ""
}

// GetGroup returns the group of the object, which is the first part of the apiVersion.
// If the group is not set, it assumes "core".
func (o Object) GetGroup() string {
	if apiVersion := o.GetAPIVersion(); apiVersion != "" {
		parts := strings.Split(apiVersion, "/")
		if len(parts) > 1 {
			return parts[0]
		}
	}

	return "core"
}

// GetKind returns the kind of the object.
// If the kind is not set, it returns "<empty>".
func (o Object) GetKind() string {
	if kind, ok := o["kind"]; ok {
		if k, ok := kind.(string); ok {
			return k
		}
	}

	return "<empty>"
}

// GetGroupKind returns the group and kind of the object in the format `group/kind`.
// If the group is not set, it returns just the kind.
func (o Object) GetGroupKind() string {
	group := o.GetGroup()
	kind := o.GetKind()
	if group != "" {
		return group + "/" + kind
	}

	return kind
}

// GetNamespace returns the namespace of the object.
// If the namespace is not set, it returns an empty string.
func (o Object) GetNamespace() string {
	if metadata, ok := o["metadata"].(map[string]any); ok {
		if namespace, ok := metadata["namespace"]; ok {
			if ns, ok := namespace.(string); ok {
				return ns
			}
		}
	}

	return ""
}

// GetName returns the name of the object.
// If the name is not set, it returns "<empty>".
func (o Object) GetName() string {
	if metadata, ok := o["metadata"].(map[string]any); ok {
		if name, ok := metadata["name"]; ok {
			if n, ok := name.(string); ok {
				return n
			}
		}
	}

	return "<empty>"
}

func (o Object) GetNamespacedName() string {
	ns := o.GetNamespace()
	name := o.GetName()
	if ns != "" {
		return ns + "/" + name
	}

	return name
}

// ObjectEqual compares two [Object] instances for equality based on their
// apiVersion, kind, namespace, and name.
func ObjectEqual(a, b *Object) bool {
	if a.GetAPIVersion() != b.GetAPIVersion() {
		return false
	}
	if a.GetKind() != b.GetKind() {
		return false
	}
	if a.GetNamespace() != b.GetNamespace() {
		return false
	}
	if a.GetName() != b.GetName() {
		return false
	}

	return true
}
