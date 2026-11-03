// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Yanni8/kubeswitch/internal/store"
)

const localKubeconfig = `apiVersion: v1
kind: Config
current-context: dev
clusters:
- name: dev
  cluster:
    server: https://dev.example.com
    certificate-authority: certs/ca.crt
- name: prod
  cluster:
    server: https://prod.example.com
contexts:
- name: dev
  context:
    cluster: dev
    user: admin
- name: prod
  context:
    cluster: prod
    user: admin
users:
- name: admin
  user:
    token: secret
`

// writeKubeconfig stores content as a kubeconfig file in a temp dir and
// points the store at another one.
func writeKubeconfig(t *testing.T, content string) string {
	t.Helper()
	t.Setenv("KUBESWITCH_CONFIG_DIR", t.TempDir())
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportKubeconfigKeepsAndSkips(t *testing.T) {
	path := writeKubeconfig(t, localKubeconfig)

	// Keep "dev" (name as suggested, prod, red), skip "prod".
	out, err := execute(t, "keep\n\n1\n2\nskip\n", "import", "kubeconfig", path)
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Found 2 contexts in " + path + ".",
		`Import "dev" (https://dev.example.com)?`,
		"Name for this kubeconfig [dev]: ",
		`Added "dev" (prod, red)`,
		`Import "prod" (https://prod.example.com)?`,
		"Imported 1 of 2 contexts.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}

	st, _ := store.Open()
	list, _ := st.List()
	if len(list) != 1 || list[0].Name != "dev" {
		t.Fatalf("stored = %+v, want only dev", list)
	}
	data, err := os.ReadFile(list[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		CurrentContext string `yaml:"current-context"`
		Clusters       []struct {
			Name    string `yaml:"name"`
			Cluster struct {
				CA string `yaml:"certificate-authority"`
			} `yaml:"cluster"`
		} `yaml:"clusters"`
		Users []struct {
			Name string `yaml:"name"`
		} `yaml:"users"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("stored kubeconfig is not valid YAML: %v", err)
	}
	if doc.CurrentContext != "dev" || len(doc.Clusters) != 1 || doc.Clusters[0].Name != "dev" {
		t.Errorf("stored kubeconfig = %+v, want only the dev cluster", doc)
	}
	if want := filepath.Join(filepath.Dir(path), "certs/ca.crt"); doc.Clusters[0].Cluster.CA != want {
		t.Errorf("certificate-authority = %q, want %q", doc.Clusters[0].Cluster.CA, want)
	}
	if len(doc.Users) != 1 || doc.Users[0].Name != "admin" {
		t.Errorf("users = %+v, want admin", doc.Users)
	}
}

func TestImportKubeconfigUsesKUBECONFIGByDefault(t *testing.T) {
	path := writeKubeconfig(t, localKubeconfig)
	t.Setenv("KUBECONFIG", path)

	out, err := execute(t, "skip\nkeep\n\n2\n3\n", "import", "kubeconfig")
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Found 2 contexts in "+path+".") || !strings.Contains(out, `Added "prod" (dev, green)`) {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestImportKubeconfigNotesExistingNames(t *testing.T) {
	path := writeKubeconfig(t, localKubeconfig)
	st, _ := store.Open()
	if _, err := st.Add("dev", store.EnvDev, "blue", []byte("a: 1\n")); err != nil {
		t.Fatal(err)
	}

	out, err := execute(t, "skip\nskip\n", "import", "kubeconfig", path)
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `Import "dev" (https://dev.example.com)? A kubeconfig named "dev" already exists.`) {
		t.Errorf("expected a note about the existing name:\n%s", out)
	}
	if !strings.Contains(out, "Imported 0 of 2 contexts.") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestImportKubeconfigWithoutContexts(t *testing.T) {
	path := writeKubeconfig(t, "apiVersion: v1\nkind: Config\n")

	out, err := execute(t, "", "import", "kubeconfig", path)
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No contexts found in "+path+".") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestImportKubeconfigMissingDefault(t *testing.T) {
	t.Setenv("KUBESWITCH_CONFIG_DIR", t.TempDir())
	missing := filepath.Join(t.TempDir(), "config")
	t.Setenv("KUBECONFIG", missing)

	_, err := execute(t, "", "import", "kubeconfig")
	if err == nil || !strings.Contains(err.Error(), "no kubeconfig found at "+missing) {
		t.Fatalf("expected a friendly not-found error, got %v", err)
	}
}

func TestImportKubeconfigAbortKeepsImported(t *testing.T) {
	path := writeKubeconfig(t, localKubeconfig)

	// Keep "dev", then input ends at the question for "prod".
	_, err := execute(t, "keep\n\n1\n2\n", "import", "kubeconfig", path)
	if err == nil || err.Error() != "aborted" {
		t.Fatalf("expected aborted, got %v", err)
	}
	st, _ := store.Open()
	if exists, _ := st.Exists("dev"); !exists {
		t.Error("contexts imported before the abort must stay")
	}
}
