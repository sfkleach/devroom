package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/sfkleach/devroom/internal/config"
	devgit "github.com/sfkleach/devroom/internal/git"
)

// runTUI implements the command loop described in docs/devroom-proposal.md
// ("TUI commands"): a single letter or digit, followed by Enter. It's a thin
// dispatcher over the existing subcommands' run* functions, so all the real
// work (container creation, forge auth, etc.) stays in one place.
func runTUI() error {
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

	remoteURL, err := devgit.RemoteOrigin(root)
	if err != nil {
		return fmt.Errorf("reading git remote: %w", err)
	}
	owner, repo, err := devgit.OwnerRepo(remoteURL)
	if err != nil {
		return err
	}

	// A single shared reader for the whole session: bufio buffers ahead of
	// the current line, so creating a fresh reader per prompt would risk
	// dropping already-buffered input typed while a subcommand was running.
	reader := bufio.NewReader(os.Stdin)

	// The room list is shown on the first screen, then hidden again until
	// explicitly requested with 'l' — otherwise it reprints after every
	// single command, which drowns out the command legend.
	showRooms := true

	for {
		nicknames, err := listRoomNicknames(cfg.Runtime, owner, repo)
		if err != nil {
			return err
		}

		// Only worth checking when it would actually change what's printed:
		// the "no rooms yet" line, when it's about to be shown.
		baseBuilt := true
		if showRooms && len(nicknames) == 0 {
			baseBuilt = baseImageBuilt(cfg.Runtime, owner, repo)
		}

		printTUIMenu(nicknames, showRooms, baseBuilt)
		showRooms = false
		key, err := readCommand(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println()
				return nil
			}
			return err
		}
		// No extra newline here: a real terminal already echoes the Enter
		// keypress that ended the input line.

		switch {
		case key == 0:
			// Blank line: just re-show the menu.
		case key == 'n':
			if !baseImageBuilt(cfg.Runtime, owner, repo) {
				fmt.Println("No base image found. Press 'B' to build one first.")
				continue
			}
			nickname := promptLine(reader, "Nickname: ")
			if nickname == "" {
				fmt.Println("Aborted: no nickname given.")
				continue
			}
			newBranch = confirmYN("Create a branch matching the room name?", false)
			if err := runNew(newCmd, []string{nickname}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case key >= '1' && key <= '9':
			idx := int(key - '1')
			if idx >= len(nicknames) {
				fmt.Println("No such room.")
				continue
			}
			if err := runEnter(enterCmd, []string{nicknames[idx]}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case key == 'e':
			nickname := promptLine(reader, "Enter room: ")
			if nickname == "" {
				fmt.Println("Aborted: no nickname given.")
				continue
			}
			if !slices.Contains(nicknames, nickname) {
				fmt.Printf("No such room %q — press 'n' to create it.\n", nickname)
				continue
			}
			if err := runEnter(enterCmd, []string{nickname}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case key == 'l':
			showRooms = true
		case key == 'd':
			for _, nickname := range nicknames {
				if err := runDescribe(describeCmd, []string{nickname}); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}
		case key == 'c':
			if err := runConfigure(configureCmd, []string{}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			if newCfg, err := config.Load(root); err == nil {
				cfg = newCfg
			} else if !errors.Is(err, config.ErrNoConfig) {
				fmt.Fprintln(os.Stderr, err)
			}
		case key == 'R':
			nickname := promptLine(reader, "Retire room: ")
			if nickname == "" {
				fmt.Println("Aborted: no nickname given.")
				continue
			}
			if err := runRetire(retireCmd, []string{nickname}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case key == 'B':
			if err := runBuild(buildCmd, []string{}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case key == 'X':
			if err := runDestroy(destroyCmd, []string{}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case key == 'q':
			return nil
		default:
			fmt.Printf("Unknown command %q. Press q to quit.\n", key)
		}
		fmt.Println()
	}
}

// printTUIMenu prints the fixed key legend, preceded by the numbered room
// list (for the numbered-entry shortcut) only when showRooms is set — the
// list is shown once up front and then only again on request (the 'l' key),
// rather than after every single command. baseBuilt is only meaningful when
// the room list is empty, and hints at 'B' if the base image doesn't exist
// yet — the most likely reason there are no rooms.
func printTUIMenu(nicknames []string, showRooms, baseBuilt bool) {
	if showRooms {
		fmt.Println("Rooms")
		if len(nicknames) == 0 {
			if baseBuilt {
				fmt.Println("  (no rooms yet)")
			} else {
				fmt.Println("  (no rooms yet — press 'B' to build the base image first)")
			}
		}
		for i, nickname := range nicknames {
			if i >= 9 {
				fmt.Printf("  ... and %d more (press 'e' to enter by name)\n", len(nicknames)-9)
				break
			}
			fmt.Printf("  %d) %s\n", i+1, nickname)
		}
		fmt.Println()
	}
	fmt.Println("Commands")
	fmt.Println("  n  Create a new room")
	fmt.Println("  1-9  Enter the listed room")
	fmt.Println("  e  Enter a room by name")
	fmt.Println("  l  List rooms")
	fmt.Println("  d  Show AI-generated description of each room")
	fmt.Println("  c  Configure devroom")
	fmt.Println("  B  Build (or rebuild) the base image")
	fmt.Println("  R  Retire a room (container deleted, image kept)")
	fmt.Println("  X  Destroy the base image")
	fmt.Println("  q  Quit")
	fmt.Println()
	fmt.Print("command: ")
}

// readCommand reads one line of input and returns its first non-space byte
// as the command (0 for a blank line, io.EOF at end of input e.g. Ctrl-D).
func readCommand(reader *bufio.Reader) (byte, error) {
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return 0, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, nil
	}
	return line[0], nil
}

// promptLine reads a line of input, e.g. a nickname typed after a menu
// action, using the same shared reader as the main command loop.
func promptLine(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return ""
	}
	return strings.TrimSpace(line)
}
