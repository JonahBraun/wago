package main

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

/*
Runnable is a type of function that runs a process, one of the actions in the
action chain a user defines for Wago.

It takes a channel to receive a kill signal. When a file change occurs (or
program exit), a kill signal is sent to the Runnable by closing the channel.
Runnable then kills and cleans up the process.

It returns two channels:

- The first channel will send a signal if the runnable is "done", that the
  chain can start the next action. This might mean the process has finished
  or that has finished starting (as in the case of a daemon). The boolean
  represents success, if it is false, the chain should abort.

- The second channel indicates by closing that the process has exited
  completely. All processes must be completely exited to ensure resources
  have been properly freed and the action chain can be safely started again.

New Runnables are created with one of the three constructors:
	NewRunWait, NewDaemonTimer, NewDaemonTrigger.
*/
type Runnable func(chan struct{}) (chan bool, chan struct{})

// Cmd extends exec.Cmd to include channels and i/o necessary for advanced
// process management.
type Cmd struct {
	*exec.Cmd
	Name string
	done chan bool
	dead chan struct{}

	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

// newCmd is a constructor for Runnables to set up their internal exec.Cmd along
// with channels to manage state and i/o pipes.
func newCmd(command string) *Cmd {
	cmd := &Cmd{
		// -c is the POSIX switch for a shell to run a command
		Cmd:  exec.Command(*shell, "-c", command),
		Name: command,

		// These channels will only be used once.
		// done is buffered so that the send can always succeed and the Runnable can
		// proceed to cleanup.
		done: make(chan bool, 1),
		// dead sends by closing the channel and so does not need buffering.
		dead: make(chan struct{}),
	}

	// Processes are set as a process group leader in a new process group. If it
	// creates any child processes, they will also belong to the new group and
	// allows us to kill all processes when necessary.
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

// kill nicely kills a process with escalating signals. This can only be called
// after a process has actually been started and so is only called internally
// by Runnables.
func (cmd *Cmd) kill(proc chan error) {
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

	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		log.Warn("Failed to send SIGTERM, command must have exited (name, error):", cmd.Name, err)
		return
	}

	// Give process time to exit…
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
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		if err.Error() == "no such process" {
			log.Info("Process exited before SIGKILL:", cmd.Name)
		} else {
			log.Err("Error killing command (cmd, error):", cmd.Name, err)
		}
	}
}

// NewRunWait constructs the Runnable RunWait.
func NewRunWait(command string) Runnable {
	return func(kill chan struct{}) (chan bool, chan struct{}) {
		log.Info("Running command, waiting:", command)

		cmd := newCmd(command)

		go cmd.RunWait(kill)

		return cmd.done, cmd.dead
	}
}

// RunWait starts a process and signals done after the process has exited.
//
// This is appropriate for actions like build commands which must complete successfully
// for the action chain to continue.
func (cmd *Cmd) RunWait(kill chan struct{}) {
	defer close(cmd.done)
	defer close(cmd.dead)

	err := cmd.Start()
	if err != nil {
		// This error is a program, system or environment error (shell is set wrong).
		// Because it is not recoverable between builds, it is fatal. The user needs
		// to adjust their system or command invocation.
		log.Fatal("Error starting command:", err)(6)
	}

	// The active process is now managed concurrently with signal management (below).
	// proc signals the process exit by closing.
	proc := make(chan error)
	go func() {
		var wg sync.WaitGroup

		// Subscribe to stdin, allows the process to receive input from the user.
		subStdin <- cmd
		copyPipe(cmd.Stdout, os.Stdout, &wg)
		copyPipe(cmd.Stderr, os.Stderr, &wg)

		// Wait for both copyPipes to finish. They will exit when the process has exited.
		wg.Wait()

		unsubStdin <- cmd

		proc <- cmd.Wait()
		close(proc)
	}()

	// We wait now for either the process to exit or a kill request. If the process
	// exits, we return success status so that action chain can conditionally continue.
	// If we receive a kill signal, the exit status no longer matters and isn't tracked.
	select {
	case err := <-proc:
		if err != nil {
			log.Err("Command error:", err)
			cmd.done <- false
		} else {
			cmd.done <- true
		}
	case <-kill:
		cmd.kill(proc)
	}

	// Runnables must not return until the process has exited completely.
	// TODO: All three Runnables end with <-proc however a code read suggests it is no
	// longer necessary. At this point either the process has exited or been killed.
	<-proc
}

// NewDaemonTimer constructs the Runnable RunDaemonTimer.
func NewDaemonTimer(command string, period int) Runnable {
	return func(kill chan struct{}) (chan bool, chan struct{}) {
		log.Info("Starting daemon:", command)

		cmd := newCmd(command)

		go cmd.RunDaemonTimer(kill, period)

		return cmd.done, cmd.dead
	}
}

