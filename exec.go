package main

import (
	"io"
	"os"
	"os/exec"
	"time"
)

type Cmd struct{
	*exec.Cmd
	Name string
	killed bool
}

func NewCmd(name string) *Cmd {
	return &Cmd{exec.Command("/bin/bash", "-c", name), name, false}
}

func (c *Cmd) Kill() {
	Talk("Killing command ("+c.Name+"), pid: ", c.Process)
	c.killed = true

	if err := c.Process.Kill(); err != nil {
		Err("Failed to kill command ("+c.Name+"): ", err)
	}
}

func (c *Cmd) Run() bool {
	Talk("Running command: ", c.Name)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err := c.Cmd.Run()

	if err != nil {
		if !c.killed {
			Err("Error running command: ", err)
		}
		return false
	}

	return true
}

func (c *Cmd) RunTimer(timer int) bool {
	Talk("Starting daemon: ", c.Name)

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	trigger := make(chan bool)

	err := c.Start()
	if err != nil {
		Err("Error starting daemon: ", err)
		return false
	}

	Talk("Waiting miliseconds: ", timer)

	go func() {
		time.Sleep(time.Duration(timer) * time.Millisecond)
		trigger <- true
	}()
	
	// watch process for exit
	go func() {
		err = c.Wait()
		if err != nil {
			Err(err)
		}
		trigger <- false
	}()

	// wait for the trigger
	ok := <-trigger

	// check if daemon is still running
	if c.ProcessState != nil {
		return false
	}

	return ok
}

func (c *Cmd) RunTrigger(triggerString string) bool {
	Talk("Starting daemon: ", c.Name)

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
		Err("Error starting daemon: ", err)
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
				Note("Stopping trigger watch")
				_, err := io.Copy(out, in)
				if err != nil {
					Err("Unwatched pipe has errored: ", err)
				}
				return
			}

			n, err := in.Read(b)
			if n > 0 {
				out.Write(b[:n])
				if b[0] == key[spot] {
					spot++
					if spot == len(key) {
						Talk("Trigger matched!")
						trigger <- true
						stop = true
					}
				}
			}
			if err != nil {
				if err.Error() != "EOF" {
					Err("Watched pipe error: ", err)
				}
				trigger <- false
				return
			}
		}
	}

	go watchPipe(stdoutPipe, os.Stdout)
	go watchPipe(stderrPipe, os.Stderr)

	// watch process for exit
	go func() {
		err = c.Wait()
		if err != nil {
			Err(err)
		}
		trigger <- false
	}()

	// wait for the trigger
	ok := <-trigger

	// check if daemon is still running
	if c.ProcessState != nil {
		return false
	}

	return ok
}
