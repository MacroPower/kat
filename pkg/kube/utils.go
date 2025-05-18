package kube

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

func UnstructuredEqual(a, b *unstructured.Unstructured) bool {
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
