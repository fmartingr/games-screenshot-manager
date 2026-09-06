package gamesscreenshotmanager

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// CheckStatus is the outcome of one check.
type CheckStatus string

const (
	// CheckOK reports a check that passed.
	CheckOK CheckStatus = "ok"
	// CheckWarn reports a run that works with a feature off or degraded.
	CheckWarn CheckStatus = "warn"
	// CheckFail reports an enabled provider that cannot work.
	CheckFail CheckStatus = "fail"
)

// The sections of the report, in the order they are printed.
const (
	scopeConfig       = "config"
	scopeProviders    = "providers"
	scopeDependencies = "dependencies"
	scopeGallery      = "gallery"
)

// The ANSI codes the report is painted with. A status is easier to find by its
// colour than by its word.
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
)

// painter writes the ANSI codes, or nothing at all when the output takes no
// colour.
type painter bool

// paint wraps text in the codes. Pad the text before it is painted, because a
// code counts as a character to a width verb such as %-4s.
func (p painter) paint(text string, codes ...string) string {
	if !p || len(codes) == 0 {
		return text
	}

	return strings.Join(codes, "") + text + ansiReset
}

// statusColor is the colour of one status.
func statusColor(status CheckStatus) string {
	switch status {
	case CheckOK:
		return ansiGreen
	case CheckWarn:
		return ansiYellow
	default:
		return ansiRed
	}
}

// takesColor reports a writer that shows the codes: a terminal, with NO_COLOR
// unset. See https://no-color.org
func takesColor(w io.Writer) bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}

	file, ok := w.(*os.File)
	if !ok {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

// CheckResult is one line of the report.
type CheckResult struct {
	Scope  string
	Name   string
	Status CheckStatus
	Detail string
	Remedy string
}

// lookPath finds a program on the system. It is a var, so a test can replace
// it with a stub.
var lookPath = exec.LookPath

// doctorProvider is what the doctor knows about one provider from the config
// alone, without building it.
type doctorProvider struct {
	name string
	// path is what the config asks for. "auto" and "" mean the provider finds
	// its own path.
	path string
	// autoPath reports a provider that can find its own path. A provider
	// without one needs a path in the config.
	autoPath bool
}

// RunDoctor checks the configuration and the system. It writes no file, it
// creates no directory, and it runs no provider.
func RunDoctor(config *Config, configPath string) []CheckResult {
	results := []CheckResult{{
		Scope:  scopeConfig,
		Name:   "config file",
		Status: CheckOK,
		Detail: configPath,
	}}

	results = append(results, checkOutputPath(config))
	results = append(results, checkProviders(config)...)
	results = append(results, checkGallery(config)...)

	uses, registryResults := providerRequirements(config)
	results = append(results, registryResults...)

	if config.Gallery.Create {
		builder, err := NewGalleryBuilder(*config)
		if err != nil {
			results = append(results, CheckResult{
				Scope:  scopeGallery,
				Name:   "gallery",
				Status: CheckFail,
				Detail: err.Error(),
			})
		} else {
			addUses(uses, "gallery", builder.Requirements())
		}
	}

	return append(results, checkRequirements(uses)...)
}

// checkOutputPath reports the folder every media file is copied into.
func checkOutputPath(config *Config) CheckResult {
	path := expandUser(config.OutputPath)

	result := CheckResult{Scope: scopeConfig, Name: "output_path", Status: CheckOK, Detail: path}

	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			result.Status = CheckFail
			result.Detail = path + " is a file, not a folder"
			result.Remedy = "Point output_path at a folder."
		}

		return result
	}

	parent := filepath.Dir(path)
	if _, err := os.Stat(parent); err != nil {
		result.Status = CheckFail
		result.Detail = path + " is missing, and so is the folder that would hold it"
		result.Remedy = "Create " + parent + ", or point output_path somewhere else."

		return result
	}

	result.Detail = path + ", which the run creates"

	return result
}

