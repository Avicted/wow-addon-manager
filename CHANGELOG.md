# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [1.4.0] - 2026-11-15

### Features

- **gui:** list addons with updates and errors first ([d52320f](https://github.com/Avicted/wow-addon-manager/commit/d52320fe859665a550207861aa7411863c38e36e))
- **gui:** gold label on primary buttons ([c37499e](https://github.com/Avicted/wow-addon-manager/commit/c37499e31f106228a306d3c8b3e6d1b72d625c3a))
- **gui:** show remove only on the hovered row ([18a5d7b](https://github.com/Avicted/wow-addon-manager/commit/18a5d7b012d70eb9c9b0d4954361260caef44aa0))
- **gui:** filter long addon lists by name ([ace0ff3](https://github.com/Avicted/wow-addon-manager/commit/ace0ff31f094dee38aa595602630872dac745482))

### Fixes

- **gui:** match table header radius to its frame ([1c5681a](https://github.com/Avicted/wow-addon-manager/commit/1c5681a132acb6ec6902ab72d7d35fc1c45a15c1))
- **gui:** draw button labels in their kind's colour ([7312e49](https://github.com/Avicted/wow-addon-manager/commit/7312e49b78065a5357d0323a51c7aa2bc53febf6))
- **gui:** keep row remove reachable by keyboard ([09a0250](https://github.com/Avicted/wow-addon-manager/commit/09a0250a7c3b220e1c39b223963aeabe1e53bd05))
- **gui:** style unidentified folders like the other settings headings ([c619292](https://github.com/Avicted/wow-addon-manager/commit/c619292e1389305c1e458d0e8b84510156ddd5fe))
- **gui:** hide the strip icon when there is no message ([d617ce3](https://github.com/Avicted/wow-addon-manager/commit/d617ce3a97364a90b131a7761617759b4163f3fc))
- **make:** decide docker -t in the recipe, so make check works in a terminal ([c4fcd34](https://github.com/Avicted/wow-addon-manager/commit/c4fcd34971c4f35d2466669bbb08657579ee5296))

### Performance

- **gui:** build release with 4 codegen units ([af6b04c](https://github.com/Avicted/wow-addon-manager/commit/af6b04c6fe92cb041e59fafd1d94d8c82201b22e))
- **gui:** emit less debuginfo in dev builds ([fa5e515](https://github.com/Avicted/wow-addon-manager/commit/fa5e5156cc1805d3f82524a64f17fa8733e318ac))
- **gui:** build the Linux and Windows binaries in parallel ([714a2cc](https://github.com/Avicted/wow-addon-manager/commit/714a2cc13744332f42898e077fdc729618ba0c29))
- **make:** run the make check steps in parallel ([38a9e2e](https://github.com/Avicted/wow-addon-manager/commit/38a9e2e3ebcb04e4ea9355ecf8821e1eacc11ba0))

### Documentation

- **gui:** refresh the screenshot ([bd9ad57](https://github.com/Avicted/wow-addon-manager/commit/bd9ad57a3583b1743fb2f0100ea5f2e869a23bca))
- **gui:** refresh the screenshot ([6c73f69](https://github.com/Avicted/wow-addon-manager/commit/6c73f69e770dd89779753cfc4a57501f8e596734))
- **gui:** refresh the screenshot ([bfc1cc6](https://github.com/Avicted/wow-addon-manager/commit/bfc1cc6c18c16a550ea4d2a3612aa1e97363f9c7))
- **readme:** add CI status and test coverage badges ([7d48f3f](https://github.com/Avicted/wow-addon-manager/commit/7d48f3fe3a4aa8a390095fb9b78d5b294d52f8e7))

### Other changes

- **gui:** refresh as a secondary button ([f156d6d](https://github.com/Avicted/wow-addon-manager/commit/f156d6df825de8660c6566db582962f48bf5e713))
- **gui:** drop the unused success button ([c0fc193](https://github.com/Avicted/wow-addon-manager/commit/c0fc1938e3ffc032d46ae1fc2c752be80fae1371))
- **gui:** stronger row stripes ([ef0b525](https://github.com/Avicted/wow-addon-manager/commit/ef0b52521b0cef8cf27ba307335906b98702ec81))
- **gui:** drop the update count from the status strip ([5013846](https://github.com/Avicted/wow-addon-manager/commit/50138468d7600fed921dbaabc9978f2f70c5d914))
- **gui:** drop the errors section, failed rows already sort first ([3136ec2](https://github.com/Avicted/wow-addon-manager/commit/3136ec22741f692eda744ff828e72434d27899ab))
- publish the Go test coverage as a badge ([cac160d](https://github.com/Avicted/wow-addon-manager/commit/cac160d6b38ddbad39c2f9671f133d262cc0549c))

## [1.3.0] - 2026-11-15

### Features

- **gui:** bundle the Cinzel display font ([47dce82](https://github.com/Avicted/wow-addon-manager/commit/47dce826cf8e5bed2f467a4a56e87dd47d61793f))
- **gui:** warcraft colour palette ([d67291f](https://github.com/Avicted/wow-addon-manager/commit/d67291fc24171cfd36946ae47860604b623ed9c7))
- **gui:** gold frames and display headings ([076f8e7](https://github.com/Avicted/wow-addon-manager/commit/076f8e7d2cd41829aab698c20fe06d6801eec12a))
- **gui:** crimson and gold app icon ([63d580a](https://github.com/Avicted/wow-addon-manager/commit/63d580a2feca7e34d68770f837d5b80c900aa965))

### Documentation

- **gui:** describe the warcraft theme and refresh the screenshot ([63ce07c](https://github.com/Avicted/wow-addon-manager/commit/63ce07c9caebc2d605197c46872423ce63c8c6fa))

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
[1.4.0]: https://github.com/Avicted/wow-addon-manager/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/Avicted/wow-addon-manager/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/Avicted/wow-addon-manager/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/Avicted/wow-addon-manager/compare/v1.0.0...v1.1.0
