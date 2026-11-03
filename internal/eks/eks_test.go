package eks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubRun replaces the AWS CLI with fn and returns the recorded calls.
func stubRun(t *testing.T, fn func(args []string) ([]byte, error)) *[][]string {
	t.Helper()
	var calls [][]string
	orig := Run
	Run = func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		return fn(args)
	}
	t.Cleanup(func() { Run = orig })
	return &calls
}

func TestListClusters(t *testing.T) {
	calls := stubRun(t, func([]string) ([]byte, error) {
		return []byte(`{"clusters": ["zeta", "alpha"]}`), nil
	})

	got, err := ListClusters(Options{Region: "eu-central-1", Profile: "work"})
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	if strings.Join(got, ",") != "alpha,zeta" {
		t.Errorf("clusters = %v, want sorted alpha,zeta", got)
	}
	if want := "eks list-clusters --output json --region eu-central-1 --profile work"; strings.Join((*calls)[0], " ") != want {
		t.Errorf("aws args = %q, want %q", strings.Join((*calls)[0], " "), want)
	}
}

func TestListClustersErrors(t *testing.T) {
	stubRun(t, func([]string) ([]byte, error) { return []byte("not json"), nil })
	if _, err := ListClusters(Options{}); err == nil || !strings.Contains(err.Error(), "unexpected output") {
		t.Errorf("bad JSON: got %v", err)
	}

	boom := errors.New("boom")
	stubRun(t, func([]string) ([]byte, error) { return nil, boom })
	if _, err := ListClusters(Options{}); !errors.Is(err, boom) {
		t.Errorf("CLI failure: got %v, want boom", err)
	}
}

func TestKubeconfig(t *testing.T) {
	var written string
	calls := stubRun(t, func(args []string) ([]byte, error) {
		for i, a := range args {
			if a == "--kubeconfig" {
				written = args[i+1]
				return []byte("Added new context\n"), os.WriteFile(written, []byte("apiVersion: v1\n"), 0o600)
			}
		}
		return nil, errors.New("no --kubeconfig argument")
	})

	got, err := Kubeconfig("payments-prod", Options{Region: "eu-central-1", RoleARN: "arn:aws:iam::1:role/x"})
	if err != nil {
		t.Fatalf("Kubeconfig: %v", err)
	}
	if string(got) != "apiVersion: v1\n" {
		t.Errorf("kubeconfig = %q", got)
	}
	args := strings.Join((*calls)[0], " ")
	for _, want := range []string{
		"eks update-kubeconfig --name payments-prod --alias payments-prod --kubeconfig ",
		" --region eu-central-1 --role-arn arn:aws:iam::1:role/x",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("aws args %q lack %q", args, want)
		}
	}
	if _, err := os.Stat(filepath.Dir(written)); !os.IsNotExist(err) {
		t.Errorf("temp dir %s should be removed (stat: %v)", filepath.Dir(written), err)
	}
}

func TestKubeconfigPropagatesCLIError(t *testing.T) {
	boom := errors.New("boom")
	stubRun(t, func([]string) ([]byte, error) { return nil, boom })
	if _, err := Kubeconfig("x", Options{}); !errors.Is(err, boom) {
		t.Errorf("got %v, want boom", err)
	}
}

func TestRunCLI(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "aws")
	script := "#!/bin/sh\nif [ \"$1\" = fail ]; then echo \"boom\" >&2; exit 1; fi\necho \"args: $*\"\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	out, err := runCLI("eks", "list-clusters")
	if err != nil || string(out) != "args: eks list-clusters\n" {
		t.Errorf("got %q, %v", out, err)
	}

	_, err = runCLI("fail", "now")
	if err == nil || err.Error() != "aws fail now: boom" {
		t.Errorf("stderr not surfaced: %v", err)
	}

	t.Setenv("PATH", t.TempDir())
	if _, err := runCLI("eks", "list-clusters"); err == nil || !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("missing CLI: got %v", err)
	}
}
