package help

import (
	"os/exec"
	"strings"
)

func getArch() (string, error) {
	cmd := exec.Command("uname", "-m")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}
