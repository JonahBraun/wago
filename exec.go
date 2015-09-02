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
	kill   chan bool
	done   chan bool
}

func NewCmd(name string) *Cmd {
	return &Cmd{
		Name: name,
		// -c is the POSIX switch to run a command
		Cmd:    exec.Command(*shell, "-c", name),
		killed: false,
		done:   make(chan bool),
	}
}

func (cmd *Cmd) Kill() {
	// return if:
	// command was never created,
	// command failed to start (PID is nil)
	// the command has finished (ProcessState is set)
	if cmd == nil || cmd.Process == nil || cmd.ProcessState != nil {
		//if cmd == nil || cmd.ProcessState != nil {
		return
	}

	// set the killed flag so when it dies we know we killed it on purpose
	cmd.killed = true

	if *exitWait < 1 {
		log.Info("Killing ("+cmd.Name+"), pid", cmd.Process)
		if err := cmd.Process.Kill(); err != nil {
			log.Err("Failed to kill command ("+cmd.Name+"), error:", err)
		}
		return
	}

	log.Info("Sending exit signal (SIGINT) to command ("+cmd.Name+"), pid:", cmd.Process)

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		log.Err("Failed to kill command ("+cmd.Name+"):", err)
		return
	}

	// give the process time to cleanup, check again if it has finished (ProcessState is set)
	time.Sleep(time.Duration(*exitWait) * time.Millisecond) // 3ms
	if cmd.ProcessState != nil {
		return
	}

	log.Info("Command ("+cmd.Name+") still alive, killing pid", cmd.Process)
	if err := cmd.Process.Kill(); err != nil {
		log.Err("Failed to kill command ("+cmd.Name+"):", err)
	}
}

type RunWait struct {
	*Cmd
}

func NewRunWait(name string) *RunWait {
	return &RunWait{Cmd: NewCmd(name)}
}

func (cmd *RunWait) Run() {
	log.Info("Running command, waiting:", cmd.Name)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		log.Fatal("Error starting command:", err)(6)
	}

	err = cmd.Wait()
	if !cmd.killed && err != nil {
		log.Err("Command exit error:", err)
	}
	close(cmd.done)
}

type Daemon struct {
	*Cmd
}

func NewDaemon(name string) *Daemon {
	return &Daemon{Cmd: NewCmd(name)}
}

func (cmd *Daemon) Run() {
	if len(*daemonTrigger) > 0 {
		cmd.RunTrigger(*daemonTrigger)
	} else {
		cmd.RunTimer(*daemonTimer)
	}
}

func (cmd *Daemon) RunTimer(period int) {
	log.Info("Starting daemon:", cmd.Name)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		log.Fatal("Error starting daemon:", err)(7)
	}

	log.Debug("Waiting miliseconds:", period)
	timer := time.AfterFunc(time.Duration(period)*time.Millisecond, func() {
		// TODO try to pass just the func
		close(cmd.done)
	})

	err = cmd.Wait()
	timer.Stop()

	// TODO mv to generic if killed that says [command type] exit error:
	if !cmd.killed && err != nil {
		log.Err("Command exit error:", err)
		return
	}

	// A daemon probably shouldn't be exiting
	log.Warn("Daemon exited cleanly")
}

func (cmd *Daemon) RunTrigger(triggerString string) {
	log.Info("Starting daemon:", cmd.Name)

	cmd.Stdin = os.Stdin

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("Error opening stdout:", err)(8)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal("Error opening stderr:", err)(8)
	}

	err = cmd.Start()
	if err != nil {
		log.Fatal("Error starting daemon:", err)(7)
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
						close(cmd.done)
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

	go watchPipe(stdoutPipe, os.Stdout)
	go watchPipe(stderrPipe, os.Stderr)

	err = cmd.Wait()

	// TODO mv to generic if killed that says [command type] exit error:
	if !cmd.killed && err != nil {
		log.Err("Command exit error:", err)
		return
	}

	// A daemon probably shouldn't be exiting
	log.Warn("Daemon exited cleanly")
}
