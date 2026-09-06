package gamesscreenshotmanager

import (
	"strings"
	"testing"
)

// The doctor subcommand must be recognised wherever it sits. A command line
// that reports must never be mistaken for one that copies files.
func TestParseArgs_FindsTheDoctorSubcommand(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"on its own", []string{"doctor"}},
		{"before the flags", []string{"doctor", "-config", "./config.toml"}},
		{"after the flags", []string{"-config", "./config.toml", "doctor"}},
		{"after a boolean flag", []string{"-dry-run", "doctor"}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			parsed, err := parseArgs(testCase.args)
			if err != nil {
				t.Fatalf("parseArgs(%q) returned an error: %v", testCase.args, err)
			}

			if !parsed.doctor {
				t.Errorf("parseArgs(%q) did not find the doctor subcommand", testCase.args)
			}
		})
	}
}

// A run is the default, so nothing that omits the subcommand may ask for the
// doctor.
func TestParseArgs_DefaultsToARun(t *testing.T) {
	parsed, err := parseArgs([]string{"-config", "./config.toml", "-dry-run"})
	if err != nil {
		t.Fatalf("parseArgs() returned an error: %v", err)
	}

	if parsed.doctor {
		t.Error("parseArgs() asked for the doctor without the subcommand")
	}

	if parsed.configPath != "./config.toml" {
		t.Errorf("configPath = %q, want %q", parsed.configPath, "./config.toml")
	}

	if !parsed.dryRun {
		t.Error("dryRun = false, want true")
	}
}

// A word the parser does not know is refused. Ignoring it would copy files
// when the user asked for something else.
func TestParseArgs_RefusesAnUnknownArgument(t *testing.T) {
	for _, args := range [][]string{
		{"doktor"},
		{"-config", "./config.toml", "doktor"},
		{"doctor", "extra"},
	} {
		_, err := parseArgs(args)
		if err == nil {
			t.Errorf("parseArgs(%q) returned no error, want the argument refused", args)
			continue
		}

		if !strings.Contains(err.Error(), "unknown argument") {
			t.Errorf("parseArgs(%q) error = %v, want it to name the unknown argument", args, err)
		}
	}
}

// "doctor" as a flag value is a value. Taking it as the subcommand would read
// a config path as a command.
func TestParseArgs_KeepsDoctorAsAFlagValue(t *testing.T) {
	parsed, err := parseArgs([]string{"-config", "doctor"})
	if err != nil {
		t.Fatalf("parseArgs() returned an error: %v", err)
	}

	if parsed.doctor {
		t.Error("parseArgs() took a flag value as the subcommand")
	}

	if parsed.configPath != "doctor" {
		t.Errorf("configPath = %q, want %q", parsed.configPath, "doctor")
	}
}

// An unknown log level names no level in levelMap, so it must be refused
// before the logger is built.
func TestParseArgs_RefusesAnUnknownLogLevel(t *testing.T) {
	_, err := parseArgs([]string{"-log", "chatty"})
	if err == nil {
		t.Fatal("parseArgs() returned no error, want the log level refused")
	}

	if !strings.Contains(err.Error(), "invalid log level") {
		t.Errorf("parseArgs() error = %v, want it to report the log level", err)
	}
}

// The defaults are what a bare command line asks for.
func TestParseArgs_Defaults(t *testing.T) {
	parsed, err := parseArgs(nil)
	if err != nil {
		t.Fatalf("parseArgs() returned an error: %v", err)
	}

	want := cliArgs{logLevel: "info"}
	if parsed != want {
		t.Errorf("parseArgs(nil) = %+v, want %+v", parsed, want)
	}
}
