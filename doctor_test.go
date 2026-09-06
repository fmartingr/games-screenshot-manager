package gamesscreenshotmanager

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// stubLookPath replaces the search for a program. Every binary named is on the
// system, and every other one is missing.
func stubLookPath(t *testing.T, present ...string) {
	t.Helper()

	found := make(map[string]bool, len(present))
	for _, binary := range present {
		found[binary] = true
	}

	original := lookPath
	t.Cleanup(func() { lookPath = original })

	lookPath = func(binary string) (string, error) {
		if found[binary] {
			return "/usr/bin/" + binary, nil
		}

		return "", exec.ErrNotFound
	}
}

// newDoctorConfig returns a config with every provider off and an output path
// that exists.
func newDoctorConfig(t *testing.T) *Config {
	t.Helper()

	// The Steam client builds its cache under this directory.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	config := &Config{}
	config.Defaults()
	config.OutputPath = t.TempDir()

	return config
}

// findCheck returns the first result of one section that carries a name.
func findCheck(results []CheckResult, scope, name string) (CheckResult, bool) {
	for _, result := range results {
		if result.Scope == scope && result.Name == name {
			return result, true
		}
	}

	return CheckResult{}, false
}

// countChecks counts the results of one section that carry a name.
func countChecks(results []CheckResult, scope, name string) int {
	count := 0

	for _, result := range results {
		if result.Scope == scope && result.Name == name {
			count++
		}
	}

	return count
}

// A program a provider cannot run without is a failure, and it sets the exit
// code. exiftool is one: initialising it returns an error that aborts the
// provider.
func TestRunDoctor_ReportsAMissingRequiredProgramAsAFailure(t *testing.T) {
	stubLookPath(t)

	config := newDoctorConfig(t)
	config.Providers.PlayStation4.Enabled = Ptr(true)
	config.Providers.PlayStation4.Path = Ptr(t.TempDir())

	results := RunDoctor(config, "config.toml")

	result, found := findCheck(results, scopeDependencies, "exiftool")
	if !found {
		t.Fatal("the report holds no exiftool check")
	}

	if result.Status != CheckFail {
		t.Errorf("exiftool status = %q, want %q", result.Status, CheckFail)
	}

	if !strings.Contains(result.Remedy, "exiftool") {
		t.Errorf("exiftool remedy = %q, want it to name the exiftool package", result.Remedy)
	}

	if !HasFailure(results) {
		t.Error("HasFailure() = false, want true")
	}
}

// A program a provider works without is a warning, and it must not set the
// exit code. ffprobe is one: without it a clip keeps its end time, and the
// clip is still collected.
func TestRunDoctor_ReportsAMissingOptionalProgramAsAWarning(t *testing.T) {
	stubLookPath(t)

	config := newDoctorConfig(t)
	config.Providers.PlayStation5.Enabled = Ptr(true)
	config.Providers.PlayStation5.Path = Ptr(t.TempDir())

	results := RunDoctor(config, "config.toml")

	result, found := findCheck(results, scopeDependencies, "ffprobe")
	if !found {
		t.Fatal("the report holds no ffprobe check")
	}

	if result.Status != CheckWarn {
		t.Errorf("ffprobe status = %q, want %q", result.Status, CheckWarn)
	}

	if !strings.Contains(result.Remedy, "ffmpeg") {
		t.Errorf("ffprobe remedy = %q, want it to name the ffmpeg package", result.Remedy)
	}

	if !strings.Contains(result.Remedy, "end time") {
		t.Errorf("ffprobe remedy = %q, want it to say what the run loses", result.Remedy)
	}

	if HasFailure(results) {
		t.Error("HasFailure() = true, want false: the run works without ffprobe")
	}
}

