---
id: TQ-0006
title: The doctor subcommand is only recognised as the first argument
status: done
priority: high
labels:
  - bug
  - component/cli
created: 2026-09-06T11:39:59+02:00
updated: 2026-09-06T11:41:23+02:00
---

## Problem

`RunCLI` detects the subcommand with `args[0] == "doctor"` only
(`cli.go:28`). A user who writes the flags first gets a real run:

    games-screenshot-manager -config ./config.toml doctor

`flag.Parse` stops at the first non-flag argument, so `doctor` is left as an
ignored positional. The tool then copies files instead of reporting on the
setup.

## Fix

Detect the subcommand wherever it sits, or refuse an unknown positional
argument. A command that reports must never be mistaken for a command that
writes.

---

## Notes

- 2026-09-06T11:41:23+02:00 — Fixed. The parsing moved out of RunCLI into parseArgs, which takes the argument list and returns a cliArgs record, so it is testable.

  A leading 'doctor' is still taken off the list before the flags are parsed,
  because flag.Parse stops at the first argument that is not a flag. A 'doctor'
  that follows the flags is now read out of flags.Args(). Any other leftover is
  refused with 'unknown argument', so a typo reports rather than copying files.

  A flag value of 'doctor', as in '-config doctor', stays a value. The leftover
  scan only sees arguments the flag package did not consume.

  parseArgs uses its own FlagSet rather than flag.CommandLine, so a test can
  call it more than once. The log level check moved into it as well.

  Six tests added in cli_test.go, which is a new file.
