---
id: TQ-0014
title: 'CI: the PlayStation 4 test fails without exiftool'
status: done
priority: high
labels:
  - bug
  - component/ci
created: 2026-09-17T09:14:51+02:00
updated: 2026-09-17T09:15:57+02:00
---

## Problem

`TestPlaystation4ProviderSkipsAnUnknownExtension` fails on both CI runners.

```
playstation_test.go:143: GetScreenshots() returned an error: error initializing
exiftool: error when executing command: exec: "exiftool": executable file not
found in $PATH
```

The test calls `GetScreenshots`, which initializes exiftool before it reads the
folder. Neither runner has exiftool, so the test fails rather than skips.

It passes on a machine that holds exiftool, which is why it was never caught
locally.

## History

The test suite has been red since 2026-09-06, run 34043623430, the push that
tagged v1.6.0. Run 35193156386 fails the same way. It is one test, and the same
one, on both dates.

## Fix

Two parts, because the two problems are different:

1. The workflow installs exiftool, so the test runs rather than being skipped
   away. The provider is documented as needing it, so CI is the place that
   proves the provider works with it.
2. The test skips where exiftool is absent, so a contributor without it can
   still run `go test ./...`. `requireNormalizationSensitiveFS` in
   `gallery_test.go` is the pattern.

---

## Notes

- 2026-09-17T09:15:57+02:00 — Fixed, in the two parts the body describes.

  The workflow installs exiftool before it runs the suite: libimage-exiftool-perl
  on ubuntu, brew on macOS. RUNNER_OS is what picks between them.

  requireExiftool in playstation_test.go skips the test where the program is
  absent. It follows requireNormalizationSensitiveFS in gallery_test.go, which is
  how this repository already handles a test its host cannot run.

  Verified both ways on Linux:
  - exiftool present: the test runs and passes.
  - PATH without /usr/bin/vendor_perl: the test reports SKIP, and the whole suite
    is ok rather than FAIL. That is the exact state both runners were in.

  The failure was never in the provider. It was a test that needed a program the
  runner did not hold, and it failed rather than skipping. The provider itself is
  unchanged.

  The commit also carries a one-line test cleanup: os.Chmod in a Diablo IV test
  cleanup was unchecked, which errcheck flagged.

  gofmt, go vet and go test are clean.