// The doctor checks what the config turns on, and nothing else.
func TestRunDoctor_IgnoresAProgramNoEnabledProviderWants(t *testing.T) {
	stubLookPath(t)

	config := newDoctorConfig(t)
	config.Providers.Minecraft.Enabled = Ptr(true)

	results := RunDoctor(config, "config.toml")

	if _, found := findCheck(results, scopeDependencies, "ffprobe"); found {
		t.Error("the report holds an ffprobe check, but no enabled provider wants it")
	}

	if HasFailure(results) {
		t.Error("HasFailure() = true, want false")
	}
}

// Two providers that want one program produce one line, and the line names
// both of them.
func TestRunDoctor_GroupsTwoProvidersUnderOneProgram(t *testing.T) {
	stubLookPath(t, "exiftool")

	config := newDoctorConfig(t)
	config.Providers.GuildWars2.Enabled = Ptr(true)
	config.Providers.PlayStation4.Enabled = Ptr(true)
	config.Providers.PlayStation4.Path = Ptr(t.TempDir())

	results := RunDoctor(config, "config.toml")

	if count := countChecks(results, scopeDependencies, "exiftool"); count != 1 {
		t.Fatalf("the report holds %d exiftool checks, want 1", count)
	}

	result, _ := findCheck(results, scopeDependencies, "exiftool")
	if result.Status != CheckOK {
		t.Errorf("exiftool status = %q, want %q", result.Status, CheckOK)
	}

	for _, provider := range []string{"guild_wars_2", "playstation4"} {
		if !strings.Contains(result.Remedy, provider) {
			t.Errorf("exiftool remedy = %q, want it to name %s", result.Remedy, provider)
		}
	}
}

// The gallery is built without ffmpeg. A video gets no thumbnail, so the miss
// is a warning rather than a failure.
func TestRunDoctor_TreatsAGalleryProgramAsAWarning(t *testing.T) {
	stubLookPath(t)

	config := newDoctorConfig(t)
	config.Providers.Minecraft.Enabled = Ptr(true)
	config.Gallery.Create = true

	results := RunDoctor(config, "config.toml")

	result, found := findCheck(results, scopeDependencies, "ffmpeg")
	if !found {
		t.Fatal("the report holds no ffmpeg check")
	}

	if result.Status != CheckWarn {
		t.Errorf("ffmpeg status = %q, want %q", result.Status, CheckWarn)
	}

	if HasFailure(results) {
		t.Error("HasFailure() = true, want false")
	}
}

// A provider without an automatic path cannot run without one in the config.
func TestRunDoctor_FailsWhenAProviderHasNoPath(t *testing.T) {
	stubLookPath(t, "ffprobe")

	config := newDoctorConfig(t)
	config.Providers.PlayStation5.Enabled = Ptr(true)

	results := RunDoctor(config, "config.toml")

	result, found := findCheck(results, scopeProviders, "playstation5")
	if !found {
		t.Fatal("the report holds no playstation5 check")
	}

	if result.Status != CheckFail {
		t.Errorf("playstation5 status = %q, want %q", result.Status, CheckFail)
	}
}

// A path in the config that is not on disk is a failure.
func TestRunDoctor_FailsWhenAProviderPathIsMissing(t *testing.T) {
	stubLookPath(t, "exiftool")

	config := newDoctorConfig(t)
	config.Providers.PlayStation4.Enabled = Ptr(true)
	config.Providers.PlayStation4.Path = Ptr(t.TempDir() + "/missing")

	results := RunDoctor(config, "config.toml")

	result, _ := findCheck(results, scopeProviders, "playstation4")
	if result.Status != CheckFail {
		t.Errorf("playstation4 status = %q, want %q", result.Status, CheckFail)
	}
}

