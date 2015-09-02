package main

import (
	"flag"
	"fmt"
	"github.com/JonahBraun/dog"
	"os"
	"runtime"
	"testing"
	"time"
)

var announce func(...interface{})

type FakeEvent string

func (s FakeEvent) String() string {
	return string(s)
}

func NewFakeWatcher() *Watcher {
	return &Watcher{
		make(chan fmt.Stringer),
		make(chan error),
	}
}

func (watcher *Watcher) SendCreate() {
	watcher.Event <- FakeEvent(`"/tmp/fake.txt": CREATE`)
}

func TestMain(m *testing.M) {
	announce = dog.CreateLog(dog.FgYellow, "")
	flag.Parse()

	// essential setup commands
	runtime.GOMAXPROCS(runtime.NumCPU())
	*shell = "/bin/sh"

	os.Exit(m.Run())
}

func TestFsRace(t *testing.T) {
	announce("TestFsRace")

	*buildCmd = "sleep 1 && echo foo"

	watcher := NewFakeWatcher()

	go func() {
		duration := time.Duration(1)
		for {
			watcher.Event <- FakeEvent(`"/tmp/fake.txt": CREATE`)
			time.Sleep(duration)
			duration = duration * 2
		}
	}()

	machine = NewMachine(watcher)
	go machine.RunHandler()

	// should not take more than a second to hit a race if it exists
	// but we also want to see our buildCmd output
	time.Sleep(time.Duration(2 * time.Second))
}

func TestDaemon(t *testing.T) {
	announce("TestDaemon")

	*daemonCmd = "sleep 1s && echo foo && sleep 5s && echo done"
	watcher := NewFakeWatcher()

	go func() {
		time.Sleep(time.Duration(1 * time.Second))
		watcher.SendCreate()
		time.Sleep(time.Duration(2 * time.Second))
		watcher.SendCreate()
	}()

	machine = NewMachine(watcher)
	go machine.RunHandler()

	time.Sleep(time.Duration(8 * time.Second))
}

func TestDaemonTimer(t *testing.T) {
	announce("TestDaemonTimer")

	*daemonCmd = "sleep 1s && echo foo && sleep 5s && echo done"
	*daemonTimer = 2 * int(time.Second)

	watcher := NewFakeWatcher()

	go func() {
		time.Sleep(time.Duration(1 * time.Second))
		watcher.SendCreate()
		time.Sleep(time.Duration(4 * time.Second))
		watcher.SendCreate()
	}()

	machine = NewMachine(watcher)
	go machine.RunHandler()

	time.Sleep(time.Duration(8 * time.Second))
}
