package main

import (
	"bufio"
	"fmt"
	"strings"
)

// Every interactive prompt takes the *bufio.Reader to read from, rather than
// wrapping os.Stdin itself. A bufio reader buffers ahead of the current line,
// so two independent readers on os.Stdin silently steal each other's input:
// for example, the TUI's reader swallowing the answer to a confirmation asked
// from inside a subcommand. Code reached from the TUI must therefore be handed
// the TUI's session reader, while a standalone subcommand creates a single
// reader for its whole invocation.

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
// action.
func promptLine(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return ""
	}
	return strings.TrimSpace(line)
}

// confirmYN prompts the user with a Y/n or y/N question. def is the default,
// which is also the answer at end of input.
func confirmYN(reader *bufio.Reader, prompt string, def bool) bool {
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
