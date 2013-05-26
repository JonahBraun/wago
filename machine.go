package main

import (
	"github.com/howeyc/fsnotify"
	"regexp"
)

type Action interface {
	// returns true if the command was started ok
	Run() bool
	Kill()
}

type Machine struct {
	Trans chan string
	chain []Action
	step  int
	watcher *fsnotify.Watcher
}

func NewMachine(watcher *fsnotify.Watcher) Machine {
	m := Machine{
		Trans: make(chan string),
		chain: make([]Action, 0),
		step:  0,
		watcher: watcher,
	}

	if len(*buildCmd) > 0 {
		m.chain = append(m.chain, NewRunWait(*buildCmd))
	}

	if len(*daemonCmd) > 0 {
		m.chain = append(m.chain, NewDaemon(*daemonCmd))
	}

	if len(*postCmd) > 0 {
		m.chain = append(m.chain, NewRunWait(*postCmd))
	}

	if *url != "" {
		m.chain = append(m.chain, NewBrowser(*url))
	}

	return m
}

func (m *Machine) action() {
	m.chain[m.step].Run()
}

func (m *Machine) begin() {
	Talk("Killing all processes")
	for i := range m.chain {
		m.chain[i].Kill()
	}

	Talk("Begin action chain")
	m.step = 0
	m.action()
}

func (m *Machine) RunHandler() {

	r, err := regexp.Compile(*watchRegex)
	if err != nil {
		Fatal("Watch regex compile error:", err)
	}

	for {
		select {
		case ev := <-m.watcher.Event:
			if r.MatchString(ev.String()) {
				Note("Matched event:", ev.String())
				m.begin()
			} else {
				Talk("Ignored event:", ev.String())
			}

		case err = <-m.watcher.Error:
			Fatal("Watcher error:", err)

		case <-m.Trans:
			if m.step == len(m.chain)-1 {
				Talk("Done!")
				break
			}
			m.step++
			m.action()
		}
	}
}
