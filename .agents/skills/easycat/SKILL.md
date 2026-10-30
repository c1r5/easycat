---
name: easycat
description: >
  Read Android logcat through the easycat commands. Use when the user mentions
  logcat, ADB, an Android device, or runs /easycat.
user-invocable: true
---

# easycat

Use the `easycat` binary on `PATH`. Run the commands below. Leave the terminal UI alone.

`adb` must be on `PATH`. The phone or emulator must show up in `adb devices -l`.

```text
easycat devices
easycat devices info --serial SERIAL
easycat apps --serial SERIAL [--query TEXT]
easycat apps info --serial SERIAL --package NAME
easycat logs --serial SERIAL --package NAME [--level E|W|I|D] [--text TEXT] [--pid-only]
```

1. Run `easycat devices`. Each line is `serial`, `state`, and `model`, separated by tabs. Continue with a row whose state is `device`.
2. Run `easycat devices info --serial SERIAL` when you need the raw `adb devices -l` line. It prints `serial:`, `state:`, `model:`, and `raw:`.
3. Run `easycat apps --serial SERIAL`. Add `--query TEXT` to keep package names that contain that text. Case does not matter.
4. Run `easycat apps info --serial SERIAL --package NAME`. The output has `package:` and `pid:`. An empty `pid:` means the app is installed and not running.
5. Run `easycat logs --serial SERIAL --package NAME` and read stdout until you stop the process. `--pid-only` defaults to on, so the lines are from the PID resolved when the command started. `--level` is `E`, `W`, `I`, or `D`. `--text` is a substring and ignores case.
6. After the app process restarts, run `easycat logs` again. That resolves the PID from scratch.
