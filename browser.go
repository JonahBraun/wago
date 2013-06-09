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

func (c *Browser) Run() bool {
	command :=fmt.Sprintf("google-chrome \"%s\"", c.url)
	c.Cmd = NewCmd(command)

	go func(cmd *Cmd) {
		Note("Opening url (OS agnostic, this may note work):", c.url)

		output, err := cmd.CombinedOutput()

		if cmd.killed {
			return
		}

		if err != nil {
			Err("Error opening URL (error, output):", err, string(output))
		}

		// finished successfully
		machine.Trans <- "next"
	}(c.Cmd)

	return true
}
