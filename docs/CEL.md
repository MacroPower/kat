# CEL

This document describes the custom CEL functions available for file path operations in rules and profiles.

## Expression Types

### Rules - Boolean Expressions

Rules use CEL expressions that return a **boolean** value to determine if a rule should be applied:

- `true` means the rule matches and should be used
- `false` means the rule doesn't match

Use `files.exists()` to check if any file matches a condition:

```yaml
rules:
  - match: >-
      files.exists(f, pathBase(f) in ["Chart.yaml", "Chart.yml"])
    profile: helm
```

Rules are processed in order, so the first matching rule will be applied. If no rules match, and no explicit profile was selected via CLI args, kat will return an error.

### Profiles - List Expressions

Profiles use CEL expressions that return a **list of files** to determine which files to watch:

- Non-empty list means the profile will watch those specific files
- Empty list means no files should be watched

Use `files.filter()` to select specific files:

```yaml
profiles:
  helm:
    source: >-
      files.filter(f, pathExt(f) in [".yaml", ".yml", ".tpl"])
```

## Custom Functions

| Function   | Signature                 | Description                                                              |
| ---------- | ------------------------- | ------------------------------------------------------------------------ |
| `pathBase` | `(string) -> string`      | Returns the last element of the path (the filename)                      |
| `pathExt`  | `(string) -> string`      | Returns the file extension of the path, including the dot                |
| `pathDir`  | `(string) -> string`      | Returns all but the last element of the path (the directory)             |
| `yamlPath` | `(string, string) -> dyn` | Reads a YAML file and extracts value at path (returns null if not found) |

## Overview

We provide direct access to Go's standard `filepath` package functions. These can be combined with CEL's built-in `in` operator and other string functions for powerful file matching.

## Functions

### `pathBase(string)` - Get filename

Returns the last element of the path (the filename).

**For Rules (boolean):**

```yaml
# Check if any specific files exist:
match: >-
  files.exists(f, pathBase(f) in ["Chart.yaml", "Chart.yml"])

# Check if a single file exists:
match: >-
  files.exists(f, pathBase(f) == "Chart.yaml")
```

**For Profiles (list):**

```yaml
# Select specific files:
source: >-
  files.filter(f, pathBase(f) in ["Chart.yaml", "Chart.yml"])
```

### `pathExt(string)` - Get file extension

Returns the file extension of the path, including the dot.

**For Rules (boolean):**

```yaml
# Check if any YAML files exist:
match: >-
  files.exists(f, pathExt(f) in [".yaml", ".yml"])
```

**For Profiles (list):**

```yaml
# Select YAML and template files:
source: >-
  files.filter(f, pathExt(f) in [".yaml", ".yml", ".tpl"])
```

### `pathDir(string)` - Get directory

Returns all but the last element of the path (the directory).

**For Rules (boolean):**

```yaml
# Check if any files exist in templates directory:
match: >-
  files.exists(f, pathDir(f).contains("/templates"))
```

**For Profiles (list):**

```yaml
# Select files in templates directory:
source: >-
  files.filter(f, pathDir(f).contains("/templates"))
```

### `yamlPath(file, path)` - Read YAML content

Reads a YAML file and extracts a value using a JSONPath expression. Returns `null` if the file doesn't exist, can't be read, or the path doesn't exist.

**For Rules (boolean):**

```yaml
# Check if any Helm v2 charts exist:
match: >-
  files.exists(f, pathBase(f) == "Chart.yaml" && yamlPath(f, "$.apiVersion") == "v2")
```

**For Profiles (list):**

```yaml
# Select Helm v2 charts:
source: >-
  files.filter(f, pathBase(f) == "Chart.yaml" && yamlPath(f, "$.apiVersion") == "v2")
```

## Using Built-in CEL Functions

Combine filepath functions with CEL's built-in string and list operations:

**For Rules (boolean with exists):**

```yaml
# Using 'in' operator for membership:
match: >-
  files.exists(f, pathBase(f) in ["deployment.yaml", "service.yaml"])

# Using 'matches' for regex patterns:
match: >-
  files.exists(f, pathBase(f).matches(".*test.*"))

# Using 'contains' for substring matching:
match: >-
  files.exists(f, pathDir(f).contains("/templates/"))

# Using 'startsWith' and 'endsWith':
match: >-
  files.exists(f,
    pathBase(f).startsWith("Chart") &&
    pathExt(f) in [".yaml", ".yml"])
```

**For Profiles (list with filter):**

```yaml
# Using 'in' operator for membership:
source: >-
  files.filter(f, pathBase(f) in ["deployment.yaml", "service.yaml"])

# Excluding files with 'matches':
source: >-
  files.filter(f, pathExt(f) in [".yaml", ".yml"] && !pathBase(f).matches(".*test.*"))
```

## Complex Examples

### Helm Chart Detection

```yaml
rules:
  - match: >-
      files.exists(f,
        pathBase(f) in ["Chart.yaml", "Chart.yml"])
    profile: helm
```

### Kustomization Detection

```yaml
rules:
  - match: >-
      files.exists(f,
        pathBase(f) in ["kustomization.yaml", "kustomization.yml"])
    profile: kustomize
```

### Template Files (excluding tests)

```yaml
profiles:
  helm:
    source: >-
      files.filter(f,
        pathExt(f) in [".yaml", ".yml", ".tpl"] &&
        !pathBase(f).matches(".*test.*"))
```

### Directory-specific Matching

```yaml
rules:
  - match: >-
      files.exists(f,
        pathDir(f).contains("/manifests/") &&
        pathExt(f) in [".yaml", ".yml"])
    profile: yaml
```

### Content-based Matching with yamlPath

```yaml
rules:
  # Only match charts using `apiVersion: v2`:
  - match: >-
      files.exists(f,
        pathBase(f) == "Chart.yaml" &&
        yamlPath(f, "$.apiVersion") == "v2")
    profile: helm
```
