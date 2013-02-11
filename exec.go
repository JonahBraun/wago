package main

import (
	"os"
	"os/exec"
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
/*
func (c *Cmd) StartTimer(timer *int) bool {
	Talk("Running command: ", c.Name)
	Talk("Starting daemon: ", *daemonCmd)
	daemon = exec.Command("/bin/bash", "-c", *daemonCmd)

	daemon.Stdin = os.Stdin
	daemon.Stdout = os.Stdout
	daemon.Stderr = os.Stderr

	trigger := make(chan bool)

	err := daemon.Start()
	if err != nil {
		Err("Error starting daemon: ", err)
		return false
	}

	if *daemonTimer > 0 {
		Talk("Waiting for: ", *daemonTimer)

		go func() {
			time.Sleep(time.Duration(*daemonTimer) * time.Millisecond)
			trigger <- true
		}()
	}

	// wait for the tirgger
	ok := <-trigger

	// check if daemon is still running
	if daemon.ProcessState != nil {
		return false
	}

	return ok
}

func startDaemonWatch() bool {
	Talk("Starting daemon: ", *daemonCmd)
	daemon = exec.Command("/bin/bash", "-c", *daemonCmd)

	daemon.Stdin = os.Stdin

	trigger := make(chan bool)

	stdoutPipe, err := daemon.StdoutPipe()
	if err != nil {
		panic(err)
	}
	stderrPipe, err := daemon.StderrPipe()
	if err != nil {
		panic(err)
	}

	err = daemon.Start()
	if err != nil {
		Err("Error starting daemon: ", err)
		return false
	}

	stop := false
	key := []byte(*daemonTrigger)

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
		err = daemon.Wait()
		if err != nil {
			Err(err)
		}
		trigger <- false
	}()

	// wait for the tirgger
	ok := <-trigger

	// check if daemon is still running
	if daemon.ProcessState != nil {
		return false
	}

	return ok
}
*/
