package cmd

import (
	"log"
	"os"
	"os/exec"
)

// Run the specified command piping Stdin, Stdout and Stderr to the parent process.
// Closes the quit channel on exit.
func RunCommand(args []string, quit chan int) {
	defer close(quit)
	cmd := &exec.Cmd{Path: args[0], Args: args, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	if err := cmd.Run(); err != nil {
		log.Println("Command", args, "exited with error", err)
	}
}