// checkProviders reports one line per enabled provider.
func checkProviders(config *Config) []CheckResult {
	providers := []doctorProvider{
		{name: "steam", path: "auto", autoPath: true},
		{name: "guild_wars_2", path: config.Providers.GuildWars2.GetPath(), autoPath: true},
		{name: "world_of_warcraft", path: config.Providers.WorldOfWarcraft.GetPath(), autoPath: true},
		{name: "minecraft", path: config.Providers.Minecraft.GetPath(), autoPath: true},
		{name: "hytale", path: config.Providers.Hytale.GetPath(), autoPath: true},
		{name: "nintendo_switch_2", path: config.Providers.NintendoSwitch2.GetPath(), autoPath: true},
		{name: "playstation4", path: config.Providers.PlayStation4.GetPath(), autoPath: false},
		{name: "playstation5", path: config.Providers.PlayStation5.GetPath(), autoPath: false},
		{name: "xbox_game_bar", path: config.Providers.XboxGameBar.GetPath(), autoPath: true},
	}

	enabled := map[string]bool{
		"steam":             config.Providers.Steam.IsEnabled(),
		"guild_wars_2":      config.Providers.GuildWars2.IsEnabled(),
		"world_of_warcraft": config.Providers.WorldOfWarcraft.IsEnabled(),
		"minecraft":         config.Providers.Minecraft.IsEnabled(),
		"hytale":            config.Providers.Hytale.IsEnabled(),
		"nintendo_switch_2": config.Providers.NintendoSwitch2.IsEnabled(),
		"playstation4":      config.Providers.PlayStation4.IsEnabled(),
		"playstation5":      config.Providers.PlayStation5.IsEnabled(),
		"xbox_game_bar":     config.Providers.XboxGameBar.IsEnabled(),
	}

	var results []CheckResult

	for _, provider := range providers {
		if !enabled[provider.name] {
			continue
		}

		result := checkProvider(provider)

		// The registry drops this provider on any other platform, so the host
		// is the only thing worth reporting about it there.
		if provider.name == "xbox_game_bar" && runtime.GOOS != "windows" {
			result.Status = CheckWarn
			result.Detail = "enabled, but this host runs " + runtime.GOOS + ", so the run skips it"
			result.Remedy = "Disable the provider, or run the tool on Windows."
		}

		results = append(results, result)
	}

	if len(results) == 0 {
		return []CheckResult{{
			Scope:  scopeProviders,
			Name:   "providers",
			Status: CheckWarn,
			Detail: "no provider is enabled, so the run collects nothing",
			Remedy: "Set enabled = true under [global] or under a provider.",
		}}
	}

	if config.Providers.Steam.IsEnabled() {
		results = append(results, checkSteam(config)...)
	}

	return results
}

// checkProvider reports the path one enabled provider works from.
func checkProvider(provider doctorProvider) CheckResult {
	result := CheckResult{
		Scope:  scopeProviders,
		Name:   provider.name,
		Status: CheckOK,
		Detail: "enabled",
	}

	if provider.path == "" || provider.path == "auto" {
		if !provider.autoPath {
			result.Status = CheckFail
			result.Detail = "enabled, but it has no path"
			result.Remedy = "This provider cannot find its own path. Set path in [providers." + provider.name + "]."

			return result
		}

		result.Detail = "enabled, and it finds its own path"

		return result
	}

	path := expandUser(provider.path)
	result.Detail = "enabled, path " + path

	if _, err := os.Stat(path); err != nil {
		result.Status = CheckFail
		result.Detail = "enabled, but the path is missing: " + path
		result.Remedy = "Point path in [providers." + provider.name + "] at a folder that exists."
	}

	return result
}

// checkSteam reports the Steam settings the run does not read the way the
// config file suggests.
func checkSteam(config *Config) []CheckResult {
	var results []CheckResult

	if path := config.Providers.Steam.GetUserDataPath(); path != "" && path != "auto" {
		results = append(results, CheckResult{
			Scope:  scopeProviders,
			Name:   "steam",
			Status: CheckWarn,
			Detail: "userdata_path is " + path + ", but the run reads the folder for this platform",
			Remedy: "The provider does not use userdata_path yet. Remove it, or set it to auto.",
		})
	}

	if config.Providers.Steam.OnlineGallery && config.Providers.Steam.UserID == "" {
		results = append(results, CheckResult{
			Scope:  scopeProviders,
			Name:   "steam",
			Status: CheckWarn,
			Detail: "online_gallery is on, but user_id is empty, so no published screenshot is collected",
			Remedy: "Set user_id in [providers.steam]. Check yours at https://steamdb.info/",
		})
	}

	return results
}

// checkGallery reports whether the gallery is built.
func checkGallery(config *Config) []CheckResult {
	if !config.Gallery.Create {
		return []CheckResult{{
			Scope:  scopeGallery,
			Name:   "create",
			Status: CheckOK,
			Detail: "off, so no page is written",
		}}
	}

	return []CheckResult{{
		Scope:  scopeGallery,
		Name:   "create",
		Status: CheckOK,
		Detail: "on",
	}}
}

// providerRequirements builds the enabled providers and asks each one what it
// needs. A provider that refuses to start is a failure on its own, and it
// leaves the requirements it would have declared out of the report.
func providerRequirements(config *Config) (map[string][]RequirementUse, []CheckResult) {
	uses := make(map[string][]RequirementUse)

	registry, err := NewProviderRegistry(config, NewGameManager(), NewFileManager(*config))
	if err != nil {
		return uses, []CheckResult{{
			Scope:  scopeProviders,
			Name:   "registry",
			Status: CheckFail,
			Detail: err.Error(),
			Remedy: "Fix the provider the message names. The checks below cover the rest.",
		}}
	}

	for binary, group := range registry.Requirements() {
		uses[binary] = append(uses[binary], group...)
	}

	return uses, nil
}

// addUses records the requirements one component declares.
func addUses(uses map[string][]RequirementUse, name string, requirements []Requirement) {
	for _, requirement := range requirements {
		uses[requirement.Binary] = append(uses[requirement.Binary], RequirementUse{
			Provider:    name,
			Requirement: requirement,
		})
	}
}