// The registry drops this provider on any host but Windows, and it says
// nothing about it today.
func TestRunDoctor_WarnsWhenXboxGameBarIsNotOnWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the run keeps the provider on Windows")
	}

	stubLookPath(t, "exiftool")

	config := newDoctorConfig(t)
	config.Providers.XboxGameBar.Enabled = Ptr(true)

	results := RunDoctor(config, "config.toml")

	if count := countChecks(results, scopeProviders, "xbox_game_bar"); count != 1 {
		t.Fatalf("the report holds %d xbox_game_bar checks, want 1", count)
	}

	result, _ := findCheck(results, scopeProviders, "xbox_game_bar")
	if result.Status != CheckWarn {
		t.Errorf("xbox_game_bar status = %q, want %q", result.Status, CheckWarn)
	}
}

// The online gallery needs a user ID as well as an API key.
func TestRunDoctor_WarnsWhenTheOnlineGalleryHasNoUserID(t *testing.T) {
	stubLookPath(t)

	config := newDoctorConfig(t)
	config.Providers.Steam.Enabled = Ptr(true)
	config.Providers.Steam.APIKey = "a-key"
	config.Providers.Steam.OnlineGallery = true

	results := RunDoctor(config, "config.toml")

	found := false
	for _, result := range results {
		if result.Scope == scopeProviders && result.Name == "steam" && result.Status == CheckWarn {
			found = true
		}
	}

	if !found {
		t.Error("the report holds no warning about the empty user_id")
	}
}

// A provider that refuses to start is a failure, and the message it gives is
// the report.
func TestRunDoctor_ReportsAProviderThatRefusesToStart(t *testing.T) {
	stubLookPath(t)

	config := newDoctorConfig(t)
	config.Providers.Steam.Enabled = Ptr(true)

	results := RunDoctor(config, "config.toml")

	result, found := findCheck(results, scopeProviders, "registry")
	if !found {
		t.Fatal("the report holds no registry check")
	}

	if result.Status != CheckFail {
		t.Errorf("registry status = %q, want %q", result.Status, CheckFail)
	}

	if !strings.Contains(result.Detail, "API key") {
		t.Errorf("registry detail = %q, want the message about the API key", result.Detail)
	}
}

// A run with no provider collects nothing, which is worth saying.
func TestRunDoctor_WarnsWhenNoProviderIsEnabled(t *testing.T) {
	stubLookPath(t)

	results := RunDoctor(newDoctorConfig(t), "config.toml")

	result, found := findCheck(results, scopeProviders, "providers")
	if !found {
		t.Fatal("the report holds no providers check")
	}

	if result.Status != CheckWarn {
		t.Errorf("providers status = %q, want %q", result.Status, CheckWarn)
	}
}

func TestHasFailure(t *testing.T) {
	passing := []CheckResult{{Status: CheckOK}, {Status: CheckWarn}}
	if HasFailure(passing) {
		t.Error("HasFailure() = true for a list of ok and warn, want false")
	}

	if !HasFailure(append(passing, CheckResult{Status: CheckFail})) {
		t.Error("HasFailure() = false for a list with a fail, want true")
	}

	if problems := CountProblems(passing); problems != 1 {
		t.Errorf("CountProblems() = %d, want 1", problems)
	}
}

// The report is the product of the command, so it holds the detail and the
// remedy of every check.
func TestPrintChecks(t *testing.T) {
	results := []CheckResult{
		{Scope: scopeConfig, Name: "config file", Status: CheckOK, Detail: "config.toml"},
		{Scope: scopeDependencies, Name: "ffprobe", Status: CheckFail, Detail: "not found in PATH", Remedy: "Install the ffmpeg package."},
	}

	report := &bytes.Buffer{}
	PrintChecks(report, results)

	for _, want := range []string{"Configuration", "Dependencies", "ffprobe", "not found in PATH", "Install the ffmpeg package.", "1 problem found."} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("the report does not hold %q:\n%s", want, report)
		}
	}

	// A file, a pipe and a test buffer take no colour.
	if strings.Contains(report.String(), "\x1b[") {
		t.Errorf("the report holds an ANSI code, and it was not written to a terminal:\n%q", report)
	}
}

