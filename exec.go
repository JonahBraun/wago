package main

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Cmd struct {
	*exec.Cmd
	Name string
	done chan bool
	dead chan struct{}

	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

func NewCmd(command string) *Cmd {
	cmd := &Cmd{
		// -c is the POSIX switch to run a command
		Cmd:  exec.Command(*shell, "-c", command),
		Name: command,
		// send on done must always succeed so the Runnable can proceed to cleanup
		done: make(chan bool, 1),
		dead: make(chan struct{}),
	}

	// give process a new process group so we can kill it's child processes (if any)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	var err error
	cmd.Stdin, err = cmd.StdinPipe()
	if err != nil {
		log.Fatal("Error making stdin (command, error):", cmd.Name, err)(9)
	}
	cmd.Stdout, err = cmd.StdoutPipe()
	if err != nil {
		log.Fatal("Error making stdout (command, error):", cmd.Name, err)(9)
	}
	cmd.Stderr, err = cmd.StderrPipe()
	if err != nil {
		log.Fatal("Error making stderr (command, error):", cmd.Name, err)(9)
	}

	return cmd
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

// we used to do
// cmd.Stdin = os.Stdin
// but that simple method does not work for child processes in process groups
func copyPipe(in io.Reader, out io.Writer, wg *sync.WaitGroup) {
	wg.Add(1)

	go func() {
		_, err := io.Copy(out, in)
		if err != nil {
			log.Err("I/O pipe has errored:", err)
		}
		wg.Done()
	}()
}

func (cmd *Cmd) RunWait(kill chan struct{}) {
	defer close(cmd.done)
	defer close(cmd.dead)

	err := cmd.Start()
	if err != nil {
		// this is a program, system or environment error (shell is set wrong)
		// because it is not recoverable between builds, it is fatal
		log.Fatal("Error starting command:", err)(6)
	}

	proc := make(chan error)
	go func() {
		var wg sync.WaitGroup

		subStdin <- cmd
		copyPipe(cmd.Stdout, os.Stdout, &wg)
		copyPipe(cmd.Stderr, os.Stderr, &wg)
		wg.Wait()

		unsubStdin <- cmd

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
	log.Info("Sending signal SIGTERM to command:", cmd.Name)

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		if err.Error() == "no such process" {
			log.Info("Process exited before SIGTERM:", cmd.Name)
		} else {
			log.Err("Error getting process group:", err)
		}
		return
	}

	if err := syscall.Kill(pgid, syscall.SIGTERM); err != nil {
		log.Warn("Failed to send SIGTERM, command must have exited (name, error):", cmd.Name, err)
		return
	}

	// give process time to exit…
	timerDone := make(chan struct{})
	timer := time.AfterFunc(time.Duration(*exitWait)*time.Millisecond, func() {
		close(timerDone)
	})

	select {
	case <-timerDone:
	case <-proc:
		timer.Stop()
		return
	}

	log.Info("After exitwait, command still running, sending SIGKILL…")
	if err := syscall.Kill(pgid, syscall.SIGKILL); err != nil {
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

	err := cmd.Start()
	if err != nil {
		log.Fatal("Error starting daemon:", err)(7)
	}

	proc := make(chan error)
	go func() {
		var wg sync.WaitGroup

		subStdin <- cmd
		copyPipe(cmd.Stdout, os.Stdout, &wg)
		copyPipe(cmd.Stderr, os.Stderr, &wg)
		wg.Wait()

		unsubStdin <- cmd

		proc <- cmd.Wait()
		close(proc)
	}()

	timerDone := make(chan struct{})

	log.Debug("Waiting miliseconds:", period)
	timer := time.AfterFunc(time.Duration(period)*time.Millisecond, func() {
		close(timerDone)
	})

	select {
	case <-timerDone:
		log.Debug("Daemon timer done")
		cmd.done <- true
		// timer is done, but we still need to wait for an exit/kill
		select {
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
		timer.Stop()
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

	err := cmd.Start()
	if err != nil {
		log.Fatal("Error starting daemon:", err)(7)
	}

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

	proc := make(chan error)
	go func() {
		var wg sync.WaitGroup

		subStdin <- cmd
		wg.Add(2)
		go func() {
			watchPipe(cmd.Stdout, os.Stdout)
			wg.Done()
		}()
		go func() {
			watchPipe(cmd.Stderr, os.Stderr)
			wg.Done()
		}()

		wg.Wait()
		unsubStdin <- cmd

		proc <- cmd.Wait()
		close(proc)
	}()

	select {
	case <-match:
		log.Debug("Daemon trigger matched")
		cmd.done <- true
		// still need to wait for proc to exit/kill
		select {
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
