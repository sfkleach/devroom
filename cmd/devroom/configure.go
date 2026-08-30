package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/sfkleach/devroom/internal/config"
	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Interactively edit this repo's devroom.toml",
	Args:  cobra.NoArgs,
	RunE:  runConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)
}

// configureSession holds the config file this configure invocation is
// editing, seeded only from the repo-level file — never the three-level
// merged config.Load() result, since seeding from the merge would risk
// silently "promoting" a value that's only set at the user/system level
// into the repo file the moment the session saves.
type configureSession struct {
	path      string
	isNew     bool
	cfg       *config.Config
	undecoded []string // dotted key names toml.MetaData.Undecoded() didn't recognize
}

func loadConfigureSession(path string) (*configureSession, error) {
	cfg := &config.Config{}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return &configureSession{path: path, isNew: true, cfg: cfg}, nil
		}
		return nil, err
	}
	meta, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	var undecoded []string
	for _, k := range meta.Undecoded() {
		undecoded = append(undecoded, k.String())
	}
	return &configureSession{path: path, cfg: cfg, undecoded: undecoded}, nil
}

func runConfigure(cmd *cobra.Command, args []string) error {
	root, err := effectiveRootDir()
	if err != nil {
		return err
	}
	path := filepath.Join(root, ".config", "devroom", "devroom.toml")

	sess, err := loadConfigureSession(path)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)

	if len(sess.undecoded) > 0 {
		fmt.Printf("Warning: %s has %d key(s) this version of devroom doesn't recognise:\n", path, len(sess.undecoded))
		for _, k := range sess.undecoded {
			fmt.Printf("  - %s\n", k)
		}
		fmt.Println("Saving from this session will drop them.")
		fmt.Println()
	}

	for {
		printConfigureMenu(sess)
		key, err := readCommand(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println()
				return nil
			}
			return err
		}
		switch key {
		case 0:
			// Blank line: just re-show the menu.
		case '1':
			editField(reader, runtimeField(sess.cfg))
		case '2':
			editField(reader, plainField("base_image", "Any image ref the runtime can pull (e.g. from Docker Hub).", &sess.cfg.BaseImage))
		case '3':
			editField(reader, plainField("build_script", "Runs during 'devroom build', baked into the shared base image.", &sess.cfg.BuildScript))
		case '4':
			editField(reader, plainField("enter_script", "Sourced during 'devroom enter', just before the interactive shell starts.", &sess.cfg.EnterScript))
		case '5':
			editField(reader, plainField("leave_script", "Run during 'devroom enter', after the interactive shell exits.", &sess.cfg.LeaveScript))
		case '6':
			editField(reader, aiDefaultField(sess.cfg))
		case 'a':
			manageAIEntries(reader, sess.cfg)
		case 's':
			saved, err := saveConfigureSession(sess, reader)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			if saved {
				return nil
			}
		case 'r':
			newSess, err := loadConfigureSession(path)
			if err != nil {
				return err
			}
			sess = newSess
			fmt.Println("Reloaded from disk; unsaved changes discarded.")
		case 'q':
			return nil
		default:
			fmt.Printf("Unknown command %q.\n", key)
		}
		fmt.Println()
	}
}

func printConfigureMenu(sess *configureSession) {
	if sess.isNew {
		fmt.Printf("Editing %s (new — not yet saved)\n", sess.path)
	} else {
		fmt.Printf("Editing %s (existing file)\n", sess.path)
	}
	fmt.Println()
	fmt.Printf("  1) runtime          = %s\n", displayOrUnset(sess.cfg.Runtime))
	fmt.Printf("  2) base_image       = %s\n", displayOrUnset(sess.cfg.BaseImage))
	fmt.Printf("  3) build_script     = %s\n", displayOrUnset(sess.cfg.BuildScript))
	fmt.Printf("  4) enter_script     = %s\n", displayOrUnset(sess.cfg.EnterScript))
	fmt.Printf("  5) leave_script     = %s\n", displayOrUnset(sess.cfg.LeaveScript))
	fmt.Printf("  6) ai_default       = %s\n", displayOrUnset(sess.cfg.AIDefault))
	fmt.Println()
	fmt.Printf("  AI entries (%d): %s\n", len(sess.cfg.AI), summarizeAIEntries(sess.cfg.AI))
	fmt.Println("  a) Manage AI entries...")
	fmt.Println()
	fmt.Println("  s) Save and quit")
	fmt.Println("  r) Reload from disk (discard unsaved changes)")
	fmt.Println("  q) Quit without saving")
	fmt.Println()
	fmt.Print("command: ")
}

