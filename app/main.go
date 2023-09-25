package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"
)

var client = &http.Client{
	Timeout: 10 * time.Second,
}

type ManifestResponse struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Size      int    `json:"size"`
		Digest    string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		MediaType string `json:"mediaType"`
		Size      int    `json:"size"`
		Digest    string `json:"digest"`
	} `json:"layers"`
}

type TokenResponse struct {
	Token       string    `json:"token"`
	AccessToken string    `json:"access_token"`
	ExpiresIn   int       `json:"expires_in"`
	IssuedAt    time.Time `json:"issued_at"`
}

const (
	getTokenURL    = "https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull"
	getManifestURL = "https://registry-1.docker.io/v2/library/%s/manifests/%s"
	getLayerURL    = "https://registry-1.docker.io/v2/library/%s/blobs/%s"

	contentTypeHeader = "application/vnd.docker.distribution.manifest.v2+json"
)

func getManifest(image, token string) (ManifestResponse, error) {
	var manifestResponse ManifestResponse
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(getManifestURL, image, "latest"), nil)
	if err != nil {
		return manifestResponse, err
	}

	req.Header.Set("Accept", contentTypeHeader)

	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	res, err := client.Do(req)
	if err != nil {
		return manifestResponse, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return manifestResponse, fmt.Errorf("cannot get manifestResponse, status: %s", res.Status)
	}

	if err := json.NewDecoder(res.Body).Decode(&manifestResponse); err != nil {
		return manifestResponse, err
	}

	return manifestResponse, nil
}

func getToken(image string) (TokenResponse, error) {
	var tokenResponse TokenResponse

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(getTokenURL, image), nil)
	if err != nil {
		return tokenResponse, err
	}

	res, err := client.Do(req)
	if err != nil {
		return tokenResponse, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return tokenResponse, fmt.Errorf("cannot get tokenResponse, status: %s", res.Status)
	}

	if err := json.NewDecoder(res.Body).Decode(&tokenResponse); err != nil {
		return tokenResponse, err
	}

	return tokenResponse, nil
}

func downloadLayer(image, token, digest string) error {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(getLayerURL, image, digest), nil)
	if err != nil {
		return err
	}

	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("cannot get layer, status: %s", res.Status)

	}

	// save the layer to a file
	out, err := os.Create(fmt.Sprintf("%s.tar.gz", digest[7:]))
	if err != nil {
		return err
	}

	defer out.Close()

	if _, err = io.Copy(out, res.Body); err != nil {
		return err
	}

	return nil
}

func extractLayer(src, dest string) error {
	cmd := exec.Command("tar", "-xvzf", src, "-C", dest)
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	imageName := os.Args[2]

	// Create temp dir
	temp, err := os.MkdirTemp("", "docker")
	if err != nil {
		log.Printf("cannot create temp dir, error: %v", err)
		os.Exit(1)
	}

	// isolateFilesystem will isolate the filesystem with chroot
	if err := isolateFilesystem(temp, command); err != nil {
		log.Printf("cannot isolate filesystem, error: %v", err)
		os.Exit(1)
	}

	// Get token
	tokenResponse, err := getToken(imageName)
	if err != nil {
		log.Printf("cannot get token, error: %v", err)
		os.Exit(1)
	}

	// Get manifest
	manifestResponse, err := getManifest(imageName, tokenResponse.Token)
	if err != nil {
		log.Printf("cannot get manifest, error: %v", err)
		os.Exit(1)
	}

	// Download first layer
	if err := downloadLayer(imageName, tokenResponse.Token, manifestResponse.Layers[0].Digest); err != nil {
		log.Printf("cannot download layer, error: %v", err)
		os.Exit(1)
	}

	// Extract first layer
	if err := extractLayer(fmt.Sprintf("%s.tar.gz", manifestResponse.Layers[0].Digest[7:]), temp); err != nil {
		log.Printf("cannot extract layer, error: %v", err)
		os.Exit(1)
	}

	// isolateProcess will isolate the process with unshare
	if err := isolateProcess(); err != nil {
		log.Printf("cannot isolate process, error: %v", err)
		os.Exit(1)
	}

	// TODO isolate resources with cgroups

	// TODO should I isolate network?

	// TODO should I isolate users?

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

func isolateFilesystem(dir, command string) error {
	// Copy the command binary to temp dir
	if _, err := copy(command, dir+command); err != nil {
		return fmt.Errorf("cannot copy command %s to %s, error: %v", command, dir, err)
	}

	// Chroot to temp dir
	if err := syscall.Chroot(dir); err != nil {
		return fmt.Errorf("cannot chroot to %s, error: %v", dir, err)
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
