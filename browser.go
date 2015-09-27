// +build !darwin

package main

import (
	"fmt"
	"os/exec"
)

func NewBrowser(url string) Runnable {
	return func(kill chan struct{}, quit chan struct{}) (chan bool, chan struct{}, bool) {
		command := fmt.Sprintf("google-chrome \"%s\"", url)

		cmd := &Cmd{
			Cmd:  exec.Command(command),
			Name: url,
			done: make(chan bool, 1),
			dead: make(chan struct{}),
		}

		go cmd.RunBrowser()

		return cmd.done, cmd.dead, true
	}
}

func (cmd *Cmd) RunBrowser() {
	defer close(cmd.done)
	defer close(cmd.dead)

	log.Info("Opening url (OS agnostic, this may not work):", cmd.Name)

	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Err("Error opening URL (error, output):", err, string(output))
	}
}
