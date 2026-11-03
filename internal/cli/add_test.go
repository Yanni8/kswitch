package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testKubeconfig = `apiVersion: v1
kind: Config
current-context: dev-cluster
clusters:
  - name: dev-cluster
    cluster:
      server: https://127.0.0.1:6443
contexts:
  - name: dev-cluster
    context:
      cluster: dev-cluster
      user: dev-user
users:
  - name: dev-user
    user: {}
`

// setupAdd points the store and the scratch files at temp dirs and
// replaces the editor with fn. It returns the store directory.
func setupAdd(t *testing.T, editor func(path string) error) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KUBESWITCH_CONFIG_DIR", dir)
	stubEditor(t, editor)
	return dir
}

// stubEditor replaces the editor with fn for the test and keeps scratch
// files in a temp dir.
func stubEditor(t *testing.T, editor func(path string) error) {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	orig := openEditor
	openEditor = editor
	t.Cleanup(func() { openEditor = orig })
}

// paste returns a fake editor that replaces the scratch file's content.
func paste(content string) func(string) error {
	return func(path string) error {
		return os.WriteFile(path, []byte(content), 0o600)
	}
}

// execAdd runs "kubeswitch add" with stdin answering the prompts.
func execAdd(t *testing.T, stdin string) (string, error) {
	t.Helper()
	return execute(t, stdin, "add")
}

