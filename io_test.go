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
		make(chan []byte),
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

func TestManageUserInput(t *testing.T) {
	tests := [][]byte{
		[]byte("foobar"),
		[]byte(" "),
		[]byte("\n"),
	}

	mockUser := NewReadWriter()

	sub, unsub := ManageUserInput(mockUser)

	cmdAMock := NewReadWriter()
	cmdA := &Cmd{
		Stdin: cmdAMock,
	}

	sub <- cmdA

	for i := range tests {
		mockUser.Pipe <- tests[i]
		assert.Equal(t, tests[i], <-cmdAMock.Pipe)
	}

	unsub <- cmdA
}
