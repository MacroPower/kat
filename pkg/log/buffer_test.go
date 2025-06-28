package log_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/log"
)

func TestCircularBuffer_NewCircularBuffer(t *testing.T) {
	t.Parallel()

	// Test with valid capacity
	cb := log.NewCircularBuffer(10)
	if cb.Capacity() != 10 {
		t.Errorf("expected capacity 10, got %d", cb.Capacity())
	}
	if cb.Size() != 0 {
		t.Errorf("expected size 0, got %d", cb.Size())
	}
	if cb.IsFull() {
		t.Error("expected buffer to not be full")
	}

	// Test with zero capacity (should default to 100)
	cb = log.NewCircularBuffer(0)
	if cb.Capacity() != 100 {
		t.Errorf("expected default capacity 100, got %d", cb.Capacity())
	}

	// Test with negative capacity (should default to 100)
	cb = log.NewCircularBuffer(-5)
	if cb.Capacity() != 100 {
		t.Errorf("expected default capacity 100, got %d", cb.Capacity())
	}
}

func TestCircularBuffer_Write(t *testing.T) {
	t.Parallel()

	cb := log.NewCircularBuffer(3)

	// Test writing to empty buffer
	n, err := cb.Write([]byte("entry1"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 6 {
		t.Errorf("expected 6 bytes written, got %d", n)
	}
	if cb.Size() != 1 {
		t.Errorf("expected size 1, got %d", cb.Size())
	}

	// Test writing empty data
	n, err = cb.Write([]byte{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes written, got %d", n)
	}
	if cb.Size() != 1 {
		t.Errorf("expected size 1, got %d", cb.Size())
	}

	// Fill the buffer
	_, err = cb.Write([]byte("entry2"))
	require.NoError(t, err)
	_, err = cb.Write([]byte("entry3"))
	require.NoError(t, err)

	if !cb.IsFull() {
		t.Error("expected buffer to be full")
	}
	if cb.Size() != 3 {
		t.Errorf("expected size 3, got %d", cb.Size())
	}

	// Write when buffer is full (should overwrite oldest)
	n, err = cb.Write([]byte("entry4"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 6 {
		t.Errorf("expected 6 bytes written, got %d", n)
	}
	if cb.Size() != 3 {
		t.Errorf("expected size 3, got %d", cb.Size())
	}
}

func TestCircularBuffer_Entries(t *testing.T) {
	t.Parallel()

	cb := log.NewCircularBuffer(3)

	// Test empty buffer
	entries := cb.Entries()
	if entries != nil {
		t.Error("expected nil entries for empty buffer")
	}

	// Add entries
	var err error
	_, err = cb.Write([]byte("first"))
	require.NoError(t, err)
	_, err = cb.Write([]byte("second"))
	require.NoError(t, err)
	_, err = cb.Write([]byte("third"))
	require.NoError(t, err)

	entries = cb.Entries()
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	expected := []string{"first", "second", "third"}
	for i, entry := range entries {
		if string(entry) != expected[i] {
			t.Errorf("expected entry %d to be %q, got %q", i, expected[i], string(entry))
		}
	}

	// Test overwriting (circular behavior)
	_, err = cb.Write([]byte("fourth"))
	require.NoError(t, err)
	_, err = cb.Write([]byte("fifth"))
	require.NoError(t, err)

	entries = cb.Entries()
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	expected = []string{"third", "fourth", "fifth"}
	for i, entry := range entries {
		if string(entry) != expected[i] {
			t.Errorf("expected entry %d to be %q, got %q", i, expected[i], string(entry))
		}
	}

	// Test that returned entries are safe to modify
	entries[0][0] = 'X'
	originalEntries := cb.Entries()
	if string(originalEntries[0]) != "third" {
		t.Error("modifying returned entries affected original buffer")
	}
}

func TestCircularBuffer_Clear(t *testing.T) {
	t.Parallel()

	cb := log.NewCircularBuffer(3)

	// Add some entries
	var err error
	_, err = cb.Write([]byte("entry1"))
	require.NoError(t, err)
	_, err = cb.Write([]byte("entry2"))
	require.NoError(t, err)

	if cb.Size() != 2 {
		t.Errorf("expected size 2, got %d", cb.Size())
	}

	// Clear the buffer
	cb.Clear()

	if cb.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", cb.Size())
	}
	if cb.IsFull() {
		t.Error("expected buffer to not be full after clear")
	}

	entries := cb.Entries()
	if entries != nil {
		t.Error("expected nil entries after clear")
	}
}

func TestCircularBuffer_WriteTo(t *testing.T) {
	t.Parallel()

	cb := log.NewCircularBuffer(3)

	// Test empty buffer
	var buf bytes.Buffer
	n, err := cb.WriteTo(&buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes written, got %d", n)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty buffer, got %d bytes", buf.Len())
	}

	// Add entries and test WriteTo
	_, err = cb.Write([]byte("line1\n"))
	require.NoError(t, err)
	_, err = cb.Write([]byte("line2\n"))
	require.NoError(t, err)
	_, err = cb.Write([]byte("line3\n"))
	require.NoError(t, err)

	buf.Reset()
	n, err = cb.WriteTo(&buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 18 { // 6 bytes per line * 3 lines
		t.Errorf("expected 18 bytes written, got %d", n)
	}

	expected := "line1\nline2\nline3\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}

	// Test with circular overwrite
	_, err = cb.Write([]byte("line4\n"))
	require.NoError(t, err)
	_, err = cb.Write([]byte("line5\n"))
	require.NoError(t, err)

	buf.Reset()
	_, err = cb.WriteTo(&buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected = "line3\nline4\nline5\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestCircularBuffer_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	cb := log.NewCircularBuffer(100)

	// Start multiple goroutines writing to the buffer
	done := make(chan bool, 10)

	for range 10 {
		go func() {
			for range 50 {
				_, err := cb.Write([]byte(strings.Repeat("x", 10)))
				assert.NoError(t, err)
			}
			done <- true
		}()
	}

	// Start goroutines reading from the buffer
	for range 5 {
		go func() {
			for range 20 {
				cb.Entries()
				cb.Size()
				cb.IsFull()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for range 15 {
		<-done
	}

	// Buffer should be full after all writes
	if !cb.IsFull() {
		t.Error("expected buffer to be full after concurrent writes")
	}
	if cb.Size() != 100 {
		t.Errorf("expected size 100, got %d", cb.Size())
	}
}

func TestCircularBuffer_AsIOWriter(t *testing.T) {
	t.Parallel()

	cb := log.NewCircularBuffer(5)

	// Test that it can be used as an io.Writer
	var writer any = cb
	if _, ok := writer.(interface{ Write(p []byte) (int, error) }); !ok {
		t.Error("CircularBuffer does not implement io.Writer interface")
	}

	// Test typical usage as a writer
	data := [][]byte{
		[]byte("log entry 1\n"),
		[]byte("log entry 2\n"),
		[]byte("log entry 3\n"),
	}

	for _, entry := range data {
		n, err := cb.Write(entry)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if n != len(entry) {
			t.Errorf("expected %d bytes written, got %d", len(entry), n)
		}
	}

	entries := cb.Entries()
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	for i, entry := range entries {
		if !bytes.Equal(entry, data[i]) {
			t.Errorf("expected entry %d to be %q, got %q", i, string(data[i]), string(entry))
		}
	}
}
