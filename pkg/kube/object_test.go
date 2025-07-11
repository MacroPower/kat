package kube_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/macropower/kat/pkg/kube"
)

func TestObject_GetAPIVersion(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input kube.Object
		want  string
	}{
		"valid apiVersion": {
			input: kube.Object{
				"apiVersion": "apps/v1",
			},
			want: "apps/v1",
		},
		"core v1 apiVersion": {
			input: kube.Object{
				"apiVersion": "v1",
			},
			want: "v1",
		},
		"missing apiVersion": {
			input: kube.Object{
				"kind": "Pod",
			},
			want: "",
		},
		"nil apiVersion": {
			input: kube.Object{
				"apiVersion": nil,
			},
			want: "",
		},
		"non-string apiVersion": {
			input: kube.Object{
				"apiVersion": 123,
			},
			want: "",
		},
		"empty object": {
			input: kube.Object{},
			want:  "",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.input.GetAPIVersion()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestObject_GetGroup(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input kube.Object
		want  string
	}{
		"apps group": {
			input: kube.Object{
				"apiVersion": "apps/v1",
			},
			want: "apps",
		},
		"networking group": {
			input: kube.Object{
				"apiVersion": "networking.k8s.io/v1",
			},
			want: "networking.k8s.io",
		},
		"core group (v1)": {
			input: kube.Object{
				"apiVersion": "v1",
			},
			want: "core",
		},
		"custom group with multiple slashes": {
			input: kube.Object{
				"apiVersion": "custom.io/v1beta1/extra",
			},
			want: "custom.io",
		},
		"missing apiVersion": {
			input: kube.Object{
				"kind": "Pod",
			},
			want: "core",
		},
		"empty object": {
			input: kube.Object{},
			want:  "core",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.input.GetGroup()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestObject_GetKind(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input kube.Object
		want  string
	}{
		"valid kind": {
			input: kube.Object{
				"kind": "Pod",
			},
			want: "Pod",
		},
		"deployment kind": {
			input: kube.Object{
				"kind": "Deployment",
			},
			want: "Deployment",
		},
		"missing kind": {
			input: kube.Object{
				"apiVersion": "v1",
			},
			want: "<empty>",
		},
		"nil kind": {
			input: kube.Object{
				"kind": nil,
			},
			want: "<empty>",
		},
		"non-string kind": {
			input: kube.Object{
				"kind": 123,
			},
			want: "<empty>",
		},
		"empty object": {
			input: kube.Object{},
			want:  "<empty>",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.input.GetKind()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestObject_GetGroupKind(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input kube.Object
		want  string
	}{
		"apps group with deployment": {
			input: kube.Object{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
			},
			want: "apps/Deployment",
		},
		"core group with pod": {
			input: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
			},
			want: "core/Pod",
		},
		"networking group with ingress": {
			input: kube.Object{
				"apiVersion": "networking.k8s.io/v1",
				"kind":       "Ingress",
			},
			want: "networking.k8s.io/Ingress",
		},
		"missing group and kind": {
			input: kube.Object{
				"metadata": map[string]any{"name": "test"},
			},
			want: "core/<empty>",
		},
		"missing kind with group": {
			input: kube.Object{
				"apiVersion": "apps/v1",
			},
			want: "apps/<empty>",
		},
		"empty object": {
			input: kube.Object{},
			want:  "core/<empty>",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.input.GetGroupKind()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestObject_GetNamespace(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input kube.Object
		want  string
	}{
		"valid namespace": {
			input: kube.Object{
				"metadata": map[string]any{
					"namespace": "default",
					"name":      "test-pod",
				},
			},
			want: "default",
		},
		"custom namespace": {
			input: kube.Object{
				"metadata": map[string]any{
					"namespace": "kube-system",
					"name":      "test-pod",
				},
			},
			want: "kube-system",
		},
		"missing namespace": {
			input: kube.Object{
				"metadata": map[string]any{
					"name": "test-pod",
				},
			},
			want: "",
		},
		"missing metadata": {
			input: kube.Object{
				"kind": "Pod",
			},
			want: "",
		},
		"nil namespace": {
			input: kube.Object{
				"metadata": map[string]any{
					"namespace": nil,
					"name":      "test-pod",
				},
			},
			want: "",
		},
		"non-string namespace": {
			input: kube.Object{
				"metadata": map[string]any{
					"namespace": 123,
					"name":      "test-pod",
				},
			},
			want: "",
		},
		"non-map metadata": {
			input: kube.Object{
				"metadata": "invalid",
			},
			want: "",
		},
		"empty object": {
			input: kube.Object{},
			want:  "",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.input.GetNamespace()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestObject_GetName(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input kube.Object
		want  string
	}{
		"valid name": {
			input: kube.Object{
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			want: "test-pod",
		},
		"deployment name": {
			input: kube.Object{
				"metadata": map[string]any{
					"name": "nginx-deployment",
				},
			},
			want: "nginx-deployment",
		},
		"missing name": {
			input: kube.Object{
				"metadata": map[string]any{
					"namespace": "default",
				},
			},
			want: "<empty>",
		},
		"missing metadata": {
			input: kube.Object{
				"kind": "Pod",
			},
			want: "<empty>",
		},
		"nil name": {
			input: kube.Object{
				"metadata": map[string]any{
					"name": nil,
				},
			},
			want: "<empty>",
		},
		"non-string name": {
			input: kube.Object{
				"metadata": map[string]any{
					"name": 123,
				},
			},
			want: "<empty>",
		},
		"non-map metadata": {
			input: kube.Object{
				"metadata": "invalid",
			},
			want: "<empty>",
		},
		"empty object": {
			input: kube.Object{},
			want:  "<empty>",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.input.GetName()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestObject_GetNamespacedName(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input kube.Object
		want  string
	}{
		"namespaced resource": {
			input: kube.Object{
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			want: "default/test-pod",
		},
		"cluster-scoped resource": {
			input: kube.Object{
				"metadata": map[string]any{
					"name": "test-node",
				},
			},
			want: "test-node",
		},
		"empty namespace": {
			input: kube.Object{
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "",
				},
			},
			want: "test-pod",
		},
		"custom namespace": {
			input: kube.Object{
				"metadata": map[string]any{
					"name":      "nginx-deployment",
					"namespace": "kube-system",
				},
			},
			want: "kube-system/nginx-deployment",
		},
		"missing metadata": {
			input: kube.Object{
				"kind": "Pod",
			},
			want: "<empty>",
		},
		"empty object": {
			input: kube.Object{},
			want:  "<empty>",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.input.GetNamespacedName()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestObjectEqual(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		inputA kube.Object
		inputB kube.Object
		want   bool
	}{
		"identical objects": {
			inputA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			inputB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			want: true,
		},
		"different apiVersion": {
			inputA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			inputB: kube.Object{
				"apiVersion": "apps/v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			want: false,
		},
		"different kind": {
			inputA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			inputB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			want: false,
		},
		"different namespace": {
			inputA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			inputB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "kube-system",
				},
			},
			want: false,
		},
		"different name": {
			inputA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			inputB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "another-pod",
					"namespace": "default",
				},
			},
			want: false,
		},
		"cluster-scoped resources": {
			inputA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Node",
				"metadata": map[string]any{
					"name": "node-1",
				},
			},
			inputB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Node",
				"metadata": map[string]any{
					"name": "node-1",
				},
			},
			want: true,
		},
		"one namespaced one cluster-scoped": {
			inputA: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			inputB: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name": "test-pod",
				},
			},
			want: false,
		},
		"objects with additional fields": {
			inputA: kube.Object{
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
			inputB: kube.Object{
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
			want: true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := kube.ObjectEqual(&tc.inputA, &tc.inputB)
			assert.Equal(t, tc.want, got)
		})
	}
}
