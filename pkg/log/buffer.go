package log

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

// ErrBufferFull indicates that the buffer has reached its maximum capacity.
var ErrBufferFull = errors.New("buffer is full")

// CircularBuffer is a thread-safe circular buffer that implements [io.Writer].
// It stores a fixed number of recent entries, automatically overwriting the
// oldest entries when the buffer is full.
type CircularBuffer struct {
	entries  [][]byte
	size     int
	capacity int
	head     int
	mu       sync.RWMutex
	full     bool
}

// NewCircularBuffer creates a new circular buffer with the specified capacity.
// The capacity determines the maximum number of entries that can be stored.
func NewCircularBuffer(capacity int) *CircularBuffer {
	if capacity <= 0 {
		capacity = 100 // Default capacity.
	}

	return &CircularBuffer{
		entries:  make([][]byte, capacity),
		capacity: capacity,
	}
}

// Write implements [io.Writer]. It stores the provided data as a new entry
// in the circular buffer. If the buffer is full, it overwrites the oldest entry.
// The data is copied to prevent external modifications.
func (cb *CircularBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Copy the data to prevent external modifications.
	entry := make([]byte, len(p))
	copy(entry, p)

	// Store the entry.
	cb.entries[cb.head] = entry
	cb.head = (cb.head + 1) % cb.capacity

	// Update size and full status.
	if !cb.full {
		cb.size++
		if cb.size == cb.capacity {
			cb.full = true
		}
	}

	return len(p), nil
}

// Entries returns a copy of all current entries in chronological order
// (oldest first). The returned slice is safe to modify.
func (cb *CircularBuffer) Entries() [][]byte {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.size == 0 {
		return nil
	}

	result := make([][]byte, 0, cb.size)

	if cb.full {
		// Read from head to end, then from start to head.
		for i := cb.head; i < cb.capacity; i++ {
			if cb.entries[i] != nil {
				entry := make([]byte, len(cb.entries[i]))
				copy(entry, cb.entries[i])

				result = append(result, entry)
			}
		}

		for i := range cb.head {
			if cb.entries[i] != nil {
				entry := make([]byte, len(cb.entries[i]))
				copy(entry, cb.entries[i])

				result = append(result, entry)
			}
		}
	} else {
		// Read from start to size.
		for i := range cb.size {
			if cb.entries[i] != nil {
				entry := make([]byte, len(cb.entries[i]))
				copy(entry, cb.entries[i])

				result = append(result, entry)
			}
		}
	}

	return result
}

// Size returns the current number of entries in the buffer.
func (cb *CircularBuffer) Size() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return cb.size
}

// Capacity returns the maximum number of entries the buffer can hold.
func (cb *CircularBuffer) Capacity() int {
	return cb.capacity
}

// IsFull returns true if the buffer has reached its maximum capacity.
func (cb *CircularBuffer) IsFull() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return cb.full
}

// Clear removes all entries from the buffer.
func (cb *CircularBuffer) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.size = 0
	cb.head = 0

	cb.full = false
	for i := range cb.entries {
		cb.entries[i] = nil
	}
}

// WriteTo writes all current entries to the provided writer in chronological
// order. It implements [io.WriterTo] for efficient bulk transfers.
func (cb *CircularBuffer) WriteTo(w io.Writer) (int64, error) {
	entries := cb.Entries()

	var total int64

	for _, entry := range entries {
		written, writeErr := w.Write(entry)
		total += int64(written)

		if writeErr != nil {
			return total, fmt.Errorf("writing entry: %w", writeErr)
		}
	}

	return total, nil
}
