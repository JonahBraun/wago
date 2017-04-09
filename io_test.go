package main

import "testing"
import "github.com/stretchr/testify/assert"

type ReadWriter struct {
	//Reader func(p []byte) (n int, err error)
	//Writer func(p []byte) (n int, err error)
	//Closer func() error

	Pipe chan []byte
}

func NewReadWriter() *ReadWriter {
	return &ReadWriter{
		// Write() below sends to this channel. Multiple ReadWriters are written to
		// in the below test. Because the writes happen in a random order (in ManageUserInput()
		// range), and we receive in order, this can result in a block.
		// Buffer the channel to avoid the block.
		make(chan []byte, 1),
	}
}

func (rw *ReadWriter) Read(p []byte) (n int, err error) {
	input := <-rw.Pipe
	copy(p, input)
	return len(input), nil
}

func (rw *ReadWriter) Write(p []byte) (n int, err error) {
	rw.Pipe <- p
	return len(p), nil
}

func (rw *ReadWriter) Close() (err error) {
	return nil
}

// Add and remove multiple processes, input should always reach all subscribed
// processed.
func TestManageUserInput(t *testing.T) {
	tests := [][]byte{
		[]byte("foo bar"),
		[]byte("¿\t\n?"),
		[]byte("\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98"), // malformed utf-8
		[]byte("\x18\x1f\x00"),                     // control chars and null
	}

	mockUser := NewReadWriter()
	sub, unsub := ManageUserInput(mockUser)

	// Create and add first process
	cmdAMock := NewReadWriter()
	cmdA := &Cmd{
		Stdin: cmdAMock,
	}
	sub <- cmdA

	// Test one process
	for i := range tests {
		mockUser.Pipe <- tests[i]
		assert.Equal(t, tests[i], <-cmdAMock.Pipe)
	}

	// Add second process
	cmdBMock := NewReadWriter()
	cmdB := &Cmd{
		Stdin: cmdBMock,
	}
	sub <- cmdB

	// Test both processes. Note input is only sent once
	for i := range tests {
		mockUser.Pipe <- tests[i]
		assert.Equal(t, tests[i], <-cmdAMock.Pipe)
		assert.Equal(t, tests[i], <-cmdBMock.Pipe)
	}

	// Add third proccess
	cmdCMock := NewReadWriter()
	cmdC := &Cmd{
		Stdin: cmdCMock,
	}
	sub <- cmdC

	// Test all three.
	for i := range tests {
		mockUser.Pipe <- tests[i]
		assert.Equal(t, tests[i], <-cmdAMock.Pipe)
		assert.Equal(t, tests[i], <-cmdBMock.Pipe)
		assert.Equal(t, tests[i], <-cmdCMock.Pipe)
	}

	unsub <- cmdA
	// Close to check that no input is routed to the removed process
	close(cmdAMock.Pipe)

	for i := range tests {
		mockUser.Pipe <- tests[i]
		assert.Equal(t, tests[i], <-cmdBMock.Pipe)
		assert.Equal(t, tests[i], <-cmdCMock.Pipe)
	}

	unsub <- cmdB
	unsub <- cmdC
}
