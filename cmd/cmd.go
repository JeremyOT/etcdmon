package cmd

import (
	"log"
	"os"
	"os/exec"
)

type Command struct {
	args []string
	quit chan struct{}
	cmd  *exec.Cmd
}

func New(args []string) *Command {
	return &Command{args: args, quit: make(chan struct{})}
}

func (c *Command) Run() (err error) {
	defer close(c.quit)
	c.cmd = &exec.Cmd{Path: c.args[0], Args: c.args, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	if err := c.cmd.Run(); err != nil {
		log.Println("Command", c.args, "exited with error", err)
		return err
	}
	return nil
}

func (c *Command) Start() {
	go c.Run()
}

func (c *Command) Signal(sig os.Signal) error {
	return c.cmd.Process.Signal(sig)
}

func (c *Command) Kill() (err error) {
	if err = c.cmd.Process.Kill(); err == nil {
		c.Wait()
	}
	return
}

func (c *Command) Wait() {
	<-c.quit
}
