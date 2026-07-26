package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sfkleach/devroom/internal/config"
	devgit "github.com/sfkleach/devroom/internal/git"
	"golang.org/x/term"
)

// runTUI implements the single-keypress command loop described in
// docs/devroom-proposal.md ("TUI commands"). It's a thin dispatcher over the
// existing subcommands' run* functions, so all the real work (container
// creation, forge auth, etc.) stays in one place.
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

	for {
		nicknames, err := listRoomNicknames(cfg.Runtime, owner, repo)
		if err != nil {
			return err
		}

		printTUIMenu(nicknames)
		key, err := readKey()
		if err != nil {
			return err
		}
		fmt.Println()

		switch {
		case key == 'n':
			nickname := promptLine("Nickname: ")
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
			nickname := promptLine("Enter room: ")
			if nickname == "" {
				fmt.Println("Aborted: no nickname given.")
				continue
			}
			if err := runEnter(enterCmd, []string{nickname}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case key == 'l':
			if err := runList(listCmd, []string{}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case key == 'd':
			for _, nickname := range nicknames {
				if err := runDescribe(describeCmd, []string{nickname}); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}
		case key == 'Q':
			nickname := promptLine("Close room: ")
			if nickname == "" {
				fmt.Println("Aborted: no nickname given.")
				continue
			}
			if err := runClose(closeCmd, []string{nickname}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case key == 'X':
			if err := runDestroy(destroyCmd, []string{}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case key == 'q', key == 3: // 3 = Ctrl-C, swallowed by raw mode
			return nil
		default:
			fmt.Printf("Unknown key %q. Press q to quit.\n", key)
		}
		fmt.Println()
	}
}

// printTUIMenu lists the current rooms (numbered 1-9, for the numbered-entry
// shortcut) followed by the fixed key legend.
func printTUIMenu(nicknames []string) {
	fmt.Println("devroom")
	if len(nicknames) == 0 {
		fmt.Println("  (no rooms yet)")
	}
	for i, nickname := range nicknames {
		if i >= 9 {
			fmt.Printf("  ... and %d more (press 'e' to enter by name)\n", len(nicknames)-9)
			break
		}
		fmt.Printf("  %d) %s\n", i+1, nickname)
	}
	fmt.Println()
	fmt.Println("  n  Create a new room")
	fmt.Println("  1-9  Enter the listed room")
	fmt.Println("  e  Enter a room by name")
	fmt.Println("  l  List rooms")
	fmt.Println("  d  Show AI-generated description of each room")
	fmt.Println("  Q  Close a room (container deleted, image kept)")
	fmt.Println("  X  Destroy the base image")
	fmt.Println("  q  Quit")
	fmt.Print("> ")
}

// readKey reads a single raw keypress from stdin without waiting for Enter.
func readKey() (byte, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return 0, fmt.Errorf("entering raw terminal mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	buf := make([]byte, 1)
	if _, err := os.Stdin.Read(buf); err != nil {
		return 0, fmt.Errorf("reading key: %w", err)
	}
	return buf[0], nil
}

// promptLine reads a line of cooked-mode input, e.g. a nickname typed after
// a menu action. Terminal state is normal here since readKey always restores
// it before this is called.
func promptLine(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}
