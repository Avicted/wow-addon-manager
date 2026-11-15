# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [1.2.0] - 2026-11-15

### Features

- **status:** point out backups left by wam 1.0 ([c7d45f4](https://github.com/Avicted/wow-addon-manager/commit/c7d45f4403e818877cb5421f3fca598359723c12))

### Fixes

- **gui:** keep the data directory private ([670471d](https://github.com/Avicted/wow-addon-manager/commit/670471d61df349caf6b0ffc8aef967c4d7276093))
- **state:** create the data directory private ([469d026](https://github.com/Avicted/wow-addon-manager/commit/469d026ae018ae65400619820df7384e6d39d76f))
- **gui:** remove embedded wam binaries of older versions ([8c89c9c](https://github.com/Avicted/wow-addon-manager/commit/8c89c9c9b2d5c8381b486d640df8f140e4211f8b))
- **cli:** refuse "." and ".." as folder names ([b803e4e](https://github.com/Avicted/wow-addon-manager/commit/b803e4e1ed15eb57ad9ded4d585d3abf7fc94bc7))

## [1.1.0] - 2026-11-15

### Breaking changes

- **install:** keep backups as zip archives beside AddOns ([c5813bf](https://github.com/Avicted/wow-addon-manager/commit/c5813bfcc77566beb4acfe5ac1fbac78df77afe4))
- **release:** drop the version from release file names ([e027b4a](https://github.com/Avicted/wow-addon-manager/commit/e027b4a757cf107417a295599b3aef54733d7eba))

### Features

- **gui:** green Refresh button ([dfcd230](https://github.com/Avicted/wow-addon-manager/commit/dfcd2307148ce91d99698c0d4b20070b5be37681))
- **gui:** show the version in the window title ([83ff123](https://github.com/Avicted/wow-addon-manager/commit/83ff123b08e9da7dd36b91b4ef9c55248633d3f1))
- **gui:** About tab ([ecbfbbb](https://github.com/Avicted/wow-addon-manager/commit/ecbfbbb821210aa287b9c6171eae83b68a1ffbf6))
- **gui:** application icon ([5c6039b](https://github.com/Avicted/wow-addon-manager/commit/5c6039b1d46df19b430875729c2d8c4872010666))
- **gui:** describe zip backups beside AddOns ([3782d46](https://github.com/Avicted/wow-addon-manager/commit/3782d460eef86ab3583e280578015c4ffbcfc70a))

### Fixes

- **gui:** keep table rows at their own height ([e854f6c](https://github.com/Avicted/wow-addon-manager/commit/e854f6cd7b790a4f199c4d79aa7c69b0c6cb4da1))
- **gui:** keep the row highlight while a mouse button is held ([b930b2f](https://github.com/Avicted/wow-addon-manager/commit/b930b2f743a3d609cddb6632f75d6a404ab7e0eb))
- **gui:** centre the searching indicator ([c8e102e](https://github.com/Avicted/wow-addon-manager/commit/c8e102efc6e5dbea606ddc813b81f8b28e65cde8))
- **gui:** line the toolbar summary up with its buttons ([a744c31](https://github.com/Avicted/wow-addon-manager/commit/a744c31b47a5dc4b61ae10bf792a9d1182deecc1))
- **gui:** centre the empty state's buttons ([6ecf923](https://github.com/Avicted/wow-addon-manager/commit/6ecf9233b7814d32aa2417e797d3b7fa656015eb))

### Documentation

- drop the link to the removed GUI plan ([12774e0](https://github.com/Avicted/wow-addon-manager/commit/12774e066ff389fab2fadc45b99425b1a15fae70))
- release with make release ([1d5ec86](https://github.com/Avicted/wow-addon-manager/commit/1d5ec86228068136bd793becd97fb40c191386b2))

### Other changes

- make new-version and undo-version ([2083449](https://github.com/Avicted/wow-addon-manager/commit/20834493c9f760168c372e970355825ce53e15e9))
- use heredocs in Dockerfiles ([878cb8a](https://github.com/Avicted/wow-addon-manager/commit/878cb8afb24a2533b8d4e4fcb5061e95819ba1eb))
- **make:** colour the help categories and targets ([9336dc7](https://github.com/Avicted/wow-addon-manager/commit/9336dc7a6fdb4b4bb0fd3f5a80023d864fb09d36))
- run make check ([5d948e2](https://github.com/Avicted/wow-addon-manager/commit/5d948e2e6cb783fd0c782d8d572a94ef79b1f42c))
- prefer the self-hosted runner ([e96cbb2](https://github.com/Avicted/wow-addon-manager/commit/e96cbb22506715587c0e73d02439e7b4261a21ae))
- make gui-run and gui-dev, and document CI and releasing ([814fa2a](https://github.com/Avicted/wow-addon-manager/commit/814fa2aedd8d86a0b9ec1d07013838aa415c7450))
- **release:** publish with action-gh-release ([fb7a41e](https://github.com/Avicted/wow-addon-manager/commit/fb7a41ecc2947402922efa2b2968489a354ea413))
- **make:** make release bumps, commits, tags and pushes ([78bbc40](https://github.com/Avicted/wow-addon-manager/commit/78bbc40caf6dbcefc9bc145eb94f4ef75a23dc44))
- require version bumps to be make release commits ([d2eb6ec](https://github.com/Avicted/wow-addon-manager/commit/d2eb6ecfc0b9c215e75216159e6fd689f7643abf))

## [1.0.0] - 2026-11-15

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
- The Windows app draws with DirectX 12 or Vulkan (Windows 10 or later); if the window
  doesn't open, use the command-line tool.
- No macOS build, and the desktop app does not update itself.
[1.2.0]: https://github.com/Avicted/wow-addon-manager/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/Avicted/wow-addon-manager/compare/v1.0.0...v1.1.0
