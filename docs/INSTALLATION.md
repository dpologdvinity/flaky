# APT and curl distribution

## Design

GoReleaser produces Debian packages for Linux amd64 and arm64. Packages install
the executable at `/usr/bin/flaky` and include its MIT license. Package metadata
identifies the project, version, architecture, and maintainer.

The existing `dpologdvinity/packages` repository hosts the packages alongside
yonder. Its existing signing key and flat APT repository remain unchanged.
A separate updater fetches Flaky's latest release, verifies checksums and Debian
metadata, regenerates the shared APT index, and signs it with the existing key.
Both package workflows use the same concurrency group to serialize publication.
Updates run hourly and can be dispatched manually. Existing packages are kept.

`install.sh` supports Linux and macOS on amd64 and arm64. It queries the latest
published release, downloads the matching archive and checksums over HTTPS,
verifies SHA-256, extracts only the executable, and replaces the destination
atomically. Its default destination is `$HOME/.local/bin`; `FLAKY_INSTALL_DIR`
selects another directory. Failure leaves an installed executable unchanged.
It requires curl, tar, and either sha256sum or shasum. Temporary files are removed
on exit and interruption. The downloaded executable is not run during install.

The existing shared repository avoids another signing key, service, or token.
The curl installer also supports systems without APT. Publishing these changes
uses a new patch release; the existing v0.1.0 tag and artifacts stay immutable.

## Global constraints

- Preserve the shared repository's existing Origin, Label, public key, and URLs.
- Do not delete another project's packages or change its package formats.
- Verify downloads before installation or repository publication.
- Use the existing package signing secret only inside GitHub Actions.
- Keep installation tests isolated in temporary directories or containers.
- No changes to the CLI's runtime configuration or Go dependencies.

## Implementation plan

- [x] Add Debian packaging to `.goreleaser.yaml`; make CI generate and install
  snapshot packages, checking amd64 and arm64 metadata.
- [x] Add `install.sh` with fake-release tests for supported platforms, checksum
  failures, failed downloads, and preservation of an existing installation.
- [x] Extend `dpologdvinity/packages` with an independent Flaky updater, shared
  APT signing, and serialized publication; verify bad downloads cannot publish.
- [x] Document APT key setup, repository setup, curl installation, and supported
  architectures. Update the installation badges and release specification.
- [x] Merge verified changes and publish v0.1.1 with Debian packages; run the
  repository updater and verify its signed public index.
- [x] Install through live APT and curl in disposable Debian/Ubuntu containers,
  run `flaky --version` and `flaky --help`, and verify published arm64 metadata.

## Acceptance

The documented repository setup followed by `apt install flaky` succeeds with
normal signature verification. The documented curl command installs a verified
binary without root. CI exercises package generation and installer failures.

## Verification (2026-10-25)

- Flaky PR #3 passed lint, race tests, snapshot packaging, and native APT install.
- Packages PR #1 passed signed-index fixture tests, including checksum and
  architecture rejection, preservation of yonder packages, and idempotence.
- Release v0.1.1 published both Debian architectures and updated Homebrew.
- The APT updater and explicit Pages deployment succeeded. The public signed
  index advertises Flaky 0.1.1 for amd64 and arm64.
- Fresh Ubuntu 24.04 and Debian stable containers installed Flaky 0.1.1 through
  the live APT repository using normal signature verification. A non-root user
  in each container also installed it through the documented curl command.
  Both methods passed `flaky --version` and `flaky --help`.
- Published arm64 package checksum, Debian metadata, and AArch64 ELF format were
  verified. Native macOS/arm64 execution was not available; installer fixtures
  cover all four supported platform combinations.
