package main

import (
	"io"
	"os"
	"os/exec"
	"syscall"
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

	// set the killed flag so when it dies we know we killed it on purpose
	c.killed = true

	if *exitWait < 1 {
		log.Info("Killing ("+c.Name+"), pid", c.Process)
		if err := c.Process.Kill(); err != nil {
			log.Err("Failed to kill command ("+c.Name+"), error:", err)
		}
		return
	}

	log.Info("Sending exit signal (SIGINT) to command ("+c.Name+"), pid:", c.Process)

	if err := c.Process.Signal(syscall.SIGINT); err != nil {
		log.Err("Failed to kill command ("+c.Name+"):", err)
		return
	}

	// give the process time to cleanup, check again if it has finished (ProcessState is set)
	time.Sleep(time.Duration(*exitWait) * time.Millisecond) // 3ms
	if c.ProcessState != nil {
		return
	}

	log.Info("Command ("+c.Name+") still alive, killing pid", c.Process)
	if err := c.Process.Kill(); err != nil {
		log.Err("Failed to kill command ("+c.Name+"):", err)
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
	log.Info("Running command:", c.Name)

	c.Cmd = NewCmd(c.Name)

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err := c.Start()
	if err != nil {
		log.Err("Error running command:", err)
		return false
	}

	// watch process for exit
	go func(cmd *Cmd) {
		err := cmd.Wait()
		if cmd.killed {
			return
		}
		if err != nil {
			log.Err("Command exit error:", err)
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
	log.Info("Starting daemon:", c.Name)

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err := c.Start()
	if err != nil {
		log.Err("Error starting daemon:", err)
		return false
	}

	log.Debug("Waiting miliseconds:", period)

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
			log.Err("Command exit error:", err)
			return
		}

		// A daemon probably shouldn't be exiting
		log.Warn("Daemon exited cleanly")
	}(c.Cmd)

	return true
}

func (c *Daemon) RunTrigger(triggerString string) bool {
	log.Info("Starting daemon:", c.Name)

	c.Stdin = os.Stdin

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
		log.Err("Error starting daemon:", err)
		return false
	}

	stop := false
	key := []byte(triggerString)

	watchPipe := func(in io.Reader, out io.Writer, key []byte) {
		b := make([]byte, 1)
		spot := 0

		for {
			// check if the trigger has been pulled and shift to copy mode
			if stop {
				_, err := io.Copy(out, in)
				if err != nil {
					log.Err("Unwatched pipe has errored:", err)
				}
				return
			}

			n, err := in.Read(b)
			if n > 0 {
				out.Write(b[:n])
				if b[0] == key[spot] {
					spot++
					if spot == len(key) {
						log.Debug("Trigger match")
						stop = true
						machine.Trans <- "next"
					}
				}
			}
			if err != nil {
				if err.Error() != "EOF" {
					log.Err("Watched pipe error:", err)
				}
				return
			}
		}
	}

	go watchPipe(stdoutPipe, os.Stdout, key)
	go watchPipe(stderrPipe, os.Stderr, key)

	// watch process for exit
	go func(cmd *Cmd) {
		err := cmd.Wait()

		if cmd.killed {
			return
		}
		if err != nil {
			log.Err("Command exit error:", err)
			return
		}

		// A daemon probably shouldn't be exiting
		log.Warn("Daemon exited cleanly")
	}(c.Cmd)

	return true
}
