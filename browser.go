// +build !darwin

package main

import (
	"fmt"
)

func NewBrowser(url string) Runnable {
	return func(kill chan struct{}) (chan bool, chan struct{}) {
		command := fmt.Sprintf("google-chrome \"%s\"", url)
		cmd := NewCmd(command)

		go cmd.RunBrowser()

		return cmd.done, cmd.dead
	}
}

func (cmd *Cmd) RunBrowser() {
	defer close(cmd.done)
	defer close(cmd.dead)

	log.Info("Opening url (OS agnostic, this may not work):", cmd.url)

	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Err("Error opening URL (error, output):", err, string(output))
	}
}