func displayOrUnset(v string) string {
	if v == "" {
		return "(unset)"
	}
	return v
}

func summarizeAIEntries(entries []config.AIEntry) string {
	if len(entries) == 0 {
		return "(none configured)"
	}
	parts := make([]string, len(entries))
	for i, e := range entries {
		status := "enabled"
		if !e.IsEnabled() {
			status = "disabled"
		}
		parts[i] = fmt.Sprintf("%s (%s)", e.Name, status)
	}
	return strings.Join(parts, ", ")
}

// configField is one editable field in the generic field-editor menu, used
// for both top-level Config scalars and per-AIEntry fields. get/set operate
// directly on the field's storage via closure, so there's no reflection or
// generics involved — just a small vtable-by-hand.
type configField struct {
	label string
	help  string
	get   func() string
	set   func(v string) error // v is never "-"; "" here means "clear"
}

// editField runs the single-field edit loop: show current value, accept a
// new value, blank line to leave unchanged, "-" to explicitly clear. Loops
// on validation failure (set returning an error) rather than aborting the
// edit.
func editField(reader *bufio.Reader, f configField) {
	for {
		fmt.Println()
		fmt.Printf("-- %s --\n", f.label)
		fmt.Println(f.help)
		fmt.Printf("Current value: %s\n", displayOrUnset(f.get()))
		fmt.Println("Enter a new value, '-' to clear, or press Enter to leave unchanged.")
		input := promptLine(reader, "> ")
		if input == "" {
			return
		}
		newVal := input
		if input == "-" {
			newVal = ""
		}
		if err := f.set(newVal); err != nil {
			fmt.Println("Error:", err)
			continue
		}
		return
	}
}

// plainField is the shared constructor for string fields with no
// validation beyond the generic clear/leave-unchanged mechanics.
func plainField(label, help string, slot *string) configField {
	return configField{
		label: label,
		help:  help,
		get:   func() string { return *slot },
		set:   func(v string) error { *slot = v; return nil },
	}
}

// stringSliceField handles comma-separated list fields (credential_paths,
// env): each part is trimmed and empty parts dropped; clearing sets nil.
func stringSliceField(label, help string, slot *[]string) configField {
	return configField{
		label: label,
		help:  help,
		get:   func() string { return strings.Join(*slot, ", ") },
		set: func(v string) error {
			if v == "" {
				*slot = nil
				return nil
			}
			var vals []string
			for part := range strings.SplitSeq(v, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					vals = append(vals, part)
				}
			}
			*slot = vals
			return nil
		},
	}
}

func runtimeField(cfg *config.Config) configField {
	return configField{
		label: "runtime",
		help:  "Container runtime: docker or podman.",
		get:   func() string { return cfg.Runtime },
		set: func(v string) error {
			if v != "" && v != "docker" && v != "podman" {
				return fmt.Errorf("must be \"docker\" or \"podman\"")
			}
			cfg.Runtime = v
			return nil
		},
	}
}

func aiDefaultField(cfg *config.Config) configField {
	return configField{
		label: "ai_default",
		help:  "Which [[ai]] entry backs 'devroom describe'.",
		get:   func() string { return cfg.AIDefault },
		set: func(v string) error {
			cfg.AIDefault = v
			if v != "" && !hasAIEntry(cfg.AI, v) {
				fmt.Printf("Note: no [[ai]] entry named %q yet.\n", v)
			}
			return nil
		},
	}
}

func hasAIEntry(entries []config.AIEntry, name string) bool {
	for _, e := range entries {
		if e.Name == name {
			return true
		}
	}
	return false
}

// confirmYNReader mirrors confirmYN's Y/n prompt, but reads from the
// session's shared bufio.Reader instead of opening a fresh
// bufio.Scanner(os.Stdin). configure interleaves confirmations with
// ordinary menu input constantly (delete-entry, save-with-warnings); two
// independent buffered readers racing on the same os.Stdin can silently
// steal each other's already-buffered bytes, which would make scripted
// stdin (see tests/functest.sh) flaky. confirmYN's other call sites are
// left untouched — this is scoped to configure only.
func confirmYNReader(reader *bufio.Reader, prompt string, def bool) bool {
	suffix := " (y/N): "
	if def {
		suffix = " (Y/n): "
	}
	answer := strings.ToLower(promptLine(reader, prompt+suffix))
	if answer == "" {
		return def
	}
	return answer == "y" || answer == "yes"
}

