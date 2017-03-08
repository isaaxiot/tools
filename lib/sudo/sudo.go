package sudo

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
)

const (
	sudoBinary = "sudo"
	// prompt must be unambiguous!
	passwordPrompt = "___"
	readBufSz      = 8 * 1024
)

// Default sudo arguments
var sudoArgs = []string{"-S", "-p", passwordPrompt}

type PasswordCallback func(data interface{}) string

// Exec sudo script with provided password
func ExecWithPassword(password string, script ...string) ([]byte, []byte, error) {
	return Exec(func(_ interface{}) string { return password }, nil, script...)
}

// Exec sudo script with provided password callback function with supplied data for it
// returns stdOut, latest stdErr row and
func Exec(cb PasswordCallback, cbData interface{}, script ...string) ([]byte, []byte, error) {
	cmd := exec.Command(sudoBinary, append(sudoArgs, script...)...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}

	// init vars, bind buffers and readers
	var errOutput []byte
	cmdOutput := &bytes.Buffer{}

	cmd.Stdout = cmdOutput
	stderrBuf := bufio.NewReader(stderr)

	err = cmd.Start()
	if err != nil {
		return nil, nil, err
	}

	// execute the pass check
	sem := make(chan struct{})
	go func() {
		buf := make([]byte, readBufSz)
		defer close(sem)

		for {
			// read into the buffer from stdErr
			n, err := stderrBuf.Read(buf)
			if err != nil || n == 0 {
				return
			}
			// copy the error's output
			errOutput = make([]byte, n)
			copy(errOutput, buf[:n])

			// check if stdErr contains the promt
			if bytes.Equal(buf[n-len(passwordPrompt):n], []byte(passwordPrompt)) {

				pwd := cb(cbData)

				if _, err = fmt.Fprintln(stdin, pwd); err != nil {
					err := []byte(err.Error())
					errOutput = make([]byte, len(err))
					copy(errOutput, err)
					return
				}
			}
		}
	}()

	err = cmd.Wait()
	// wait for the routine's exit
	<-sem

	if err != nil {
		return cmdOutput.Bytes(), errOutput, err
	}

	return cmdOutput.Bytes(), errOutput, nil
}
