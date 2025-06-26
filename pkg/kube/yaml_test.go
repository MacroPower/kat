package kube_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/kube"
)

const deploymentObject = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    foo: bar
spec:
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx
        ports:
        - containerPort: 80
`

const invalidYAML = `
apiVersion: v1
	kind: Deployment
`

func TestSplitYAML_SingleObject(t *testing.T) {
	t.Parallel()

	objs, err := kube.SplitYAML([]byte(deploymentObject))
	require.NoError(t, err)
	assert.Len(t, objs, 1)
}

func TestSplitYAML_MultipleObjects(t *testing.T) {
	t.Parallel()

	objs, err := kube.SplitYAML([]byte(deploymentObject + "\n---\n" + deploymentObject))
	require.NoError(t, err)
	assert.Len(t, objs, 2)
}

func TestSplitYAML_TrailingNewLines(t *testing.T) {
	t.Parallel()

	objs, err := kube.SplitYAML([]byte("\n\n\n---" + deploymentObject))
	require.NoError(t, err)
	assert.Len(t, objs, 1)
}

func TestSplitYAML_InvalidYAML(t *testing.T) {
	t.Parallel()

	objs, err := kube.SplitYAML([]byte(invalidYAML))
	require.Error(t, err)
	require.ErrorIs(t, err, kube.ErrInvalidKubeResource)
	assert.Empty(t, objs)
}
