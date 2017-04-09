package main

import "io"

// ManageUserInput passes bytes from user input to all actively running commands.
// Input is always os.Stdin
func ManageUserInput(input io.Reader) (sub chan *Cmd, unsub chan *Cmd) {
	termIn := make(chan []byte)
	ready := make(chan struct{})

	sub = make(chan *Cmd)
	unsub = make(chan *Cmd)

	// Index with the pointer so they can easily be removed.
	subscribers := make(map[*Cmd]struct{}, 2)

	// endlessly read from terminal stdin
	go func() {
		p := make([]byte, 0, 4*1024)

		for {
			// reslice p for Read() to use entire capacity
			n, err := input.Read(p[:cap(p)])
			// reslice p so we only have what was read
			p = p[:n]
			if n == 0 {
				if err == nil {
					continue
				}
				if err == io.EOF {
					log.Warn("Warning: input ended (EOF), no further input will be sent to processes.")
					return
				}
				log.Fatal(err)
			}

			if err != nil && err != io.EOF {
				log.Fatal("Error reading stdin.", err)(10)
			}

			termIn <- p
			// Slices are passed by reference, wait for the write so there is no race.
			<-ready
		}
	}()

	go func() {
		var c *Cmd
		var p []byte
		var err error

		for {
			select {
			case p = <-termIn:
				for c = range subscribers {
					_, err = c.Stdin.Write(p)
					if err != nil {
						// While researching this I came across
						// https://github.com/golang/go/issues/9307
						// https://github.com/golang/go/issues/9173
						// So we will prob just need to eat the error and keep going.
						log.Err("Error writing stdin (cmd, error):", c.Name, err)
					}
				}
				ready <- struct{}{}
			case c = <-sub:
				subscribers[c] = struct{}{}
			case c = <-unsub:
				delete(subscribers, c)
			}
		}
	}()

	return sub, unsub
}
