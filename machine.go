package main

type Action interface {
	// returns true if the command was started ok
	Run() bool
	Kill()
}

type Machine struct {
	trans chan string
	chain []Action
	step  int
}

func NewMachine() Machine {
	m := Machine{
		trans: make(chan string),
		chain: make([]Action, 0),
		step:  0,
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

// TODO just send to the channel everywhere instead of calling this func
func (m *Machine) Trans(e string) {
	m.trans <- e
}

func (m *Machine) action() {
	m.chain[m.step].Run()
}

func (m *Machine) RunHandler() {
	for {
		t := <-m.trans

		switch t {
		case "begin":
			Talk("Killing all processes")
			for i := range m.chain {
				m.chain[i].Kill()
			}

			Talk("Begin action chain")
			m.step = 0
			m.action()

		case "next":
			if m.step == len(m.chain)-1 {
				Talk("Done!")
				break
			}
			m.step++
			m.action()

		}
	}
}
