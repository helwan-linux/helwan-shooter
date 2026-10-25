# Helwan Shooter

Part of Helwan Linux.

Helwan Shooter is a mouse-aim arcade target-shooting game written in Go with Ebiten 2.8.8. It supports Linux and Windows.

## Gameplay

- 25 stages across 5 worlds.
- Multi-wave stages and a boss fight at the end of each world.
- Bosses have a vulnerable core; hitting the outer armor does not damage them.
- Moving, fading, armored, fast, bonus, hazard, and ammunition targets.
- Limited ammunition and a countdown timer.
- Three-star results, best scores, unlocked-stage progress, and saved statistics.
- Keyboard Escape pauses gameplay; left mouse button aims and shoots.

## Requirements

- Go 1.23 or newer.
- Internet access on the first build so Go can download Ebiten and its dependencies.

## Run

```sh
go mod tidy
go run ./cmd/helwan-shooter
```

## Build on Linux

```sh
go build -o HelwanShooter ./cmd/helwan-shooter
```

## Build on Windows (PowerShell or CMD)

```powershell
go build -o HelwanShooter.exe ./cmd/helwan-shooter
```

Progress is stored in the operating system's user configuration directory under `HelwanShooter/progress.json`.
