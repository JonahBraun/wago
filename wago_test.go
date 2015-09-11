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

var announceTest func(...interface{})

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
	announceTest = dog.CreateLog(dog.FgYellow, "")
	flag.Parse()

	// essential setup commands
	runtime.GOMAXPROCS(runtime.NumCPU())
	*shell = "/bin/sh"

	ManageStdin()

	os.Exit(m.Run())
}

func TestSimple(t *testing.T) {
	announceTest("TestSimple")

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

			watcher.Event <- FakeEvent(`"/tmp/fake.txt": CREATE`)
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

func TestEventRace(t *testing.T) {
	announceTest("TestEventRace")

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

			watcher.Event <- FakeEvent(`"/tmp/fake.txt": CREATE`)
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

func TestDaemon(t *testing.T) {
	announceTest("TestDaemon")

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

func TestDaemonTimer(t *testing.T) {
	announceTest("TestDaemonTimer")

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
