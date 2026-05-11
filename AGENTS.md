# Repository Guidelines

## Project Structure & Module Organization

`easycat` is a Go terminal UI for Android `logcat` debugging. The entry point is [main.go](/home/dev/Dev/github.com/c1r5/easycat/main.go), which delegates to `internal/app`. Core packages live under `internal/`: `adb` wraps ADB commands and streaming, `domain` contains devices, packages, log parsing, filters, and buffers, and `tui` contains the Bubble Tea model, commands, views, and delegates. Tests sit beside implementation files as `*_test.go`. User documentation and demo assets live in `README.md` and `docs/`.

## Build, Test, and Development Commands

- `make deps`: runs `go mod tidy` and downloads modules.
- `make build`: builds `bin/easycat` from the repository root command.
- `make test`: runs `go test ./...` across all packages.
- `go run .`: runs the TUI from source.
- `make dev`: runs `go run .` under `cargo watch`; install it with `cargo install cargo-watch` first.
- `make clean`: removes the generated `bin/` directory.

ADB must be available in `PATH`; verify with `adb devices -l` before testing device-dependent behavior.

## Coding Style & Naming Conventions

Follow standard Go formatting: run `gofmt` on changed `.go` files and keep imports organized by `go fmt`/`goimports` conventions. Use tabs for Go indentation. Keep package names short and lowercase (`adb`, `tui`, `domain`). Export only API needed across packages; prefer unexported helpers inside `internal/tui` unless another package needs them. Test names should use descriptive `TestBehaviorCondition` style, as in `TestLogBatchIgnoresStaleStream`.

## Testing Guidelines

Use the standard `testing` package. Keep tests package-local and focused on parsing, filtering, model transitions, rendering, and ADB command behavior. Add or update tests when changing log parsing, filter semantics, stream lifecycle handling, or viewport rendering. Run `make test` before submitting changes. There is no fixed coverage threshold, but behavior changes should include regression coverage.

## Commit & Pull Request Guidelines

Recent history uses concise conventional-style commits such as `feat(tui): add app filtering and filter navigation`, `perf(tui): batch log lines and sanitize output`, and `refactor: move main package to project root`. Prefer `type(scope): summary` when a scope is clear.

Pull requests should include a short problem/solution summary, tests run, and any manual ADB/device checks. Include screenshots or terminal recordings for visible TUI changes, especially layout, focus, filtering, or wrapping updates. Link related issues when available and call out follow-up work explicitly.

## Agent-Specific Instructions

Do not assume a connected Android device is available in automated checks. Avoid overwriting unrelated work in the tree, including generated binaries and documentation drafts.

## MAP

### Root

- `main.go`: application entrypoint. Creates a root `context.Context` and calls `internal/app.Run`.
- `Makefile`: common development commands.
  - `make deps`: tidy and download Go modules.
  - `make build`: builds `bin/easycat`.
  - `make dev`: runs hot reload through `cargo watch`.
  - `make test`: runs `go test ./...`.
  - `make clean`: removes `bin/`.
- `README.md`: user-facing installation, running, shortcuts, and demo notes.
- `go.mod` / `go.sum`: Go module and dependency lock files.
- `.gitignore`: ignored local/build artifacts.

### `internal/app`

- `internal/app/app.go`: wires the application together.
  - Creates the Bubble Tea program.
  - Uses `tea.WithAltScreen()`.
  - Instantiates the TUI with `tui.New(ctx, adb.NewClient())`.

### `internal/adb`

- `internal/adb/adb.go`: thin wrapper around `exec.Command` for ADB.
  - `ListDevices`: runs `adb devices -l`.
  - `ListPackages`: runs `adb -s SERIAL shell pm list packages`.
  - `PIDOf`: runs `adb -s SERIAL shell pidof PACKAGE`.
  - `StartLogcat`: runs realtime logcat with `adb -s SERIAL logcat -v threadtime -T 0`.
  - `Stream`: owns logcat channels and cancellation.
- `internal/adb/adb_test.go`: tests logcat argument construction.

### `internal/domain`

- `internal/domain/domain.go`: core structs and state helpers.
  - `Device`, `Package`, `LogLine`, `Filters`.
  - `LogBuffer`: circular log buffer with a fixed limit.
  - Filter matching logic for text, level, and PID.
- `internal/domain/parse.go`: parsing helpers.
  - Parses ADB device lines.
  - Parses package lines.
  - Parses common logcat formats.
- `internal/domain/domain_test.go`: parser, filter, and buffer tests.

### `internal/tui`

- `internal/tui/model.go`: main Bubble Tea model and update loop.
  - Manages focus between Filters, Devices, Apps, and Logcat.
  - Handles keyboard shortcuts.
  - Loads devices/apps asynchronously.
  - Starts/stops logcat streams.
  - Applies app search and log filters.
  - Renders wrapped log lines into the viewport.
- `internal/tui/cmds.go`: Bubble Tea async commands and messages.
  - Device loading.
  - App loading.
  - Logcat stream startup.
  - Stream line/done waiters.
- `internal/tui/view.go`: layout and visual rendering.
  - Left/right panel sizing.
  - Footer rendering.
  - Filter rows and toggle state styling.
  - Panel sizing helpers to keep the UI inside the terminal.
- `internal/tui/list_delegate.go`: compact one-line list delegate.
  - Used by Devices and Apps.
  - Shows a `>` cursor for selected items.
- `internal/tui/constants.go`: shared TUI constants.
- `internal/tui/model_test.go`: model behavior tests.
  - Logcat line wrapping.
  - Focusing Apps after selecting a valid device.
- `internal/tui/view_test.go`: render-size tests.
  - Ensures the TUI fits common terminal sizes.
  - Ensures footer shortcuts are not clipped.

### `docs`

- `docs/ui-list-containers.md`: project convention for compact lists inside containers.
- `docs/demo.gif`: README demo asset.

### Generated Artifacts

- `bin/easycat`: local build output from `make build`; do not edit manually.