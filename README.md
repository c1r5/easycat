# easycat

`easycat` is a fast terminal UI for Android `logcat` debugging through ADB.

It helps you:

- list connected Android devices
- list installed packages for the selected device
- stream `logcat` in real time
- filter apps by typing in the Apps panel
- filter logs by text, level, and selected app PID
- navigate filters from the keyboard
- read wrapped log lines inside the terminal viewport
- pause, clear, scroll, and follow logs without leaving the terminal

## Installation

### Requirements

- Go 1.25 or newer
- ADB available in your `PATH`
- An Android device or emulator with USB debugging enabled

Check ADB:

```sh
adb devices -l
```

Install with Go:

```sh
go install github.com/c1r5/easycat@latest
```

Make sure `$(go env GOPATH)/bin` is available in your `PATH`.

Install dependencies:

```sh
make deps
```

Build the binary:

```sh
make build
```

The binary will be created at:

```sh
bin/easycat
```

## Running

Run from source:

```sh
go run .
```

Run the built binary:

```sh
./bin/easycat
```

Development mode with hot reload:

```sh
make dev
```

`make dev` uses `cargo watch`, so install it first if needed:

```sh
cargo install cargo-watch
```

## Shortcuts

| Key | Action |
| --- | --- |
| `tab` / `shift+tab` | Change focused panel |
| `enter` | Select the focused device/app or edit/toggle the selected filter |
| `/` | Focus the text log filter directly |
| `esc` | Leave text filter editing or clear the app search |
| `up` / `down` | Move selection, scroll logs, or choose a filter field |
| `j` / `k` | Choose the next/previous filter field while Filters is focused |
| `left` / `right` / `space` | Toggle the selected filter option while Filters is focused |
| typing while Apps is focused | Filter apps incrementally |
| `backspace` while Apps is focused | Remove one character from the app search |
| `pgup` / `pgdown` | Scroll logcat |
| `g` / `G` | Go to top / bottom while Logcat is focused |
| `l` | Cycle log level filter: all, `E`, `W`, `I`, `D` |
| `o` | Toggle selected app PID-only filtering |
| `r` | Refresh devices |
| `c` | Clear logs |
| `p` | Pause / resume rendering |
| `q` | Quit |

## Demo

![easycat demo](./docs/demo.gif)
