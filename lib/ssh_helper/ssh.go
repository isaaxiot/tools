package ssh_helper

import (
	"github.com/Sirupsen/logrus"
	"github.com/hypersleep/easyssh"
	"github.com/xshellinc/tools/lib/help"
)

// Util is a ssh utility to scp run and stream commands/files
type Util interface {
	Scp(string, string) error
	Run(string) (string, string, error)
	Stream(string) (chan string, chan string, chan bool, error)
}

type config struct {
	SSH easyssh.MakeConfig

	Sudo     bool
	SudoPass string
	Timeout  int
	Retry    bool
	Verbose  bool
}

// New returns new config with default values
func New() Util {
	cf := config{}
	cf.Timeout = 30
	cf.Retry = true
	return &cf
}

// Scp file, directly with workaround to HOME `~` and then run `mv`
func (s *config) Scp(src string, dst string) error {
	err := s.SSH.Scp(src, dst)
	if err == nil {
		return nil
	}

	logrus.Error(err)

	fileName := help.FileName(src)

	if err := s.SSH.Scp(src, fileName); err != nil {
		return err
	}

	// @todo run scp

	return nil
}

// Run command over ssh
func (s *config) Run(command string) (string, string, error) {
	out, eut, t, err := s.SSH.Run(command, s.Timeout)

	if t && s.Retry {
		// retry
	}

	return out, eut, err
}

// Stream command
func (s *config) Stream(command string) (chan string, chan string, chan bool, error) {
	return s.SSH.Stream(command, s.Timeout)
}
