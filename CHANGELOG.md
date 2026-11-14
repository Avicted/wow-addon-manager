# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [1.0.0] - Unreleased

First stable release.

### Added

- Desktop app (`wam-gui`) for Linux and Windows: see installed addons and their updates,
  update one or all (with cancel), search and install, track unidentified folders, fix broken
  ones, and remove addons to the backups. The matching `wam` is built into the executable.
- Windows support for the command-line tool: file locking, cross-drive backups, WoW process
  detection and default paths (`%LOCALAPPDATA%\wam` for state and backups).
- `-json` flag: one JSON event per line on stdout (`log`, `progress`, `result`), and a `cancel`
  line on stdin that stops an update after the current addon. This is the interface the desktop
  app uses.
- Release binaries for Linux and Windows (`wam`, `wam-gui`) with SHA-256 checksums.
- GitHub rate-limit detection, with the time the limit resets.

### Changed

- `make dist` also exports `wam.exe` for Windows.
- The Go test stage also runs `go vet` for Windows.

### Known limitations

- Binaries are not code-signed: Windows SmartScreen asks for confirmation, and some antivirus
  tools may flag the app because it unpacks and runs the embedded `wam.exe`.
- The Windows build is tested under Wine, not on a real Windows machine.
- Updating addons while the game runs can fail on Windows because the game keeps files open;
  wam rolls back and warns first.
- No macOS build, and the desktop app does not update itself.
