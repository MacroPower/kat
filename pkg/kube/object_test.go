package kube_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/macropower/kat/pkg/kube"
)

func TestObject_GetAPIVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		object   kube.Object
		expected string
	}{
		{
			name: "valid apiVersion",
			object: kube.Object{
				"apiVersion": "apps/v1",
			},
			expected: "apps/v1",
		},
		{
			name: "core v1 apiVersion",
			object: kube.Object{
				"apiVersion": "v1",
			},
			expected: "v1",
		},
		{
			name: "missing apiVersion",
			object: kube.Object{
				"kind": "Pod",
			},
			expected: "",
		},
		{
			name: "nil apiVersion",
			object: kube.Object{
				"apiVersion": nil,
			},
			expected: "",
		},
		{
			name: "non-string apiVersion",
			object: kube.Object{
				"apiVersion": 123,
			},
			expected: "",
		},
		{
			name:     "empty object",
			object:   kube.Object{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.object.GetAPIVersion()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestObject_GetGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		object   kube.Object
		expected string
	}{
		{
			name: "apps group",
			object: kube.Object{
				"apiVersion": "apps/v1",
			},
			expected: "apps",
		},
		{
			name: "networking group",
			object: kube.Object{
				"apiVersion": "networking.k8s.io/v1",
			},
			expected: "networking.k8s.io",
		},
		{
			name: "core group (v1)",
			object: kube.Object{
				"apiVersion": "v1",
			},
			expected: "",
		},
		{
			name: "custom group with multiple slashes",
			object: kube.Object{
				"apiVersion": "custom.io/v1beta1/extra",
			},
			expected: "custom.io",
		},
		{
			name: "missing apiVersion",
			object: kube.Object{
				"kind": "Pod",
			},
			expected: "",
		},
		{
			name:     "empty object",
			object:   kube.Object{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.object.GetGroup()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestObject_GetKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		object   kube.Object
		expected string
	}{
		{
			name: "valid kind",
			object: kube.Object{
				"kind": "Pod",
			},
			expected: "Pod",
		},
		{
			name: "deployment kind",
			object: kube.Object{
				"kind": "Deployment",
			},
			expected: "Deployment",
		},
		{
			name: "missing kind",
			object: kube.Object{
				"apiVersion": "v1",
			},
			expected: "<empty>",
		},
		{
			name: "nil kind",
			object: kube.Object{
				"kind": nil,
			},
			expected: "<empty>",
		},
		{
			name: "non-string kind",
			object: kube.Object{
				"kind": 123,
			},
			expected: "<empty>",
		},
		{
			name:     "empty object",
			object:   kube.Object{},
			expected: "<empty>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.object.GetKind()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestObject_GetGroupKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		object   kube.Object
		expected string
	}{
		{
			name: "apps group with deployment",
			object: kube.Object{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
			},
			expected: "apps/Deployment",
		},
		{
			name: "core group with pod",
			object: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
			},
			expected: "Pod",
		},
		{
			name: "networking group with ingress",
			object: kube.Object{
				"apiVersion": "networking.k8s.io/v1",
				"kind":       "Ingress",
			},
			expected: "networking.k8s.io/Ingress",
		},
		{
			name: "missing group and kind",
			object: kube.Object{
				"metadata": map[string]any{"name": "test"},
			},
			expected: "<empty>",
		},
		{
			name: "missing kind with group",
			object: kube.Object{
				"apiVersion": "apps/v1",
			},
			expected: "apps/<empty>",
		},
		{
			name:     "empty object",
			object:   kube.Object{},
			expected: "<empty>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.object.GetGroupKind()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestObject_GetNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		object   kube.Object
		expected string
	}{
		{
			name: "valid namespace",
			object: kube.Object{
				"metadata": map[string]any{
					"namespace": "default",
					"name":      "test-pod",
				},
			},
			expected: "default",
		},
		{
			name: "custom namespace",
			object: kube.Object{
				"metadata": map[string]any{
					"namespace": "kube-system",
					"name":      "test-pod",
				},
			},
			expected: "kube-system",
		},
		{
			name: "missing namespace",
			object: kube.Object{
				"metadata": map[string]any{
					"name": "test-pod",
				},
			},
			expected: "",
		},
		{
			name: "missing metadata",
			object: kube.Object{
				"kind": "Pod",
			},
			expected: "",
		},
		{
			name: "nil namespace",
			object: kube.Object{
				"metadata": map[string]any{
					"namespace": nil,
					"name":      "test-pod",
				},
			},
			expected: "",
		},
		{
			name: "non-string namespace",
			object: kube.Object{
				"metadata": map[string]any{
					"namespace": 123,
					"name":      "test-pod",
				},
			},
			expected: "",
		},
		{
			name: "non-map metadata",
			object: kube.Object{
				"metadata": "invalid",
			},
			expected: "",
		},
		{
			name:     "empty object",
			object:   kube.Object{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.object.GetNamespace()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestObject_GetName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		object   kube.Object
		expected string
	}{
		{
			name: "valid name",
			object: kube.Object{
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			expected: "test-pod",
		},
		{
			name: "deployment name",
			object: kube.Object{
				"metadata": map[string]any{
					"name": "nginx-deployment",
				},
			},
			expected: "nginx-deployment",
		},
		{
			name: "missing name",
			object: kube.Object{
				"metadata": map[string]any{
					"namespace": "default",
				},
			},
			expected: "<empty>",
		},
		{
			name: "missing metadata",
			object: kube.Object{
				"kind": "Pod",
			},
			expected: "<empty>",
		},
		{
			name: "nil name",
			object: kube.Object{
				"metadata": map[string]any{
					"name": nil,
				},
			},
			expected: "<empty>",
		},
		{
			name: "non-string name",
			object: kube.Object{
				"metadata": map[string]any{
					"name": 123,
				},
			},
			expected: "<empty>",
		},
		{
			name: "non-map metadata",
			object: kube.Object{
				"metadata": "invalid",
			},
			expected: "<empty>",
		},
		{
			name:     "empty object",
			object:   kube.Object{},
			expected: "<empty>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.object.GetName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestObject_GetNamespacedName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		object   kube.Object
		expected string
	}{
		{
			name: "namespaced resource",
			object: kube.Object{
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			expected: "default/test-pod",
		},
		{
			name: "cluster-scoped resource",
			object: kube.Object{
				"metadata": map[string]any{
					"name": "test-node",
				},
			},
			expected: "test-node",
		},
		{
			name: "empty namespace",
			object: kube.Object{
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "",
				},
			},
			expected: "test-pod",
		},
		{
			name: "custom namespace",
			object: kube.Object{
				"metadata": map[string]any{
					"name":      "nginx-deployment",
					"namespace": "kube-system",
				},
			},
			expected: "kube-system/nginx-deployment",
		},
		{
			name: "missing metadata",
			object: kube.Object{
				"kind": "Pod",
			},
			expected: "<empty>",
		},
		{
			name:     "empty object",
			object:   kube.Object{},
			expected: "<empty>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.object.GetNamespacedName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestObjectEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		objectA  kube.Object
		objectB  kube.Object
		name     string
		expected bool
	}{
		{
			name: "identical objects",
			objectA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			objectB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			expected: true,
		},
		{
			name: "different apiVersion",
			objectA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			objectB: kube.Object{
				"apiVersion": "apps/v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			expected: false,
		},
		{
			name: "different kind",
			objectA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			objectB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			expected: false,
		},
		{
			name: "different namespace",
			objectA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			objectB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "kube-system",
				},
			},
			expected: false,
		},
		{
			name: "different name",
			objectA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			objectB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "another-pod",
					"namespace": "default",
				},
			},
			expected: false,
		},
		{
			name: "cluster-scoped resources",
			objectA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Node",
				"metadata": map[string]any{
					"name": "node-1",
				},
			},
			objectB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Node",
				"metadata": map[string]any{
					"name": "node-1",
				},
			},
			expected: true,
		},
		{
			name: "one namespaced one cluster-scoped",
			objectA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			objectB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name": "test-pod",
				},
			},
			expected: false,
		},
		{
			name: "objects with additional fields",
			objectA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
					"labels": map[string]any{
						"app": "test",
					},
				},
				"spec": map[string]any{
					"containers": []any{},
				},
			},
			objectB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
					"annotations": map[string]any{
						"key": "value",
					},
				},
				"status": map[string]any{
					"phase": "Running",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := kube.ObjectEqual(&tt.objectA, &tt.objectB)
			assert.Equal(t, tt.expected, result)
		})
	}
}
