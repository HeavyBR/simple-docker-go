package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	// Create temp dir
	if err := exec.Command("mkdir", "-p", "/tmp/your_docker").Run(); err != nil {
		os.Exit(1)
	}

	// Copy the command binary to temp dir
	commandPath, err := exec.LookPath(command)
	if err != nil {
		os.Exit(1)
	}

	if err := exec.Command("cp", commandPath, fmt.Sprintf("/tmp/your_docker/%s", command)).Run(); err != nil {
		os.Exit(1)
	}

	// CHRoot to temp dir
	if err := exec.Command("chroot", "/tmp/your_docker").Run(); err != nil {
		os.Exit(1)
	}

	cmd := exec.Command(command, args...)
	pipeIO(cmd)

	var exitError *exec.ExitError
	if err := cmd.Run(); err != nil {
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}

		os.Exit(1)
	}
}

func pipeIO(cmd *exec.Cmd) {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
}
