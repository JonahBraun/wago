package main

import (
	"os"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func NewFakeWatcher() *Watcher {
	return &Watcher{
		make(chan fsnotify.Event),
		make(chan error),
	}
}

func (watcher *Watcher) SendCreate() {
	watcher.Event <- fsnotify.Event{`"/tmp/fake.txt": CREATE`, fsnotify.Create}
}

func TestAppIntegrations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping application integration testing.")
	}

	// essential setup commands
	*shell = "/bin/sh"

	subStdin, unsubStdin = ManageUserInput(os.Stdin)

	t.Run("Simple", appSimple)
	t.Run("EventRace", appEventRace)
	t.Run("Daemon", appDaemon)
	t.Run("DaemonTimer", appDaemonTimer)
}

func appSimple(t *testing.T) {
	*buildCmd = "echo testsimple"

	watcher := NewFakeWatcher()

	quit := make(chan struct{})

	go func() {
		duration := time.Duration(1) * time.Second
		for {
			select {
			case <-quit:
				return
			default:
			}

			watcher.Event <- fsnotify.Event{`"/tmp/fake.txt": CREATE`, fsnotify.Create}
			time.Sleep(duration)
			duration += time.Second
		}
	}()

	go func() {
		time.Sleep(time.Duration(3 * time.Second))
		close(quit)
	}()

	runChain(watcher, quit)
	*buildCmd = ""
}

func appEventRace(t *testing.T) {
	*buildCmd = "echo echonow"

	watcher := NewFakeWatcher()

	quit := make(chan struct{})

	go func() {
		duration := time.Duration(1)
		for {
			select {
			case <-quit:
				return
			default:
			}

			watcher.Event <- fsnotify.Event{`"/tmp/fake.txt": CREATE`, fsnotify.Create}
			time.Sleep(duration)
			duration += 10
		}
	}()

	go func() {
		time.Sleep(time.Duration(1 * time.Second))
		close(quit)
	}()

	runChain(watcher, quit)
	*buildCmd = ""
}

func appDaemon(t *testing.T) {
	*daemonCmd = "sleep 1s && echo testdaemonOut1 && sleep 2s && echo testdaemonOut2"
	watcher := NewFakeWatcher()

	quit := make(chan struct{})
	go func() {
		time.Sleep(time.Duration(1 * time.Second))
		watcher.SendCreate()
		time.Sleep(time.Duration(2 * time.Second))
		watcher.SendCreate()
		time.Sleep(time.Duration(4 * time.Second))
		close(quit)
	}()

	runChain(watcher, quit)
	*daemonCmd = ""
}

func appDaemonTimer(t *testing.T) {
	*daemonCmd = "sleep 1s && echo testdaemontimerOut1 && sleep 2s && echo testdaemontimerOut2"
	*daemonTimer = 2 * int(time.Second)

	watcher := NewFakeWatcher()

	quit := make(chan struct{})
	go func() {
		time.Sleep(time.Duration(1 * time.Second))
		watcher.SendCreate()
		time.Sleep(time.Duration(4 * time.Second))
		watcher.SendCreate()
		time.Sleep(time.Duration(4 * time.Second))
		close(quit)
	}()

	runChain(watcher, quit)
	*daemonCmd = ""
	*daemonTimer = 0
}
