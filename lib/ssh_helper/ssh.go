package ssh_helper

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/hypersleep/easyssh"
	"github.com/xshellinc/tools/lib/help"
	"golang.org/x/crypto/ssh"
)

const (
	readBufSz = 1024 * 512
)

// Util is a ssh utility to scp run and stream commands/files
type Util interface {
	SetTimer(int)
	Scp(string, string) error
	Run(string) (string, string, error)
	Stream(string) (chan string, chan string, chan bool, error)
	ScpFromServer(string, string) error
}

type config struct {
	SSH easyssh.MakeConfig

	Sudo     bool
	SudoPass string

	timer   int
	timeout int

	retry   bool
	verbose bool
}

// New returns new config with default values
func New(ip, user, pass, port string) Util {
	cf := config{}

	cf.SSH.Server = ip
	cf.SSH.User = user
	cf.SSH.Password = pass
	cf.SSH.Port = port

	cf.timer = 30
	cf.timeout = 30
	cf.retry = true
	return &cf
}

// SetTimer sets a timeout for the next command execution
func (s *config) SetTimer(timeout int) {
	s.timer = timeout
}

// SetTimeout sets a global timeout for command executions
func (s *config) SetTimeout(timeout int) {
	s.timeout = timeout
}

// Scp a file, directly to a destination with a workaround copying to HOME `~` and running `mv` to the destination
func (s *config) Scp(src string, dst string) error {
	fileName := help.FileName(src)

	err := s.SSH.Scp(src, help.AddPathSuffix(dst, fileName, "unix"))
	if err == nil {
		return nil
	}

	logrus.Error(err)

	if err := s.SSH.Scp(src, fileName); err != nil {
		return err
	}

	// @todo run scp

	return nil
}

// Run command over ssh
func (s *config) Run(command string) (string, string, error) {
	defer func() {
		s.timer = s.timeout
	}()
	out, eut, t, err := s.SSH.Run(command, s.timer)

	if t && s.retry {
		// retry
	}

	return out, eut, err
}

// Stream command
func (s *config) Stream(command string) (chan string, chan string, chan bool, error) {
	defer func() {
		s.timer = s.timeout
	}()
	return s.SSH.Stream(command, s.timer)
}

// ScpFromServer copies a file from remote server using readBufSz buffer
func (s *config) ScpFromServer(src, dst string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}

	clientConfig := &ssh.ClientConfig{
		User: s.SSH.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.SSH.Password),
		},
	}
	client, err := ssh.Dial("tcp", s.SSH.Server+":"+s.SSH.Port, clientConfig)
	if err != nil {
		return err
	}
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	outPipe, err := session.StdoutPipe()
	if err != nil {
		return err
	}

	// buffering
	sem := make(chan struct{})

	go func() {
		buf := make([]byte, readBufSz)
		defer close(sem)

		for {
			n, err := outPipe.Read(buf)
			if err != nil || n == 0 {
				break
			}

			f.Write(buf[:n])
		}
	}()

	if err := session.Run("cat " + src); err != nil {
		return err
	}

	<-sem

	return err
}
