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
	Name string
	done chan bool
	dead chan struct{}
}

func NewCmd(command string) *Cmd {
	return &Cmd{
		// -c is the POSIX switch to run a command
		Cmd:  exec.Command(*shell, "-c", command),
		Name: command,
		// send on done must always succeed so the Runnable can proceed to cleanup
		done: make(chan bool, 1),
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
	// to ensure we pick up this error, the first receive on proc must be done in this select
	case err := <-proc:
		if err != nil {
			log.Err("Command error:", err)
			cmd.done <- false
		} else {
			cmd.done <- true
		}
	case <-kill:
		cmd.Kill(proc)
	}

	// we can not return until the process has exited
	// while this is guarunteed in a RunWait(), it is necessary for daemons
	// and so standardized here as well
	<-proc
}

// This should only be called from within the Runnable
// which ensures that the process has started and so can be killed
func (cmd *Cmd) Kill(proc chan error) {
	if *exitWait < 1 {
		log.Info("Killing command:", cmd.Name)
		if err := cmd.Process.Kill(); err != nil {
			log.Err("Failed to kill command ("+cmd.Name+"), error:", err)
		}
		return
	}

	log.Info("Sending exit signal (SIGINT) to command:", cmd.Name)

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		log.Err("Failed to kill command ("+cmd.Name+"):", err)
		return
	}

	// give the process time to cleanup, check if process has exited
	time.Sleep(time.Duration(*exitWait) * time.Millisecond)
	select {
	case <-proc:
		return
	default:
	}

	log.Info("Command still alive, killing…")
	if err := cmd.Process.Kill(); err != nil {
		log.Err("Failed to kill command ("+cmd.Name+"):", err)
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

	timerDone := make(chan struct{})

	var timer *time.Timer
	log.Debug("Waiting miliseconds:", period)
	timer = time.AfterFunc(time.Duration(period)*time.Millisecond, func() {
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
		cmd.Kill(proc)
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

	key := []byte(trigger)
	match := make(chan struct{})

	watchPipe := func(in io.Reader, out io.Writer) {
		b := make([]byte, 1)
		spot := 0

		for {
			// check if the trigger has been pulled and shift to copy mode
			select {
			case <-match:
				_, err := io.Copy(out, in)
				if err != nil {
					log.Err("Unwatched pipe has errored:", err)
				}
				return
			default:
			}

			n, err := in.Read(b)
			if n > 0 {
				out.Write(b[:n])
				if b[0] == key[spot] {
					spot++
					if spot == len(key) {
						log.Debug("Trigger match")
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
		cmd.Kill(proc)
	}

	// we can not return until the process has exited
	<-proc
}
