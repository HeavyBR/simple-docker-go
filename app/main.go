package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
)

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	if err := isolateFilesystem(command); err != nil {
		log.Printf("cannot isolate filesystem, error: %v", err)
		os.Exit(1)
	}

	if err := isolateProcess(); err != nil {
		log.Printf("cannot isolate process, error: %v", err)
		os.Exit(1)
	}

	// Run the command
	cmd := exec.Command(command, args...)
	pipeIO(cmd)

	var exitError *exec.ExitError
	if err := cmd.Run(); err != nil {
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}

		log.Printf("cannot run command %s, error: %v", command, err)
		os.Exit(1)
	}
}

func isolateFilesystem(command string) error {
	// Create temp dir
	temp, err := os.MkdirTemp("", "docker")
	if err != nil {
		return err
	}

	// Copy the command binary to temp dir
	if _, err := copy(command, temp+command); err != nil {
		return fmt.Errorf("cannot copy command %s to %s, error: %v", command, temp, err)
	}

	// Chroot to temp dir
	if err := syscall.Chroot(temp); err != nil {
		return fmt.Errorf("cannot chroot to %s, error: %v", temp, err)
	}

	return nil
}

// isolateResources isolates the resources of the process using cgroups
func isolateResources() error {
	return nil
}

func isolateProcess() error {
	// Unshare
	if err := syscall.Unshare(syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID); err != nil {
		return fmt.Errorf("cannot unshare, error: %v", err)
	}

	return nil
}

func pipeIO(cmd *exec.Cmd) {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
}

// Copy file from src to dst
func copy(src, dst string) (int64, error) {

	// Check if src exists
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	// Check if src is a regular file
	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	// Open src file
	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	// Create dst folder
	dstFolder := dst[0 : len(dst)-len(sourceFileStat.Name())]
	if err := os.MkdirAll(dstFolder, sourceFileStat.Mode()); err != nil {
		return 0, err
	}

	// Create destination file
	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()

	// Give destination file the same permission as src file
	if err := os.Chmod(dst, sourceFileStat.Mode()); err != nil {
		return 0, err
	}

	// Copy src to dst
	return io.Copy(destination, source)
}
