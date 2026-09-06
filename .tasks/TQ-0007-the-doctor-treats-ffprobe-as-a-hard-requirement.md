---
id: TQ-0007
title: The doctor treats ffprobe as a hard requirement
status: done
priority: normal
labels:
  - bug
  - component/cli
created: 2026-09-06T11:39:59+02:00
updated: 2026-09-06T11:44:19+02:00
---

## Problem

`checkRequirements` marks any provider-wanted binary as `CheckFail`, and
`HasFailure` makes the CLI exit 1. The PlayStation 5 provider declares
ffprobe (`playstation5.go:165`), so a user without ffmpeg gets a red "fail"
and a non-zero exit for a setup that works.

A missing ffprobe only degrades. `GetVideoDuration` logs an error and returns
0, and `AdjustDatetimeByVideoDuration` then returns the timestamp unchanged.
The clip is still collected, with the end time rather than the start time.

Contrast exiftool. `exiftool.NewExiftool()` failing returns an error that
aborts the provider, so that one is a genuine failure.

## Fix

Add a severity or `Optional` field to `Requirement`. A requirement a provider
can work without becomes a warning, and it must not set the exit code.

---

## Notes

- 2026-09-06T11:44:19+02:00 — Fixed. Requirement gains two fields: Optional, and Degradation.

  Optional marks a program the provider works without. checkRequirements now
  reports a missing one as a warning, so it does not set the exit code.
  onlyTheGalleryWants is replaced by everyUseCanDoWithout, which treats the
  gallery and an optional provider requirement the same way: neither needs the
  program to run.

  Degradation says what the run loses. describeDegradation appends it to the
  remedy, so the report says why the miss is only a warning. A program with no
  declared degradation reads exactly as before.

  The PlayStation 5 ffprobe requirement is marked optional, with 'a clip keeps
  its end time rather than its start time'. exiftool stays required, because
  exiftool.NewExiftool failing returns an error that aborts the provider.

  TestRunDoctor_ReportsAMissingProgramAsAFailure asserted the old behaviour, so
  it is replaced by two tests: ReportsAMissingRequiredProgramAsAFailure, which
  uses exiftool through the PlayStation 4 provider, and
  ReportsAMissingOptionalProgramAsAWarning, which uses ffprobe and asserts
  HasFailure stays false.
