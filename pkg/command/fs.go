package command

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/macropower/kat/pkg/rule"
)

const defaultMaxDepth = 10

// FilteredFS wraps an [os.Root] and filters the tree based on rule matches. It
// hides files and directories that do not match any rules. Note that it does
// not directly prevent access to any files in the tree. (However, [os.Root]
// will prevent leaving the initially provided directory tree.)
type FilteredFS struct {
	root     *os.Root
	rules    []*rule.Rule
	maxDepth uint // Maximum depth to traverse directories. 0 means no limit.
}

// NewFilteredFS creates a new FilteredFS with the given directory path and rules.
func NewFilteredFS(dirPath string, rules ...*rule.Rule) (*FilteredFS, error) {
	root, err := os.OpenRoot(dirPath)
	if err != nil {
		return nil, fmt.Errorf("open directory %q: %w", dirPath, err)
	}

	return &FilteredFS{
		root:     root,
		rules:    rules,
		maxDepth: defaultMaxDepth,
	}, nil
}

// Close closes the [FilteredFS]. After Close is called, methods on [FilteredFS] return errors.
func (f *FilteredFS) Close() error {
	return f.root.Close() //nolint:wrapcheck // Return the original error.
}

// Name returns the name of the directory presented to OpenRoot.
// It is safe to call Name after [FilteredFS.Close].
func (f *FilteredFS) Name() string {
	return f.root.Name()
}

// Open opens the named file in the root for reading. See [os.Open] for more details.
func (f *FilteredFS) Open(name string) (*os.File, error) {
	return f.root.Open(name) //nolint:wrapcheck // Return the original error.
}

// OpenFile opens the named file in the root. See [os.OpenFile] for more details.
// If perm contains bits other than the nine least-significant bits (0o777), OpenFile returns an error.
func (f *FilteredFS) OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	return f.root.OpenFile(name, flag, perm) //nolint:wrapcheck // Return the original error.
}

// Stat returns a [fs.FileInfo] describing the named file in the root.
// See [os.Stat] for more details.
func (f *FilteredFS) Stat(name string) (fs.FileInfo, error) {
	return f.root.Stat(name) //nolint:wrapcheck // Return the original error.
}

// ReadDir reads the named directory, returning any entries that match at least
// one [rule.Rule] (recursively), sorted by filename.
func (f *FilteredFS) ReadDir(name string) ([]os.DirEntry, error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // Ignore errors.

	// Read all entries from the directory.
	entries, err := file.ReadDir(-1)
	if err != nil {
		return nil, err //nolint:wrapcheck // Return the original error.
	}

	// Check which entries are allowed based on rules.
	allowed := f.filterEntries(name, entries, 0)

	return allowed, nil
}

// filterEntries filters directory entries based on rules, returning only those that match.
// It recursively checks subdirectories up to maxDepth.
func (f *FilteredFS) filterEntries(dirPath string, entries []os.DirEntry, depth uint) []os.DirEntry {
	var (
		files  []os.DirEntry
		dirs   []os.DirEntry
		result []os.DirEntry
	)

	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	// Check if any files in this directory match rules.
	for _, file := range files {
		for _, r := range f.rules {
			if r.MatchFiles(dirPath, []string{filepath.Join(dirPath, file.Name())}) {
				result = append(result, file)
				continue
			}
		}
	}

	for _, dir := range dirs {
		subPath := filepath.Join(dirPath, dir.Name())
		if f.hasAllowedContent(subPath, depth+1) {
			result = append(result, dir)
		}
	}

	return result
}

// hasAllowedContent checks if a directory contains any files that match rules,
// either directly or in subdirectories (up to maxDepth).
func (f *FilteredFS) hasAllowedContent(dirPath string, depth uint) bool {
	if f.maxDepth > 0 && depth > f.maxDepth {
		return false
	}

	file, err := f.Open(dirPath)
	if err != nil {
		return false
	}
	defer file.Close() //nolint:errcheck // Ignore errors.

	entries, err := file.ReadDir(-1)
	if err != nil {
		return false
	}

	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() && f.hasAllowedContent(filepath.Join(dirPath, entry.Name()), depth+1) {
			// If the subdirectory matches, this directory is also implicitly allowed.
			// So, we can exit early.
			return true
		}

		files = append(files, filepath.Join(dirPath, entry.Name()))
	}

	// Check if this directory matches any rules.
	for _, r := range f.rules {
		if r.MatchFiles(dirPath, files) {
			return true
		}
	}

	return false
}