// checkRequirements looks for each program once, whichever component asked for
// it. A missing program is a failure only when something cannot run without
// it. It is a warning when every component that asked can work without it:
// the gallery, which is built either way, and a provider that marked the
// requirement optional.
func checkRequirements(uses map[string][]RequirementUse) []CheckResult {
	binaries := make([]string, 0, len(uses))
	for binary := range uses {
		binaries = append(binaries, binary)
	}
	sort.Strings(binaries)

	results := make([]CheckResult, 0, len(binaries))

	for _, binary := range binaries {
		group := uses[binary]
		sort.Slice(group, func(i, j int) bool { return group[i].Provider < group[j].Provider })

		result := CheckResult{
			Scope:  scopeDependencies,
			Name:   binary,
			Status: CheckOK,
			Remedy: "wanted by " + describeUses(group),
		}

		path, err := lookPath(binary)
		if err == nil {
			result.Detail = path
			results = append(results, result)

			continue
		}

		result.Status = CheckFail
		if everyUseCanDoWithout(group) {
			result.Status = CheckWarn
		}

		result.Detail = "not found in PATH"
		result.Remedy = fmt.Sprintf("Install the %s package. It is wanted by %s.%s",
			group[0].Requirement.Package, describeUses(group), describeDegradation(group))

		results = append(results, result)
	}

	return results
}

// everyUseCanDoWithout reports a program that nothing needs to run. The
// gallery is built without any program it asks for, and a provider says so for
// itself with Requirement.Optional.
func everyUseCanDoWithout(group []RequirementUse) bool {
	for _, use := range group {
		if use.Provider != "gallery" && !use.Requirement.Optional {
			return false
		}
	}

	return true
}

// describeDegradation names what a run loses without an optional program. It
// returns an empty string when no use declares one, so the remedy reads the
// same as before for a program that is simply needed.
func describeDegradation(group []RequirementUse) string {
	parts := make([]string, 0, len(group))
	for _, use := range group {
		if use.Requirement.Optional && use.Requirement.Degradation != "" {
			parts = append(parts, use.Requirement.Degradation)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return " Without it, " + strings.Join(parts, ", and ") + "."
}

// describeUses names every component that wants a program, and why.
func describeUses(group []RequirementUse) string {
	parts := make([]string, 0, len(group))
	for _, use := range group {
		parts = append(parts, fmt.Sprintf("%s (%s)", use.Provider, use.Requirement.Reason))
	}

	return strings.Join(parts, ", ")
}

// PrintChecks writes the report. It goes to stdout rather than through the
// logger, because the report is the product of the command.
func PrintChecks(w io.Writer, results []CheckResult) {
	printChecks(w, results, painter(takesColor(w)))
}

// printChecks writes the report with the colours a caller asks for. A test
// reads both forms through it.
func printChecks(w io.Writer, results []CheckResult, colors painter) {
	sections := []struct {
		scope string
		title string
	}{
		{scopeConfig, "Configuration"},
		{scopeProviders, "Providers"},
		{scopeDependencies, "Dependencies"},
		{scopeGallery, "Gallery"},
	}

	for _, section := range sections {
		lines := resultsFor(results, section.scope)
		if len(lines) == 0 {
			continue
		}

		fmt.Fprintf(w, "%s\n", colors.paint(section.title, ansiBold))

		for _, line := range lines {
			status := colors.paint(fmt.Sprintf("%-4s", line.Status), ansiBold, statusColor(line.Status))
			name := colors.paint(fmt.Sprintf("%-18s", line.Name), ansiBold)

			fmt.Fprintf(w, "  %s  %s %s\n", status, name, line.Detail)

			if line.Remedy != "" {
				fmt.Fprintf(w, "  %-4s  %-18s %s\n", "", "", line.Remedy)
			}
		}

		fmt.Fprintln(w)
	}

	problems := CountProblems(results)
	if problems == 0 {
		fmt.Fprintln(w, colors.paint("No problems found.", ansiBold, ansiGreen))

		return
	}

	summary := fmt.Sprintf("%d problems found.", problems)
	if problems == 1 {
		summary = "1 problem found."
	}

	color := ansiYellow
	if HasFailure(results) {
		color = ansiRed
	}

	fmt.Fprintln(w, colors.paint(summary, ansiBold, color))
}

// resultsFor returns the lines of one section, in the order they were added.
func resultsFor(results []CheckResult, scope string) []CheckResult {
	var lines []CheckResult

	for _, result := range results {
		if result.Scope == scope {
			lines = append(lines, result)
		}
	}

	return lines
}

// CountProblems counts the checks that did not pass.
func CountProblems(results []CheckResult) int {
	problems := 0

	for _, result := range results {
		if result.Status != CheckOK {
			problems++
		}
	}

	return problems
}

// CountFailures counts the checks that failed.
func CountFailures(results []CheckResult) int {
	failures := 0

	for _, result := range results {
		if result.Status == CheckFail {
			failures++
		}
	}

	return failures
}

// HasFailure reports a run that cannot work as configured.
func HasFailure(results []CheckResult) bool {
	return CountFailures(results) > 0
}
