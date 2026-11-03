package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yanni8/kswitch/internal/eks"
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

// stubAWS replaces the AWS CLI with a fake that lists clusters and writes
// kubecfg for update-kubeconfig. It returns the recorded calls.
func stubAWS(t *testing.T, clusters []string, kubecfg string) *[][]string {
	t.Helper()
	t.Setenv("KSWITCH_CONFIG_DIR", t.TempDir())
	var calls [][]string
	orig := eks.Run
	eks.Run = func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		switch {
		case len(args) > 1 && args[1] == "list-clusters":
			return json.Marshal(map[string][]string{"clusters": clusters})
		case len(args) > 1 && args[1] == "update-kubeconfig":
			for i, a := range args {
				if a == "--kubeconfig" {
					return []byte("Added new context\n"), os.WriteFile(args[i+1], []byte(kubecfg), 0o600)
				}
			}
		}
		return nil, fmt.Errorf("unexpected aws call: %v", args)
	}
	t.Cleanup(func() { eks.Run = orig })
	return &calls
}

func TestImportAWSByName(t *testing.T) {
	calls := stubAWS(t, nil, eksKubeconfig)

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

	args := strings.Join((*calls)[0], " ")
	for _, want := range []string{
		"eks update-kubeconfig --name payments-prod --alias payments-prod --kubeconfig ",
		"--region eu-central-1 --profile work --role-arn arn:aws:iam::123456789012:role/eks-admin",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("aws args %q lack %q", args, want)
		}
	}

	saved, err := os.ReadFile(filepath.Join(os.Getenv("KSWITCH_CONFIG_DIR"), "configs", "payments-prod.yaml"))
	if err != nil {
		t.Fatalf("kubeconfig not stored: %v", err)
	}
	if string(saved) != eksKubeconfig {
		t.Errorf("stored kubeconfig differs:\n%s", saved)
	}
}

func TestImportAWSPicksCluster(t *testing.T) {
	stubAWS(t, []string{"zeta", "alpha"}, eksKubeconfig)

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
}

func TestImportAWSNoClusters(t *testing.T) {
	stubAWS(t, []string{}, eksKubeconfig)

	out, err := execute(t, "", "import", "aws", "--region", "eu-west-1")
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No EKS clusters found in eu-west-1.") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestImportAWSCLIError(t *testing.T) {
	t.Setenv("KSWITCH_CONFIG_DIR", t.TempDir())
	orig := eks.Run
	eks.Run = func(args ...string) ([]byte, error) {
		return nil, errors.New("aws eks update-kubeconfig: An error occurred (ResourceNotFoundException)")
	}
	t.Cleanup(func() { eks.Run = orig })

	_, err := execute(t, "", "import", "aws", "nope")
	if err == nil || !strings.Contains(err.Error(), "ResourceNotFoundException") {
		t.Fatalf("expected the CLI error, got %v", err)
	}
}

func TestImportAWSRejectsUnexpectedKubeconfig(t *testing.T) {
	stubAWS(t, nil, "foo: bar\n")

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
	if !strings.Contains(out, "Usage:") || !strings.Contains(out, "aws") {
		t.Errorf("expected help listing the aws provider:\n%s", out)
	}
}
