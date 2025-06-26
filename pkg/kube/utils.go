package kube

func UnstructuredEqual(a, b *Object) bool {
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