// RunDaemonTimer starts a daemon, waits period milliseconds, then signals done
// for the action chain to continue.
//
// This is used for daemons without a timer, period is set to 0.
//
// Because Wago will warn the user when a daemon has exited (even with success)
// this can be used for for regular commands that do not have any output as the
// user will be told when it has completed.
func (cmd *Cmd) RunDaemonTimer(kill chan struct{}, period int) {
	defer close(cmd.done)
	defer close(cmd.dead)

	err := cmd.Start()
	if err != nil {
		// This error is a program, system or environment error (shell is set wrong).
		// Because it is not recoverable between builds, it is fatal. The user needs
		// to adjust their system or command invocation.
		log.Fatal("Error starting daemon:", err)(7)
	}

	// The active process is now managed concurrently with signal management (below).
	// proc signals the process exit by closing.
	proc := make(chan error)
	go func() {
		var wg sync.WaitGroup

		// Subscribe to stdin, allows the process to receive input from the user.
		subStdin <- cmd
		copyPipe(cmd.Stdout, os.Stdout, &wg)
		copyPipe(cmd.Stderr, os.Stderr, &wg)
		wg.Wait()

		unsubStdin <- cmd

		proc <- cmd.Wait()
		close(proc)
	}()

	// timerDone signals by closing.
	timerDone := make(chan struct{})

	log.Debug("Waiting milliseconds:", period)
	timer := time.AfterFunc(time.Duration(period)*time.Millisecond, func() {
		close(timerDone)
	})

	// Signal management.
	select {
	case <-timerDone:
		log.Debug("Daemon timer done")
		cmd.done <- true

		// Timer is done, but we still need to wait for an exit/kill. This nested
		// select duplicates the two cases of the parent select.
		select {
		case err := <-proc:
			if err != nil {
				log.Err("Daemon error:", err)
				cmd.done <- false
			} else {
				// A daemon probably shouldn't be exiting, warn the user.
				log.Warn("Daemon exited cleanly")
				cmd.done <- true
			}
		case <-kill:
			cmd.kill(proc)
		}

	case err := <-proc:
		timer.Stop()
		if err != nil {
			log.Err("Daemon error:", err)
			cmd.done <- false
		} else {
			log.Warn("Daemon exited cleanly")
			cmd.done <- true
		}
	case <-kill:
		timer.Stop()
		cmd.kill(proc)
	}

	// Runnables must not return until the process has exited completely.
	// TODO: All three Runnables end with <-proc however a code read suggests it is no
	// longer necessary. At this point either the process has exited or been killed.
	<-proc
}

// NewDaemonTrigger constructs the Runnable RunDaemonTrigger.
func NewDaemonTrigger(command string, trigger string) Runnable {
	return func(kill chan struct{}) (chan bool, chan struct{}) {
		log.Info("Starting daemon:", command)

		cmd := newCmd(command)

		go cmd.RunDaemonTrigger(kill, trigger)

		return cmd.done, cmd.dead
	}
}

// RunDaemonTrigger starts a daemon, waits until trigger is emitted by the process
// stdout or stderr, then signals done for the action chain to continue.
//
// This is useful for running daemons that have some setup and then output a ready
// status like "Listening on port…"
func (cmd *Cmd) RunDaemonTrigger(kill chan struct{}, trigger string) {
	defer close(cmd.done)
	defer close(cmd.dead)

	err := cmd.Start()
	if err != nil {
		log.Fatal("Error starting daemon:", err)(7)
	}

	key := []byte(trigger)
	match := make(chan struct{})

	// watchPipe watches an output stream for the trigger text. When the trigger
	// text is seen, it stops checking and simply copies.
	//
	// TODO: Consider abstracting further and moving to io.go.
	// TODO: This reads one byte at a time. Intead, we should fill a buffer, similar
	// to what ManageUserInput does.
	watchPipe := func(in io.Reader, out io.Writer) {
		b := make([]byte, 1)
		spot := 0

		for {
			// Check for a trigger match
			select {
			case <-match:
				// If there is a match, call io.Copy which will simply copy output to stdout
				// until the process exits.
				_, err := io.Copy(out, in)
				if err != nil {
					log.Err("Unwatched pipe has errored:", err)
				}
				return
			default:
				// If there is no match, fallthrough and check the next byte. This paradigm
				// is necessary as len(chan) only works when chan is buffered.
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

	// The active process is now managed concurrently with signal management (below).
	// proc signals the process exit by closing.
	proc := make(chan error)
	go func() {
		var wg sync.WaitGroup

		// Subscribe to stdin, allows the process to receive input from the user.
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

	// Signal management.
	select {
	case <-match:
		log.Debug("Daemon trigger matched")
		cmd.done <- true

		// Trigger is matched and we have signaled done, but we still need to wait for
		// an exit/kill. This nested select duplicates the two cases of the parent select.
		select {
		case err := <-proc:
			if err != nil {
				log.Err("Daemon error:", err)
				cmd.done <- false
			} else {
				// A daemon probably shouldn't be exiting, warn the user.
				log.Warn("Daemon exited cleanly")
				cmd.done <- true
			}
		case <-kill:
			cmd.kill(proc)
		}

	case err := <-proc:
		if err != nil {
			log.Err("Daemon error:", err)
			cmd.done <- false
		} else {
			// A daemon probably shouldn't be exiting, warn the user.
			log.Warn("Daemon exited cleanly")
			cmd.done <- true
		}
	case <-kill:
		cmd.kill(proc)
	}

	// Runnables must not return until the process has exited completely.
	// TODO: All three Runnables end with <-proc however a code read suggests it is no
	// longer necessary. At this point either the process has exited or been killed.
	<-proc
}
