// +build !darwin

package main

import (
	"fmt"
)

type Browser struct {
	*Cmd
	url string
}

func NewBrowser(url string) *Browser {
	return &Browser{url: url}
}

func (cmd *Browser) Run() {
	command := fmt.Sprintf("google-chrome \"%s\"", cmd.url)
	cmd.Cmd = NewCmd(command)

	log.Info("Opening url (OS agnostic, this may not work):", cmd.url)

	output, err := cmd.CombinedOutput()

	if !cmd.killed && err != nil {
		log.Err("Error opening URL (error, output):", err, string(output))
	}
	close(cmd.done)
}
