// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yanni8/kubeswitch/internal/eks"
)

// eksKubeconfig is what "aws eks update-kubeconfig --alias payments-prod
// --profile work" produces.
const eksKubeconfig = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTi0tLS0t
    server: https://ABCDEF.gr7.eu-central-1.eks.amazonaws.com
  name: arn:aws:eks:eu-central-1:123456789012:cluster/payments-prod
contexts:
- context:
    cluster: arn:aws:eks:eu-central-1:123456789012:cluster/payments-prod
    user: arn:aws:eks:eu-central-1:123456789012:cluster/payments-prod
  name: payments-prod
current-context: payments-prod
kind: Config
preferences: {}
users:
- name: arn:aws:eks:eu-central-1:123456789012:cluster/payments-prod
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      args:
      - --region
      - eu-central-1
      - eks
      - get-token
      - --cluster-name
      - payments-prod
      - --output
      - json
      command: aws
      env:
      - name: AWS_PROFILE
        value: work
`

// fakeAWS stands in for the AWS CLI.
type fakeAWS struct {
	profiles []string // answered to "configure list-profiles"
	region   string   // answered to "configure get region"; "" means not configured
	clusters []string // answered to "eks list-clusters"
	kubecfg  string   // written by "eks update-kubeconfig"
	calls    [][]string
}

// call returns the arguments of the first recorded call of the given
// subcommand (e.g. "update-kubeconfig"), or "" if it was never made.
func (f *fakeAWS) call(sub string) string {
	for _, c := range f.calls {
		if len(c) > 1 && c[1] == sub {
			return strings.Join(c, " ")
		}
	}
	return ""
}

// stubAWS installs f as the AWS CLI, clears the AWS environment so only
// f's configuration counts, and points the store at a temp dir. Unless
// f says otherwise, a "default" profile with a region is configured.
func stubAWS(t *testing.T, f *fakeAWS) *fakeAWS {
	t.Helper()
	t.Setenv("KUBESWITCH_CONFIG_DIR", t.TempDir())
	for _, v := range []string{"AWS_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION"} {
		t.Setenv(v, "")
	}
	if f.profiles == nil {
		f.profiles = []string{"default"}
		f.region = "eu-central-1"
	}

	orig := eks.Run
	eks.Run = func(args ...string) ([]byte, error) {
		f.calls = append(f.calls, args)
		sub := strings.Join(args[:min(len(args), 2)], " ")
		switch sub {
		case "configure list-profiles":
			return []byte(strings.Join(f.profiles, "\n") + "\n"), nil
		case "configure get":
			if f.region == "" {
				return nil, errors.New("aws configure get: exit status 1")
			}
			return []byte(f.region + "\n"), nil
		case "eks list-clusters":
			return json.Marshal(map[string][]string{"clusters": f.clusters})
		case "eks update-kubeconfig":
			for i, a := range args {
				if a == "--kubeconfig" {
					return []byte("Added new context\n"), os.WriteFile(args[i+1], []byte(f.kubecfg), 0o600)
				}
			}
		}
		return nil, fmt.Errorf("unexpected aws call: %v", args)
	}
	t.Cleanup(func() { eks.Run = orig })
	return f
}

func TestImportAWSByName(t *testing.T) {
	f := stubAWS(t, &fakeAWS{kubecfg: eksKubeconfig})

	out, err := execute(t, "\n1\n2\n", "import", "aws", "payments-prod",
		"--region", "eu-central-1", "--profile", "work", "--role-arn", "arn:aws:iam::123456789012:role/eks-admin")
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		`Fetching the kubeconfig for EKS cluster "payments-prod" in eu-central-1...`,
		"Name for this kubeconfig [payments-prod]: ",
		`Added "payments-prod" (prod, red)`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}

	args := f.call("update-kubeconfig")
	for _, want := range []string{
		"eks update-kubeconfig --name payments-prod --alias payments-prod --kubeconfig ",
		"--region eu-central-1 --profile work --role-arn arn:aws:iam::123456789012:role/eks-admin",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("aws args %q lack %q", args, want)
		}
	}
	if c := f.call("list-profiles"); c != "" {
		t.Errorf("profiles must not be listed when --profile is given, got %q", c)
	}
	if c := f.call("get"); c != "" {
		t.Errorf("the region must not be looked up when --region is given, got %q", c)
	}

	saved, err := os.ReadFile(filepath.Join(os.Getenv("KUBESWITCH_CONFIG_DIR"), "configs", "payments-prod.yaml"))
	if err != nil {
		t.Fatalf("kubeconfig not stored: %v", err)
	}
	if string(saved) != eksKubeconfig {
		t.Errorf("stored kubeconfig differs:\n%s", saved)
	}
}

func TestImportAWSPicksCluster(t *testing.T) {
	f := stubAWS(t, &fakeAWS{clusters: []string{"zeta", "alpha"}, kubecfg: eksKubeconfig})

	// A default profile with a region is configured: no AWS questions.
	// Pick "zeta", keep the suggested name, choose dev and blue.
	out, err := execute(t, "zeta\n\n2\n5\n", "import", "aws")
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Which EKS cluster do you want to import?",
		"  1) alpha\n  2) zeta\n",
		`Fetching the kubeconfig for EKS cluster "zeta"...`,
		`Added "zeta" (dev, blue)`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "AWS profile") || strings.Contains(out, "AWS region") {
		t.Errorf("no AWS questions expected with a configured default profile:\n%s", out)
	}
	if args := f.call("update-kubeconfig"); strings.Contains(args, "--profile") || strings.Contains(args, "--region") {
		t.Errorf("configured defaults must be left to the CLI, got %q", args)
	}
}

func TestImportAWSAsksForProfileWithoutDefault(t *testing.T) {
	f := stubAWS(t, &fakeAWS{profiles: []string{"personal", "work"}, region: "eu-west-1", kubecfg: eksKubeconfig})

	// Pick the profile, keep the suggested name, choose prod and red.
	out, err := execute(t, "work\n\n1\n2\n", "import", "aws", "payments-prod")
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Which AWS profile do you want to use?",
		"  1) personal\n  2) work\n",
		`Added "payments-prod" (prod, red)`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "AWS region") {
		t.Errorf("the profile's region is configured, no region question expected:\n%s", out)
	}
	if c := f.call("get"); c != "configure get region --profile work" {
		t.Errorf("region lookup = %q", c)
	}
	args := f.call("update-kubeconfig")
	if !strings.Contains(args, "--profile work") || strings.Contains(args, "--region") {
		t.Errorf("aws args = %q, want the chosen profile and no region", args)
	}
}

func TestImportAWSAsksForRegionWhenUnconfigured(t *testing.T) {
	f := stubAWS(t, &fakeAWS{profiles: []string{"default"}, region: "", kubecfg: eksKubeconfig})

	// An invalid region is rejected, then a valid one accepted.
	out, err := execute(t, "nope\neu-north-1\n\n1\n2\n", "import", "aws", "payments-prod")
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		"AWS region of the cluster: ",
		`"nope" does not look like an AWS region (for example eu-central-1).`,
		`Fetching the kubeconfig for EKS cluster "payments-prod" in eu-north-1...`,
		`Added "payments-prod" (prod, red)`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	args := f.call("update-kubeconfig")
	if !strings.Contains(args, "--region eu-north-1") || strings.Contains(args, "--profile") {
		t.Errorf("aws args = %q, want the chosen region and no profile", args)
	}
}

func TestImportAWSWithoutProfiles(t *testing.T) {
	stubAWS(t, &fakeAWS{profiles: []string{}})

	_, err := execute(t, "", "import", "aws", "payments-prod")
	if err == nil || !strings.Contains(err.Error(), "no AWS CLI profiles are configured") {
		t.Fatalf("expected a no-profiles error, got %v", err)
	}
}

func TestImportAWSHonorsEnvironment(t *testing.T) {
	// Listing profiles or looking up the region would fail here, so the
	// environment must settle both.
	f := stubAWS(t, &fakeAWS{profiles: []string{}, kubecfg: eksKubeconfig})
	t.Setenv("AWS_PROFILE", "work")
	t.Setenv("AWS_DEFAULT_REGION", "eu-west-1")

	out, err := execute(t, "\n1\n2\n", "import", "aws", "payments-prod")
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}
	if f.call("list-profiles") != "" || f.call("get") != "" {
		t.Errorf("the AWS CLI configuration must not be consulted, calls: %v", f.calls)
	}
}

func TestImportAWSNoClusters(t *testing.T) {
	stubAWS(t, &fakeAWS{clusters: []string{}})

	out, err := execute(t, "", "import", "aws", "--region", "eu-west-1")
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No EKS clusters found in eu-west-1.") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestImportAWSCLIError(t *testing.T) {
	t.Setenv("KUBESWITCH_CONFIG_DIR", t.TempDir())
	orig := eks.Run
	eks.Run = func(args ...string) ([]byte, error) {
		return nil, errors.New("aws eks update-kubeconfig: An error occurred (ResourceNotFoundException)")
	}
	t.Cleanup(func() { eks.Run = orig })

	_, err := execute(t, "", "import", "aws", "nope", "--profile", "work", "--region", "eu-west-1")
	if err == nil || !strings.Contains(err.Error(), "ResourceNotFoundException") {
		t.Fatalf("expected the CLI error, got %v", err)
	}
}

func TestImportAWSRejectsUnexpectedKubeconfig(t *testing.T) {
	stubAWS(t, &fakeAWS{kubecfg: "foo: bar\n"})

	_, err := execute(t, "", "import", "aws", "payments-prod")
	if err == nil || !strings.Contains(err.Error(), "unexpected kubeconfig") {
		t.Fatalf("expected an unexpected-kubeconfig error, got %v", err)
	}
}

func TestImportShowsProviders(t *testing.T) {
	out, err := execute(t, "", "import")
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	for _, provider := range []string{"aws", "kubeconfig"} {
		if !strings.Contains(out, "Usage:") || !strings.Contains(out, provider) {
			t.Errorf("expected help listing the %s provider:\n%s", provider, out)
		}
	}
}
