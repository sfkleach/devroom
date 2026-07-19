package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/sfkleach/devroom/internal/config"
	devgit "github.com/sfkleach/devroom/internal/git"
	"github.com/spf13/cobra"
)

var listStatistics bool
var listBranch bool
var listImage bool
var listFormat string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the rooms for this repo",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func init() {
	listCmd.Flags().BoolVarP(&listStatistics, "statistics", "s", false, "Include container statistics (state, built, last entered, size)")
	listCmd.Flags().BoolVarP(&listBranch, "branch", "b", false, "Include the room's current branch")
	listCmd.Flags().BoolVarP(&listImage, "image", "i", false, "Include the image the room's container was built from, and whether it's current")
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "", `Output format: "json" or "md" (default: plain text)`)
	rootCmd.AddCommand(listCmd)
}

// listColumn pairs a column's display header (used in text/md output) with
// its JSON key.
type listColumn struct {
	header string
	key    string
}

func runList(cmd *cobra.Command, args []string) error {
	switch listFormat {
	case "", "json", "md":
	default:
		return fmt.Errorf(`invalid --format %q: must be "json" or "md"`, listFormat)
	}

	root, err := effectiveRootDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		if errors.Is(err, config.ErrNoConfig) {
			fmt.Fprintln(os.Stderr, "No devroom configuration file found.")
			fmt.Fprintln(os.Stderr, "Hint: run 'devroom init' to create one.")
			os.Exit(1)
		}
		return err
	}

	if cfg.Runtime == "" {
		return fmt.Errorf("missing required config key 'runtime' in devroom.toml")
	}

	remoteURL, err := devgit.RemoteOrigin(root)
	if err != nil {
		return fmt.Errorf("reading git remote: %w", err)
	}
	owner, repo, err := devgit.OwnerRepo(remoteURL)
	if err != nil {
		return err
	}

	nicknames, err := listRoomNicknames(cfg.Runtime, owner, repo)
	if err != nil {
		return err
	}

	if len(nicknames) == 0 {
		if listFormat == "json" {
			fmt.Println("[]")
		} else {
			fmt.Println("No rooms found for this repo.")
		}
		return nil
	}

	// The bare-nickname shortcut (no header, no extra columns) only applies
	// to the default text format; -f json/md always produce a structured
	// one-column result so the output shape doesn't depend on which other
	// flags happen to be set.
	if listFormat == "" && !listBranch && !listStatistics && !listImage {
		for _, nickname := range nicknames {
			fmt.Println(nickname)
		}
		return nil
	}

	columns := []listColumn{{"NICKNAME", "nickname"}}
	if listBranch {
		columns = append(columns, listColumn{"BRANCH", "branch"})
	}
	if listStatistics {
		columns = append(columns,
			listColumn{"STATE", "state"},
			listColumn{"BUILT", "built"},
			listColumn{"LAST ENTERED", "lastEntered"},
			listColumn{"SIZE", "size"},
		)
	}
	if listImage {
		columns = append(columns, listColumn{"IMAGE", "image"})
	}

	baseImage := fmt.Sprintf("localhost/dev-%s-%s:base", owner, repo)

	rows := make([][]string, 0, len(nicknames))
	for _, nickname := range nicknames {
		containerName := fmt.Sprintf("devroom-%s-%s-%s", owner, repo, nickname)
		row := []string{nickname}
		if listBranch {
			branch, err := roomBranch(cfg.Runtime, containerName)
			if err != nil {
				return err
			}
			row = append(row, branch)
		}
		if listStatistics {
			stats, err := containerStatistics(cfg.Runtime, containerName)
			if err != nil {
				return err
			}
			row = append(row, stats.state, stats.built, stats.lastEntered, stats.size)
		}
		if listImage {
			image, err := roomImage(cfg.Runtime, containerName, baseImage)
			if err != nil {
				return err
			}
			row = append(row, image)
		}
		rows = append(rows, row)
	}

	switch listFormat {
	case "json":
		return printListJSON(columns, rows)
	case "md":
		printListMarkdown(columns, rows)
		return nil
	default:
		printListText(columns, rows)
		return nil
	}
}

// printListText renders rows as a tabwriter-aligned plain-text table.
func printListText(columns []listColumn, rows [][]string) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers(columns), "\t"))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}

// printListMarkdown renders rows as a GitHub-flavoured markdown table,
// space-padded so it stays visually aligned in a fixed-width font.
func printListMarkdown(columns []listColumn, rows [][]string) {
	widths := make([]int, len(columns))
	for i, c := range columns {
		widths[i] = len(c.header)
	}
	for _, row := range rows {
		for i, v := range row {
			if len(v) > widths[i] {
				widths[i] = len(v)
			}
		}
	}

	printMarkdownRow := func(cells []string) {
		var sb strings.Builder
		sb.WriteString("|")
		for i, cell := range cells {
			sb.WriteString(" ")
			sb.WriteString(cell)
			sb.WriteString(strings.Repeat(" ", widths[i]-len(cell)))
			sb.WriteString(" |")
		}
		fmt.Println(sb.String())
	}

	printMarkdownRow(headers(columns))

	separators := make([]string, len(columns))
	for i, w := range widths {
		separators[i] = strings.Repeat("-", w)
	}
	printMarkdownRow(separators)

	for _, row := range rows {
		printMarkdownRow(row)
	}
}