func manageAIEntries(reader *bufio.Reader, cfg *config.Config) {
	for {
		fmt.Println()
		fmt.Println("AI entries")
		fmt.Println()
		if len(cfg.AI) == 0 {
			fmt.Println("  (none configured)")
		}
		for i, e := range cfg.AI {
			status := "enabled"
			if !e.IsEnabled() {
				status = "disabled"
			}
			fmt.Printf("  %d) %-20s %s\n", i+1, e.Name, status)
		}
		fmt.Println()
		fmt.Println("  1-9  Edit the listed entry")
		fmt.Println("  a    Add a new entry")
		fmt.Println("  d    Delete an entry")
		fmt.Println("  b    Back to main menu")
		fmt.Println()
		key, err := readCommand(reader)
		if err != nil {
			return
		}
		switch {
		case key == 0:
		case key >= '1' && key <= '9':
			idx := int(key - '1')
			if idx >= len(cfg.AI) {
				fmt.Println("No such entry.")
				continue
			}
			editAIEntry(reader, cfg, idx)
		case key == 'a':
			name := promptLine(reader, "Name for the new [[ai]] entry: ")
			if name == "" {
				fmt.Println("Aborted: no name given.")
				continue
			}
			idx := -1
			for i, e := range cfg.AI {
				if e.Name == name {
					idx = i
					break
				}
			}
			if idx >= 0 {
				fmt.Printf("An entry named %q already exists — editing it instead.\n", name)
			} else {
				cfg.AI = append(cfg.AI, config.AIEntry{Name: name})
				idx = len(cfg.AI) - 1
			}
			editAIEntry(reader, cfg, idx)
		case key == 'd':
			numStr := promptLine(reader, "Delete entry number: ")
			idx, ok := parseEntryNumber(numStr, len(cfg.AI))
			if !ok {
				fmt.Println("Invalid entry number.")
				continue
			}
			entry := cfg.AI[idx]
			if confirmYNReader(reader, fmt.Sprintf("Delete [[ai]] entry %q?", entry.Name), false) {
				cfg.AI = slices.Delete(cfg.AI, idx, idx+1)
				fmt.Printf("Deleted %q.\n", entry.Name)
			} else {
				fmt.Println("Not deleted.")
			}
		case key == 'b':
			return
		default:
			fmt.Printf("Unknown command %q.\n", key)
		}
	}
}

func parseEntryNumber(s string, count int) (int, bool) {
	if len(s) != 1 || s[0] < '1' || s[0] > '9' {
		return 0, false
	}
	idx := int(s[0] - '1')
	if idx >= count {
		return 0, false
	}
	return idx, true
}

func editAIEntry(reader *bufio.Reader, cfg *config.Config, idx int) {
	entry := &cfg.AI[idx]
	for {
		fmt.Println()
		fmt.Printf("Editing [[ai]] entry %d of %d\n", idx+1, len(cfg.AI))
		fmt.Println()
		fmt.Printf("  1) name             = %s\n", displayOrUnset(entry.Name))
		fmt.Printf("  2) enabled          = %s\n", displayEnabled(entry.Enabled))
		fmt.Printf("  3) install_command  = %s\n", displayOrUnset(entry.InstallCommand))
		fmt.Printf("  4) credential_paths = %s\n", displayOrUnset(strings.Join(entry.CredentialPaths, ", ")))
		fmt.Printf("  5) describe_command = %s\n", displayOrUnset(entry.DescribeCommand))
		fmt.Printf("  6) env              = %s\n", displayOrUnset(strings.Join(entry.Env, ", ")))
		fmt.Println()
		fmt.Println("  b) Back to AI entries list")
		fmt.Println()
		key, err := readCommand(reader)
		if err != nil {
			return
		}
		switch key {
		case 0:
		case '1':
			editField(reader, plainField("name", "Identifier referenced by ai_default.", &entry.Name))
		case '2':
			editField(reader, enabledField(entry))
		case '3':
			editField(reader, plainField("install_command", "Run during 'devroom build', baked into the shared base image.", &entry.InstallCommand))
		case '4':
			editField(reader, stringSliceField("credential_paths", "Comma-separated host paths bind-mounted read-only into every room.", &entry.CredentialPaths))
		case '5':
			editField(reader, plainField("describe_command", "Command used inside the container for 'devroom describe'; {} is the prompt.", &entry.DescribeCommand))
		case '6':
			editField(reader, stringSliceField("env", "Comma-separated host environment variable names forwarded into every room.", &entry.Env))
		case 'b':
			return
		default:
			fmt.Printf("Unknown command %q.\n", key)
		}
	}
}

func displayEnabled(v *bool) string {
	if v == nil {
		return "(default: true)"
	}
	if *v {
		return "true"
	}
	return "false"
}