// A terminal gets a bold heading, and a status in its own colour.
func TestPrintChecks_PaintsTheReport(t *testing.T) {
	results := []CheckResult{
		{Scope: scopeProviders, Name: "steam", Status: CheckOK, Detail: "enabled"},
		{Scope: scopeProviders, Name: "playstation5", Status: CheckWarn, Detail: "no path"},
		{Scope: scopeDependencies, Name: "ffprobe", Status: CheckFail, Detail: "not found in PATH"},
	}

	report := &bytes.Buffer{}
	printChecks(report, results, painter(true))

	for _, want := range []string{
		ansiBold + "Providers" + ansiReset,
		ansiBold + ansiGreen + "ok  " + ansiReset,
		ansiBold + ansiYellow + "warn" + ansiReset,
		ansiBold + ansiRed + "fail" + ansiReset,
		ansiBold + ansiRed + "2 problems found." + ansiReset,
	} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("the report does not hold %q:\n%q", want, report)
		}
	}
}

// NO_COLOR is a promise, so it holds even on a terminal.
func TestTakesColor_ObeysNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	if takesColor(os.Stdout) {
		t.Error("takesColor() = true with NO_COLOR set, want false")
	}
}

// A buffer is not a terminal.
func TestTakesColor_RefusesAWriterThatIsNotATerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")

	if takesColor(&bytes.Buffer{}) {
		t.Error("takesColor() = true for a buffer, want false")
	}
}

// The doctor reports on the system. It must leave the system as it found it,
// so it creates no directory, the Steam client's cache included.
func TestRunDoctor_CreatesNoDirectory(t *testing.T) {
	stubLookPath(t, "exiftool", "ffprobe", "ffmpeg")

	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	config := &Config{}
	config.Defaults()
	config.OutputPath = t.TempDir()
	config.Providers.Steam.Enabled = Ptr(true)
	// Without a key the provider refuses to start, and the Steam client that
	// builds the cache is never reached.
	config.Providers.Steam.APIKey = "test-key"
	config.Gallery.Create = true

	RunDoctor(config, "config.toml")

	entries, err := os.ReadDir(cacheHome)
	if err != nil {
		t.Fatalf("failed to read the cache home: %v", err)
	}

	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}

		t.Errorf("the doctor left %d entries under the cache home: %q", len(entries), names)
	}
}

// The provider reads the console through the libmtp tools, so it cannot run
// without them. A path in the config reads a folder instead, and then it needs
// none of them.
func TestRunDoctor_ReportsTheLibmtpToolsForNintendoSwitch2(t *testing.T) {
	stubLookPath(t)

	config := newDoctorConfig(t)
	config.Providers.NintendoSwitch2.Enabled = Ptr(true)

	results := RunDoctor(config, "config.toml")

	for _, binary := range []string{"mtp-folders", "mtp-files", "mtp-connect"} {
		result, found := findCheck(results, scopeDependencies, binary)
		if !found {
			t.Fatalf("the report holds no %s check", binary)
		}

		if result.Status != CheckFail {
			t.Errorf("%s status = %q, want %q", binary, result.Status, CheckFail)
		}

		if !strings.Contains(result.Remedy, "libmtp") {
			t.Errorf("%s remedy = %q, want it to name the libmtp package", binary, result.Remedy)
		}
	}

	if !HasFailure(results) {
		t.Error("HasFailure() = false, want true")
	}
}

func TestRunDoctor_WantsNoLibmtpToolForAnAlbumFolder(t *testing.T) {
	stubLookPath(t)

	config := newDoctorConfig(t)
	config.Providers.NintendoSwitch2.Enabled = Ptr(true)
	config.Providers.NintendoSwitch2.Path = Ptr(t.TempDir())

	results := RunDoctor(config, "config.toml")

	if _, found := findCheck(results, scopeDependencies, "mtp-connect"); found {
		t.Error("the report holds an mtp-connect check, and the provider reads a folder")
	}

	if HasFailure(results) {
		t.Error("HasFailure() = true, want false")
	}
}