func readState(t *testing.T, dir string) map[string]map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	var st struct {
		Entries map[string]map[string]string `json:"entries"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("parsing state: %v", err)
	}
	return st.Entries
}

func scratchFiles(t *testing.T) []string {
	t.Helper()
	left, err := filepath.Glob(filepath.Join(os.Getenv("TMPDIR"), "kubeswitch-*"))
	if err != nil {
		t.Fatal(err)
	}
	return left
}

func TestAddWithDefaults(t *testing.T) {
	dir := setupAdd(t, paste(testKubeconfig))

	// Enter accepts the default name; 1 = prod; 2 = red.
	out, err := execAdd(t, "\n1\n2\n")
	if err != nil {
		t.Fatalf("add failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Name for this kubeconfig [dev-cluster]:") {
		t.Errorf("default name not offered:\n%s", out)
	}
	if !strings.Contains(out, `Added "dev-cluster" (prod, red)`) {
		t.Errorf("unexpected confirmation:\n%s", out)
	}

	saved, err := os.ReadFile(filepath.Join(dir, "configs", "dev-cluster.yaml"))
	if err != nil {
		t.Fatalf("kubeconfig not stored: %v", err)
	}
	if string(saved) != testKubeconfig {
		t.Errorf("stored kubeconfig differs:\n%s", saved)
	}

	entry := readState(t, dir)["dev-cluster"]
	if entry["environment"] != "prod" || entry["color"] != "red" || entry["file"] != "dev-cluster.yaml" {
		t.Errorf("unexpected entry: %v", entry)
	}
	if left := scratchFiles(t); len(left) != 0 {
		t.Errorf("scratch file left behind: %v", left)
	}
}

func TestAddOpensAnEmptyScratchFile(t *testing.T) {
	var seen []byte
	setupAdd(t, func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		seen = data
		return os.WriteFile(path, []byte(testKubeconfig), 0o600)
	})

	out, err := execAdd(t, "\n1\n2\n")
	if err != nil {
		t.Fatalf("add failed: %v\n%s", err, out)
	}
	if len(seen) != 0 {
		t.Errorf("the scratch file should be empty, the editor saw %q", seen)
	}
	if !strings.Contains(out, "Paste your kubeconfig into the editor") {
		t.Errorf("expected a hint before the editor opens:\n%s", out)
	}
}

func TestAddWithCustomName(t *testing.T) {
	dir := setupAdd(t, paste(testKubeconfig))

	out, err := execAdd(t, "My Test Cluster\n2\n5\n") // test, blue
	if err != nil {
		t.Fatalf("add failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `Added "My Test Cluster" (dev, blue)`) {
		t.Errorf("unexpected confirmation:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "configs", "my-test-cluster.yaml")); err != nil {
		t.Errorf("kubeconfig not stored under slug: %v", err)
	}
	if entry := readState(t, dir)["My Test Cluster"]; entry["file"] != "my-test-cluster.yaml" {
		t.Errorf("unexpected entry: %v", entry)
	}
}

func TestAddAcceptsChoiceNames(t *testing.T) {
	setupAdd(t, paste(testKubeconfig))

	out, err := execAdd(t, "\nprod\nCyan\n")
	if err != nil {
		t.Fatalf("add failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "(prod, cyan)") {
		t.Errorf("unexpected confirmation:\n%s", out)
	}
}

func TestAddRepromptsOnInvalidAnswers(t *testing.T) {
	setupAdd(t, paste(testKubeconfig))

	out, err := execAdd(t, "\nbanana\n1\n99\n3\n")
	if err != nil {
		t.Fatalf("add failed: %v\n%s", err, out)
	}
	if n := strings.Count(out, "Please answer with"); n != 2 {
		t.Errorf("expected two re-prompts, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "(prod, green)") {
		t.Errorf("unexpected confirmation:\n%s", out)
	}
}

func TestAddRepromptsOnDuplicateName(t *testing.T) {
	setupAdd(t, paste(testKubeconfig))
	if out, err := execAdd(t, "\n1\n2\n"); err != nil {
		t.Fatalf("first add failed: %v\n%s", err, out)
	}

	// The default name is taken (case-insensitively), so another is needed.
	out, err := execAdd(t, "DEV-CLUSTER\nsecond\n2\n3\n")
	if err != nil {
		t.Fatalf("second add failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "already exists") {
		t.Errorf("expected duplicate warning:\n%s", out)
	}
	if !strings.Contains(out, `Added "second" (dev, green)`) {
		t.Errorf("unexpected confirmation:\n%s", out)
	}
}

func TestAddRejectsBadName(t *testing.T) {
	setupAdd(t, paste(testKubeconfig))

	out, err := execAdd(t, "a\tb\nfine\n1\n1\n")
	if err != nil {
		t.Fatalf("add failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "must not contain control characters") {
		t.Errorf("expected name validation message:\n%s", out)
	}
	if !strings.Contains(out, `Added "fine"`) {
		t.Errorf("unexpected confirmation:\n%s", out)
	}
}

func TestAddAbortsWhenNothingPasted(t *testing.T) {
	dir := setupAdd(t, func(string) error { return nil }) // template left untouched

	_, err := execAdd(t, "\n1\n2\n")
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("expected abort, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "state.json")); !os.IsNotExist(err) {
		t.Errorf("nothing should have been stored (stat: %v)", err)
	}
	if left := scratchFiles(t); len(left) != 0 {
		t.Errorf("scratch file left behind: %v", left)
	}
}

func TestAddKeepsPasteWhenNotAKubeconfig(t *testing.T) {
	setupAdd(t, paste("foo: bar\n"))

	out, err := execAdd(t, "\n1\n2\n")
	if err == nil || !strings.Contains(err.Error(), "does not look like a kubeconfig") {
		t.Fatalf("expected kubeconfig error, got %v", err)
	}
	if !strings.Contains(out, "is kept at") {
		t.Errorf("expected notice about the kept scratch file:\n%s", out)
	}
	left := scratchFiles(t)
	if len(left) != 1 {
		t.Fatalf("expected one kept scratch file, got %v", left)
	}
	if data, _ := os.ReadFile(left[0]); string(data) != "foo: bar\n" {
		t.Errorf("kept file content: %q", data)
	}
}

func TestAddAbortsWhenInputEnds(t *testing.T) {
	setupAdd(t, paste(testKubeconfig))

	_, err := execAdd(t, "\n") // name accepted, then no answer for the environment
	if err == nil || err.Error() != "aborted" {
		t.Fatalf("expected aborted, got %v", err)
	}
}

func TestAddFailsWhenEditorFails(t *testing.T) {
	setupAdd(t, func(string) error { return errors.New("boom") })

	_, err := execAdd(t, "")
	if err == nil || !strings.Contains(err.Error(), "editor: boom") {
		t.Fatalf("expected editor error, got %v", err)
	}
	if left := scratchFiles(t); len(left) != 0 {
		t.Errorf("scratch file left behind: %v", left)
	}
}