// enabledField handles the tri-state *bool: unset (nil, default true),
// explicit true, or explicit false.
func enabledField(entry *config.AIEntry) configField {
	return configField{
		label: "enabled",
		help:  "Installed/mounted into every room unless set to false. Leave unset to default to true.",
		get:   func() string { return displayEnabled(entry.Enabled) },
		set: func(v string) error {
			switch strings.ToLower(v) {
			case "":
				entry.Enabled = nil
			case "true", "t", "yes", "y":
				b := true
				entry.Enabled = &b
			case "false", "f", "no", "n":
				b := false
				entry.Enabled = &b
			default:
				return fmt.Errorf("must be true, false, or blank to unset")
			}
			return nil
		},
	}
}

// buildConfigureOutput renders cfg as the full contents of devroom.toml,
// regenerated from scratch on every save (see init.go's buildInitConfig for
// the one-shot-scaffold precedent this adapts). Scalar keys are emitted
// only when non-empty; [[ai]] blocks are always emitted last, which is a
// hard TOML requirement (a bare key = value after a table header belongs to
// that table, not the top level) — satisfied here by construction, since
// scalars are written first regardless of which ones are set.
func buildConfigureOutput(cfg *config.Config) string {
	var b strings.Builder
	writeScalar := func(comment, key, value string) {
		if value == "" {
			return
		}
		fmt.Fprintf(&b, "# %s\n%s = %s\n\n", comment, key, tomlQuote(value))
	}
	writeScalar("Container runtime: docker or podman.", "runtime", cfg.Runtime)
	writeScalar("Base image (any ref the runtime can pull, e.g. from Docker Hub).", "base_image", cfg.BaseImage)
	writeScalar("Runs during 'devroom build', baked into the shared base image.", "build_script", cfg.BuildScript)
	writeScalar("Sourced during 'devroom enter', just before the interactive shell starts.", "enter_script", cfg.EnterScript)
	writeScalar("Run during 'devroom enter', after the interactive shell exits.", "leave_script", cfg.LeaveScript)
	if cfg.AIDefault != "" {
		b.WriteString("# Which [[ai]] entry backs 'devroom describe'.\n")
		fmt.Fprintf(&b, "ai_default = %s\n\n", tomlQuote(cfg.AIDefault))
	}
	if len(cfg.AI) > 0 {
		for _, ai := range cfg.AI {
			b.WriteString("[[ai]]\n")
			fmt.Fprintf(&b, "name = %s\n", tomlQuote(ai.Name))
			if ai.Enabled != nil {
				fmt.Fprintf(&b, "enabled = %v\n", *ai.Enabled)
			}
			if ai.InstallCommand != "" {
				fmt.Fprintf(&b, "install_command = %s\n", tomlQuote(ai.InstallCommand))
			}
			if len(ai.CredentialPaths) > 0 {
				fmt.Fprintf(&b, "credential_paths = %s\n", tomlStringArray(ai.CredentialPaths))
			}
			if ai.DescribeCommand != "" {
				fmt.Fprintf(&b, "describe_command = %s\n", tomlQuote(ai.DescribeCommand))
			}
			if len(ai.Env) > 0 {
				fmt.Fprintf(&b, "env = %s\n", tomlStringArray(ai.Env))
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// tomlQuote renders s as a double-quoted TOML basic string. init.go's
// buildInitConfig never needed this because its only interpolated value
// (runtime) comes from detectRuntime()'s fixed {"docker","podman"} set;
// configure's fields are freely typed, so a value containing a double quote
// or backslash would otherwise produce invalid or semantically wrong TOML
// if interpolated raw.
func tomlQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func tomlStringArray(vals []string) string {
	quoted := make([]string, len(vals))
	for i, v := range vals {
		quoted[i] = tomlQuote(v)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// saveConfigureSession writes the session's config to disk. It returns
// saved=false (without error) if the user declines the drop-warning
// confirmation, so the caller can keep the menu loop running instead of
// exiting.
func saveConfigureSession(sess *configureSession, reader *bufio.Reader) (saved bool, err error) {
	if len(sess.undecoded) > 0 {
		fmt.Printf("Saving will drop %d unrecognised key(s): %s\n", len(sess.undecoded), strings.Join(sess.undecoded, ", "))
		if !confirmYNReader(reader, "Continue?", false) {
			fmt.Println("Not saved.")
			return false, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(sess.path), 0755); err != nil {
		return false, fmt.Errorf("creating config directory: %w", err)
	}
	content := buildConfigureOutput(sess.cfg)
	if err := os.WriteFile(sess.path, []byte(content), 0644); err != nil {
		return false, fmt.Errorf("writing config: %w", err)
	}
	fmt.Printf("Saved %s\n", sess.path)
	return true, nil
}
