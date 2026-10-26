# Helwan Shooter

**Part of Helwan Linux**

> **Development Status: Work in Progress**
>
> Helwan Shooter is currently under active development. Features, gameplay, and other aspects of the game may change as development continues.

Helwan Shooter is a mouse-aim arcade target-shooting game written in Go using Ebiten 2.8.8. It supports Linux and Windows.

## Gameplay

- 25 stages across 5 worlds.
- Multiple waves in regular stages, with a boss fight at the end of each world.
- Bosses have a vulnerable core; hitting the outer armor does not damage them.
- Different target types, including moving, fading, armored, fast, bonus, hazard, and ammunition targets.
- Limited ammunition and a countdown timer.
- Three-star stage results, best scores, unlocked-stage progress, and saved statistics.

## Controls

| Action | Control |
|---|---|
| Aim | Move the mouse |
| Shoot | Left mouse button |
| Pause | `Escape` |

## Requirements

- Go 1.23 or newer.
- Internet access for the first build so Go can download Ebiten and its dependencies.

## Run from Source

Run the following commands from the project directory:

```sh
go mod tidy
go run ./cmd/helwan-shooter
```

## Build on Linux

```sh
go build -o HelwanShooter ./cmd/helwan-shooter
```

Run the game:

```sh
./HelwanShooter
```

## Build on Windows

Run this command in PowerShell or Command Prompt from the project directory:

```powershell
go build -o HelwanShooter.exe ./cmd/helwan-shooter
```

Then launch `HelwanShooter.exe`.

## Saved Progress

Game progress is stored in `progress.json` under the `HelwanShooter` directory within the operating system's user configuration directory.

## Development

Helwan Shooter is a work in progress. The project is still being developed, and improvements, fixes, and new features may be introduced in future updates.

**Part of Helwan Linux — Arch Power, Helwan Simplicity.**


## License

Helwan Shooter is distributed under the Helwan Source-Available License (HSAL) Version 1.0.

The source code is available for reading, learning, modification, and non-commercial redistribution, subject to the terms of the `LICENSE` file.

Commercial use requires prior written permission from the copyright holder.

Copyright (c) 2026 Saeed Badreldin.

See the [LICENSE](LICENSE) file for the complete license terms.

