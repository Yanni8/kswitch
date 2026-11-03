// Package eks fetches kubeconfigs of AWS EKS clusters through the AWS
// CLI, which kubectl needs for authenticating to them anyway.
package eks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Options select the AWS region, profile, and role to use.
type Options struct {
	Region  string // "" leaves the choice to the AWS CLI's configuration
	Profile string // AWS CLI profile, "" for the default
	RoleARN string // IAM role to assume when authenticating to the cluster
}

// cliArgs returns the global AWS CLI options for o.
func (o Options) cliArgs() []string {
	var args []string
	if o.Region != "" {
		args = append(args, "--region", o.Region)
	}
	if o.Profile != "" {
		args = append(args, "--profile", o.Profile)
	}
	return args
}

// ListClusters returns the names of the EKS clusters in the region.
func ListClusters(o Options) ([]string, error) {
	out, err := Run(append([]string{"eks", "list-clusters", "--output", "json"}, o.cliArgs()...)...)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Clusters []string `json:"clusters"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("unexpected output from aws eks list-clusters: %w", err)
	}
	sort.Strings(resp.Clusters)
	return resp.Clusters, nil
}

// Kubeconfig fetches the kubeconfig of cluster with "aws eks
// update-kubeconfig". Its context is named after the cluster, and it
// authenticates through the AWS CLI ("aws eks get-token"), remembering
// the profile when one is given.
func Kubeconfig(cluster string, o Options) ([]byte, error) {
	// The CLI writes a file; keep it in a private directory.
	dir, err := os.MkdirTemp("", "kubeswitch-eks-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "kubeconfig")

	args := []string{"eks", "update-kubeconfig", "--name", cluster, "--alias", cluster, "--kubeconfig", path}
	args = append(args, o.cliArgs()...)
	if o.RoleARN != "" {
		args = append(args, "--role-arn", o.RoleARN)
	}
	if _, err := Run(args...); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// Profiles returns the names of the configured AWS CLI profiles, in the
// order the CLI lists them.
func Profiles() ([]string, error) {
	out, err := Run("configure", "list-profiles")
	if err != nil {
		return nil, err
	}
	var profiles []string
	for _, line := range strings.Split(string(out), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			profiles = append(profiles, p)
		}
	}
	return profiles, nil
}

// ConfiguredRegion returns the region configured for profile ("" for the
// default profile), or "" when none is configured.
func ConfiguredRegion(profile string) (string, error) {
	args := []string{"configure", "get", "region"}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	out, err := Run(args...)
	if errors.Is(err, ErrNotInstalled) {
		return "", err
	}
	if err != nil {
		return "", nil // "aws configure get" fails when the value is not set
	}
	return strings.TrimSpace(string(out)), nil
}

// ErrNotInstalled is returned when the AWS CLI cannot be found.
var ErrNotInstalled = errors.New("the AWS CLI (aws) was not found in PATH; install it from https://aws.amazon.com/cli/")

// Run executes the AWS CLI with args and returns its standard output.
// It is a variable so tests can substitute a fake.
var Run = runCLI

func runCLI(args ...string) ([]byte, error) {
	if _, err := exec.LookPath("aws"); err != nil {
		return nil, ErrNotInstalled
	}
	cmd := exec.Command("aws", args...)
	cmd.Stdin = os.Stdin // the CLI may prompt, for example for an MFA code
	cmd.Env = append(os.Environ(), "AWS_PAGER=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s: %s", describe(args), msg)
	}
	return stdout.Bytes(), nil
}

// describe names a CLI invocation for error messages, e.g. "aws eks
// list-clusters".
func describe(args []string) string {
	if len(args) > 2 {
		args = args[:2]
	}
	return strings.TrimSpace("aws " + strings.Join(args, " "))
}
