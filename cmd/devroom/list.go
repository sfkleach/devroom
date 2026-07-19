package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/sfkleach/devroom/internal/config"
	devgit "github.com/sfkleach/devroom/internal/git"
	"github.com/spf13/cobra"
)

var listStatistics bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the rooms for this repo",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func init() {
	listCmd.Flags().BoolVarP(&listStatistics, "statistics", "s", false, "Include container statistics (state, built, last entered, size)")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
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
		fmt.Println("No rooms found for this repo.")
		return nil
	}

	if !listStatistics {
		for _, nickname := range nicknames {
			fmt.Println(nickname)
		}
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "NICKNAME\tSTATE\tBUILT\tLAST ENTERED\tSIZE")
	for _, nickname := range nicknames {
		containerName := fmt.Sprintf("devroom-%s-%s-%s", owner, repo, nickname)
		stats, err := containerStatistics(cfg.Runtime, containerName)
		if err != nil {
			return err
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", nickname, stats.state, stats.built, stats.lastEntered, stats.size)
	}
	return tw.Flush()
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
