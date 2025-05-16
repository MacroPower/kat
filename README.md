# go_template

A template for my Go projects.

## Features

- Configured linter (using [golangci][golangci]).
- Development environment (using [devbox][devbox]).
- Build and release automation (using [goreleaser][goreleaser]).
- Command-line and environment variable parser (using [kong][kong]).
- Leveled logger with logfmt and json support (using slog).
- [Taskfile][task] with help, format, lint, test, and build targets.
- GitHub Actions for all of the above.

[golangci]: https://golangci-lint.run
[devbox]: https://www.jetify.com/devbox
[goreleaser]: https://goreleaser.com
[kong]: https://github.com/alecthomas/kong
[task]: https://taskfile.dev/
