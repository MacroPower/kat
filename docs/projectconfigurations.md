# Project Configuration (katrc files)

Project configuration files allow repository owners to define custom rendering rules and profiles that are specific to their project. When `kat` is run, it searches for a project config file starting from the target path and walking up the directory tree.

## File Names

The following file names are recognized (in order of precedence):

1. `.katrc.yaml`
2. `katrc.yaml`

## Trust System

Because project configurations can define arbitrary rendering commands, `kat` implements a trust system to protect users from potentially malicious configurations.

When a project config file is found in an untrusted project:

1. **Interactive mode**: A prompt asks the user to trust or skip the project configuration
2. **Non-interactive mode**: The project configuration is skipped with a warning

Trusted projects are stored in the global configuration file at `~/.config/kat/config.yaml` under `projects.trust`.

### CLI Flags

You can control trust behavior without prompting using CLI flags:

| Flag         | Description                                                            |
| ------------ | ---------------------------------------------------------------------- |
| `--trust`    | Trust the project configuration without prompting (adds to trust list) |
| `--no-trust` | Skip the project configuration without prompting                       |

These flags are mutually exclusive.

### Global Configuration

To pre-trust projects, add them to your global configuration:

```yaml
apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
projects:
  trust:
    - path: /path/to/trusted/project
    - path: /another/trusted/project
```

## Configuration Schema

Project configurations use `kind: ProjectConfiguration` and can define rules and/or profiles.

```yaml
# yaml-language-server: $schema=https://jacobcolvin.com/kat/schemas/projectconfigurations.v1beta1.json
apiVersion: kat.jacobcolvin.com/v1beta1
kind: ProjectConfiguration
rules:
  - match: <expression>
    profile: <profile name>
profiles:
  <profile name>:
    command: <cmd>
    args: [<arg>]
```

## Merge Behavior

When a project configuration is loaded, it merges with the global configuration:

- **Profiles**: Project profiles override global profiles with the same key
- **Rules**: Project rules are prepended to global rules (i.e. they are evaluated first)

This allows projects to override specific profiles while falling back to global defaults for others.

## Example

A project that adds a custom profile for a specific tool:

```yaml
apiVersion: kat.jacobcolvin.com/v1beta1
kind: ProjectConfiguration

rules:
  - match: >-
      files.exists(f, pathExt(f) == ".jsonnet")
    profile: jsonnet

profiles:
  jsonnet:
    source: >-
      files.filter(f, pathExt(f) in [".jsonnet", ".libsonnet"])
    command: jsonnet
    args: ["-y", "main.jsonnet"]
```

## Security Considerations

- Project configurations can execute arbitrary commands defined in profiles
- Always review project config files before trusting a project
- Use `--no-trust` in CI/CD pipelines or automated environments where you want to use only the global configuration
- The trust prompt displays the full path to both the configuration file and project directory
