package main

import "os"

var StdinListeners = make(map[*Cmd]bool, 2)
var subStdin = make(chan *Cmd)
var unsubStdin = make(chan *Cmd)

func ManageStdin() {
	termIn := make(chan []byte)
	ready := make(chan struct{})
	// endlessly read from terminal stdin
	go func() {
		p := make([]byte, 1)
		var err error

		for {
			_, err = os.Stdin.Read(p)
			if err != nil {
				if err.Error() == "EOF" {
					log.Warn("EOF reading stdin, input will not be connected to processes.")
					log.Warn("You should only see this if you are running go test!")
					return
				} else {
					log.Fatal("Error reading stdin.", err)(10)
				}
			}
			termIn <- p
			// wait for the write to occur, otherwise there is a race condition
			// TODO: why is there a race when seperate fd's are being read() and write()?
			<-ready
		}
	}()

	go func() {
		var c *Cmd
		p := make([]byte, 1)
		var err error

		for {
			select {
			case p = <-termIn:
				for c = range StdinListeners {
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
			case c = <-subStdin:
				StdinListeners[c] = true
			case c = <-unsubStdin:
				delete(StdinListeners, c)
			}
		}
	}()
}
