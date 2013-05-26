package main

import (
	"io"
	"os"
	"os/exec"
	"time"
)

type Cmd struct {
	Name string
	*exec.Cmd
	killed bool
}

func NewCmd(name string) *Cmd {
	return &Cmd{
		Name:   name,
		Cmd:    exec.Command("/bin/bash", "-c", name),
		killed: false,
	}
}

func (c *Cmd) Kill() {
	// return if the cmd has not been run or the command has finished (ProcessState is set)
	if c == nil || c.ProcessState != nil {
		return
	}

	Note("Killing command ("+c.Name+"), pid:", c.Process)
	c.killed = true

	if err := c.Process.Kill(); err != nil {
		Err("Failed to kill command ("+c.Name+"):", err)
	}
}

type RunWait struct {
	Name string
	*Cmd
}

func NewRunWait(name string) *RunWait {
	return &RunWait{Name: name}
}

func (c *RunWait) Run() bool {
	Note("Running command:", c.Name)

	c.Cmd = NewCmd(c.Name)

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err := c.Start()
	if err != nil {
		Err("Error running command:", err)
		return false
	}

	// watch process for exit
	go func(cmd *Cmd) {
		err := cmd.Wait()
		if cmd.killed {
			return
		}
		if err != nil {
			Err("Command exit error:", err)
			return
		}

		// finished successfully
		machine.Trans <- "next"
	}(c.Cmd)

	return true
}

type Daemon struct {
	Name string
	*Cmd
}

func NewDaemon(name string) *Daemon {
	return &Daemon{Name: name}
}

func (c *Daemon) Run() bool {
	c.Cmd = NewCmd(c.Name)

	if len(*daemonTrigger) > 0 {
		return c.RunTrigger(*daemonTrigger)
	} else {
		return c.RunTimer(*daemonTimer)
	}
}

func (c *Daemon) RunTimer(period int) bool {
	Note("Starting daemon:", c.Name)

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err := c.Start()
	if err != nil {
		Err("Error starting daemon:", err)
		return false
	}

	Talk("Waiting miliseconds:", period)

	timer := time.AfterFunc(time.Duration(period)*time.Millisecond, func() {
		machine.Trans <- "next"
	})

	// watch process for exit
	go func(cmd *Cmd) {
		err := cmd.Wait()
		timer.Stop()

		if cmd.killed {
			return
		}
		if err != nil {
			Err("Command exit error:", err)
			return
		}

		// A daemon probably shouldn't be exiting
		Warn("Daemon exited cleanly")
	}(c.Cmd)

	return true
}

func (c *Daemon) RunTrigger(triggerString string) bool {
	Note("Starting daemon:", c.Name)

	c.Stdin = os.Stdin

	trigger := make(chan bool)

	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		panic(err)
	}
	stderrPipe, err := c.StderrPipe()
	if err != nil {
		panic(err)
	}

	err = c.Start()
	if err != nil {
		Err("Error starting daemon:", err)
		return false
	}

	stop := false
	key := []byte(triggerString)

	watchPipe := func(in io.Reader, out io.Writer) {
		b := make([]byte, 1)
		spot := 0

		for {
			// check if the trigger has been pulled and shift to copy mode
			if stop {
				_, err := io.Copy(out, in)
				if err != nil {
					Err("Unwatched pipe has errored:", err)
				}
				machine.Trans <- "next"
				return
			}

			n, err := in.Read(b)
			if n > 0 {
				out.Write(b[:n])
				if b[0] == key[spot] {
					spot++
					if spot == len(key) {
						Talk("Trigger match")
						trigger <- true
						stop = true
					}
				}
			}
			if err != nil {
				if err.Error() != "EOF" {
					Err("Watched pipe error:", err)
				}
				trigger <- false
				return
			}
		}
	}

	go watchPipe(stdoutPipe, os.Stdout)
	go watchPipe(stderrPipe, os.Stderr)

	// watch process for exit
	go func(cmd *Cmd) {
		err := cmd.Wait()

		if cmd.killed {
			return
		}
		if err != nil {
			Err("Command exit error:", err)
			return
		}

		// A daemon probably shouldn't be exiting
		Warn("Daemon exited cleanly")
	}(c.Cmd)

	return true
}
