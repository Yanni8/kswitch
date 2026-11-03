package kubeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const sampleKubeconfig = `apiVersion: v1
kind: Config
current-context: dev
preferences:
  colors: true
clusters:
- name: dev
  cluster:
    server: https://dev.example.com
    certificate-authority: certs/ca.crt
- name: prod
  cluster:
    server: https://prod.example.com
    certificate-authority-data: LS0t
contexts:
- name: dev
  context:
    cluster: dev
    user: admin
    namespace: default
- name: prod
  context:
    cluster: prod
    user: admin
- name: orphan
  context:
    cluster: missing
    user: admin
- name: anonymous
  context:
    cluster: prod
users:
- name: admin
  user:
    client-certificate: /abs/admin.crt
    client-key: certs/admin.key
`

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// parsed decodes a split kubeconfig for inspection.
func parsed(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("split kubeconfig is not valid YAML: %v\n%s", err, data)
	}
	return doc
}

func TestSplit(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "config", sampleKubeconfig)

	contexts, err := Split(path)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	var names []string
	for _, c := range contexts {
		names = append(names, c.Name)
	}
	if got := strings.Join(names, ","); got != "dev,prod,anonymous" {
		t.Fatalf("contexts = %s, want dev,prod,anonymous (the orphan has no cluster)", got)
	}

	dev := contexts[0]
	if dev.Cluster != "dev" || dev.Server != "https://dev.example.com" {
		t.Errorf("dev = %+v", dev)
	}
	doc := parsed(t, dev.Config)
	if doc["current-context"] != "dev" || doc["kind"] != "Config" || doc["apiVersion"] != "v1" {
		t.Errorf("header = %v", doc)
	}
	if n := len(entries(doc["clusters"])); n != 1 {
		t.Errorf("dev config has %d clusters, want 1", n)
	}
	if n := len(entries(doc["contexts"])); n != 1 {
		t.Errorf("dev config has %d contexts, want 1", n)
	}
	cluster := sub(entries(doc["clusters"])[0], "cluster")
	if want := filepath.Join(dir, "certs/ca.crt"); cluster["certificate-authority"] != want {
		t.Errorf("certificate-authority = %v, want %s", cluster["certificate-authority"], want)
	}
	user := sub(entries(doc["users"])[0], "user")
	if user["client-certificate"] != "/abs/admin.crt" {
		t.Errorf("absolute paths must be kept, got %v", user["client-certificate"])
	}
	if want := filepath.Join(dir, "certs/admin.key"); user["client-key"] != want {
		t.Errorf("client-key = %v, want %s", user["client-key"], want)
	}
	if prefs, _ := doc["preferences"].(map[string]any); prefs["colors"] != true {
		t.Errorf("preferences not carried over: %v", doc["preferences"])
	}
	if _, err := Parse(dev.Config); err != nil {
		t.Errorf("split config does not parse as a kubeconfig: %v", err)
	}

	if anon := parsed(t, contexts[2].Config); len(entries(anon["users"])) != 0 {
		t.Errorf("a context without a user must not get one: %v", anon["users"])
	}
}

func TestSplitMergesFilesFirstWins(t *testing.T) {
	dir := t.TempDir()
	first := write(t, dir, "first", `clusters:
- name: shared
  cluster:
    server: https://first.example.com
contexts:
- name: one
  context:
    cluster: shared
    user: u
users:
- name: u
  user:
    token: t
`)
	second := write(t, dir, "second", `clusters:
- name: shared
  cluster:
    server: https://second.example.com
contexts:
- name: two
  context:
    cluster: shared
    user: u
`)

	contexts, err := Split(first, second)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(contexts) != 2 || contexts[0].Name != "one" || contexts[1].Name != "two" {
		t.Fatalf("contexts = %+v", contexts)
	}
	if contexts[1].Server != "https://first.example.com" {
		t.Errorf("the first definition of a cluster must win, got %s", contexts[1].Server)
	}
	if doc := parsed(t, contexts[1].Config); len(entries(doc["users"])) != 1 {
		t.Error("the user from the first file must be available to the second file's context")
	}
}

func TestSplitErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Split(filepath.Join(dir, "missing")); !os.IsNotExist(err) {
		t.Errorf("missing file: got %v, want a not-exist error", err)
	}
	bad := write(t, dir, "bad", "clusters: [\n")
	if _, err := Split(bad); err == nil || !strings.Contains(err.Error(), "not valid YAML") {
		t.Errorf("invalid YAML: got %v", err)
	}
	empty := write(t, dir, "empty", "apiVersion: v1\nkind: Config\n")
	if contexts, err := Split(empty); err != nil || len(contexts) != 0 {
		t.Errorf("no contexts: got %v, %v", contexts, err)
	}
}

func TestDefaultPaths(t *testing.T) {
	t.Setenv("KUBECONFIG", "/a/config:/b/config::")
	if paths, err := DefaultPaths(); err != nil || strings.Join(paths, ",") != "/a/config,/b/config" {
		t.Errorf("KUBECONFIG set: got %v, %v", paths, err)
	}

	t.Setenv("KUBECONFIG", "")
	t.Setenv("HOME", "/home/someone")
	if paths, err := DefaultPaths(); err != nil || strings.Join(paths, ",") != "/home/someone/.kube/config" {
		t.Errorf("KUBECONFIG unset: got %v, %v", paths, err)
	}
}
