package main

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type Cmd struct {
	*exec.Cmd
	done chan bool
	dead chan struct{}
}

func NewCmd(command string) *Cmd {
	return &Cmd{
		// -c is the POSIX switch to run a command
		Cmd:  exec.Command(*shell, "-c", command),
		done: make(chan bool),
		dead: make(chan struct{}),
	}
}

type Runnable func(chan struct{}) (chan bool, chan struct{})

func NewRunWait(command string) Runnable {
	return func(kill chan struct{}) (chan bool, chan struct{}) {
		log.Info("Running command, waiting:", command)

		cmd := NewCmd(command)

		go cmd.RunWait(kill)

		return cmd.done, cmd.dead
	}
}

func (cmd *Cmd) RunWait(kill chan struct{}) {
	defer close(cmd.done)
	defer close(cmd.dead)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		// this is a program, system or environment error (shell is set wrong)
		// because it is not recoverable between builds, it is fatal
		log.Fatal("Error starting command:", err)(6)
	}

	proc := make(chan error)
	go func() {
		proc <- cmd.Wait()
		close(proc)
	}()

	select {
	case err := <-proc:
		if err != nil {
			log.Err("Command error:", err)
			cmd.done <- false
		} else {
			cmd.done <- true
		}
	case <-kill:
		cmd.Kill()
	}

	// we can not return until the process has exited
	// while this is guarunteed in a RunWait(), it is necessary for daemons
	// and so standardized here as well
	<-proc
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

	if *exitWait < 1 {
		log.Info("Killing ("+cmd.Path+"), pid", cmd.Process)
		if err := cmd.Process.Kill(); err != nil {
			log.Err("Failed to kill command ("+cmd.Path+"), error:", err)
		}
		return
	}

	log.Info("Sending exit signal (SIGINT) to command ("+cmd.Path+"), pid:", cmd.Process)

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		log.Err("Failed to kill command ("+cmd.Path+"):", err)
		return
	}

	// give the process time to cleanup, check again if it has finished (ProcessState is set)
	time.Sleep(time.Duration(*exitWait) * time.Millisecond)
	if cmd.ProcessState != nil {
		return
	}

	log.Info("Command ("+cmd.Path+") still alive, killing pid", cmd.Process)
	if err := cmd.Process.Kill(); err != nil {
		log.Err("Failed to kill command ("+cmd.Path+"):", err)
	}
}

func NewDaemonTimer(command string, period int) Runnable {
	return func(kill chan struct{}) (chan bool, chan struct{}) {
		log.Info("Starting daemon:", command)

		cmd := NewCmd(command)

		go cmd.RunDaemonTimer(kill, period)

		return cmd.done, cmd.dead
	}
}
func (cmd *Cmd) RunDaemonTimer(kill chan struct{}, period int) {
	defer close(cmd.done)
	defer close(cmd.dead)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		log.Fatal("Error starting daemon:", err)(7)
	}

	proc := make(chan error)
	go func() {
		proc <- cmd.Wait()
		close(proc)
	}()

	log.Debug("Waiting miliseconds:", period)
	timerDone := make(chan struct{})
	timer := time.AfterFunc(time.Duration(period)*time.Millisecond, func() {
		close(timerDone)
	})

	select {
	case <-timerDone:
		log.Debug("Daemon timer done")
		cmd.done <- true
	case err := <-proc:
		timer.Stop()
		if err != nil {
			log.Err("Daemon error:", err)
			cmd.done <- false
		} else {
			// A daemon probably shouldn't be exiting
			log.Warn("Daemon exited cleanly")
			cmd.done <- true
		}
	case <-kill:
		cmd.Kill()
	}

	// we can not return until the process has exited
	<-proc
}

func NewDaemonTrigger(command string, trigger string) Runnable {
	return func(kill chan struct{}) (chan bool, chan struct{}) {
		log.Info("Starting daemon:", command)

		cmd := NewCmd(command)

		go cmd.RunDaemonTrigger(kill, trigger)

		return cmd.done, cmd.dead
	}
}

func (cmd *Cmd) RunDaemonTrigger(kill chan struct{}, trigger string) {
	defer close(cmd.done)
	defer close(cmd.dead)

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

	proc := make(chan error)
	go func() {
		proc <- cmd.Wait()
		close(proc)
	}()

	stop := false
	key := []byte(trigger)
	match := make(chan struct{})

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
						close(match)
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

	select {
	case <-match:
		log.Debug("Daemon trigger matched")
		cmd.done <- true
	case err := <-proc:
		if err != nil {
			log.Err("Daemon error:", err)
			cmd.done <- false
		} else {
			// A daemon probably shouldn't be exiting
			log.Warn("Daemon exited cleanly")
			cmd.done <- true
		}
	case <-kill:
		cmd.Kill()
	}

	// we can not return until the process has exited
	<-proc
}