// printListJSON renders rows as a JSON array of objects keyed by column.
func printListJSON(columns []listColumn, rows [][]string) error {
	objs := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		obj := make(map[string]string, len(columns))
		for i, c := range columns {
			obj[c.key] = row[i]
		}
		objs = append(objs, obj)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(objs)
}

func headers(columns []listColumn) []string {
	h := make([]string, len(columns))
	for i, c := range columns {
		h[i] = c.header
	}
	return h
}

// listRoomNicknames returns the nicknames of every room container (running
// or stopped) for the given owner/repo, sorted alphabetically.
func listRoomNicknames(runtime, owner, repo string) ([]string, error) {
	prefix := fmt.Sprintf("devroom-%s-%s-", owner, repo)

	out, err := exec.Command(runtime, "ps", "-a", "--filter", "name=^"+prefix, "--format", "{{.Names}}").Output()
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	var nicknames []string
	for name := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if name == "" {
			continue
		}
		nicknames = append(nicknames, strings.TrimPrefix(name, prefix))
	}
	sort.Strings(nicknames)
	return nicknames, nil
}

// roomBranch returns the current branch checked out in the room's
// ~/workspace clone. Reading it means exec-ing git inside the container,
// which only works while it's running; for a stopped room this returns a
// placeholder rather than starting the container as a side effect of what
// should be a read-only command (starting it would also disturb the
// "last entered" statistic shown by -s).
func roomBranch(runtime, containerName string) (string, error) {
	state, err := containerState(runtime, containerName)
	if err != nil {
		return "", err
	}
	if state != "running" {
		return "(stopped)", nil
	}

	u, err := user.Current()
	if err != nil {
		return "", err
	}
	workspace := u.HomeDir + "/workspace"

	out, err := exec.Command(runtime, "exec", "--user", u.Uid+":"+u.Gid,
		containerName, "git", "-C", workspace, "rev-parse", "--abbrev-ref", "HEAD",
	).Output()
	if err != nil {
		return "", fmt.Errorf("reading branch for %q: %w", containerName, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// roomImage returns the short ID of the image a room's container was
// created from, annotated with whether it's still "current" (matches the
// live baseImage tag) or "stale" (the tag has since been rebuilt, so this
// room is running on an older image than a new `devroom new`/`enter` would
// get). "unknown" means the baseImage tag itself couldn't be inspected
// (e.g. it was removed since this room was created).
func roomImage(runtime, containerName, baseImage string) (string, error) {
	containerImageID, err := imageID(runtime, containerName, "{{.Image}}")
	if err != nil {
		return "", fmt.Errorf("inspecting image for %q: %w", containerName, err)
	}

	marker := "unknown"
	if currentImageID, err := imageID(runtime, baseImage, "{{.Id}}"); err == nil {
		if containerImageID == currentImageID {
			marker = "current"
		} else {
			marker = "stale"
		}
	}

	return fmt.Sprintf("%s (%s)", shortImageID(containerImageID), marker), nil
}

// imageID inspects a container or image reference for a single field.
func imageID(runtime, ref, format string) (string, error) {
	out, err := exec.Command(runtime, "inspect", "--format", format, ref).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// shortImageID renders an image ID the way `docker images`/`podman images`
// do: a 12-character prefix, with any "sha256:" prefix stripped first.
func shortImageID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// roomStats holds the per-room details shown by `devroom list -s`.
type roomStats struct {
	state       string
	built       string
	lastEntered string
	size        string
}

// containerStatisticsFormat asks the container engine to do the timestamp
// formatting itself (Go templates support calling time.Time.Format), so no
// date parsing is needed on this side. Fields are '|'-separated since none
// of them can contain that character.
const containerStatisticsFormat = `{{.State.Status}}|{{.Created.Format "2006-01-02 15:04"}}|{{.State.StartedAt.Format "2006-01-02 15:04"}}|{{.SizeRw}}|{{.SizeRootFs}}`

// containerStatistics inspects a single container for the fields shown by
// `devroom list -s`. "Built" is the container's creation time (i.e. when the
// room was first created); "last entered" is when it was last started,
// which updates each time a stopped room is resumed.
func containerStatistics(runtime, containerName string) (roomStats, error) {
	out, err := exec.Command(runtime, "inspect", "--size", "--format", containerStatisticsFormat, containerName).Output()
	if err != nil {
		return roomStats{}, fmt.Errorf("inspecting container %q: %w", containerName, err)
	}

	fields := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(fields) != 5 {
		return roomStats{}, fmt.Errorf("unexpected inspect output for %q: %q", containerName, out)
	}
	status, built, lastEntered, sizeRwStr, sizeRootFsStr := fields[0], fields[1], fields[2], fields[3], fields[4]

	state := "stopped"
	if status == "running" {
		state = "running"
	}

	sizeRw, _ := strconv.ParseInt(sizeRwStr, 10, 64)
	sizeRootFs, _ := strconv.ParseInt(sizeRootFsStr, 10, 64)

	return roomStats{
		state:       state,
		built:       built,
		lastEntered: lastEntered,
		size:        formatBytes(sizeRw + sizeRootFs),
	}, nil
}

// formatBytes renders a byte count as a human-readable binary size (KiB,
// MiB, ...).
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n2 := n / unit; n2 >= unit; n2 /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
