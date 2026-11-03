package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yanni8/kswitch/internal/store"
)

func getKubeconfig(t *testing.T, name string) store.Kubeconfig {
	t.Helper()
	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	k, err := st.Get(name)
	if err != nil {
		t.Fatalf("Get(%q): %v", name, err)
	}
	return k
}

func TestEditColor(t *testing.T) {
	seedStore(t)

	// Field 4 is the color; answers may also be typed as names.
	out, err := execute(t, "color\nblue\n", "edit", "payments prod")
	if err != nil {
		t.Fatalf("edit failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		`What do you want to change for "Payments Prod"?`,
		"Color  (red)",
		"(default: red)",
		`Updated "Payments Prod": color is now blue`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if k := getKubeconfig(t, "Payments Prod"); k.Color != "blue" || k.Environment != store.EnvProd {
		t.Errorf("entry = %+v", k)
	}
}

func TestEditEnvironment(t *testing.T) {
	seedStore(t)

	// An empty answer keeps the current value.
	out, err := execute(t, "3\n\n", "edit", "Payments Prod")
	if err != nil {
		t.Fatalf("edit failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "(default: prod)") || !strings.Contains(out, "Nothing changed.") {
		t.Errorf("unexpected output:\n%s", out)
	}

	out, err = execute(t, "3\ndev\n", "edit", "Payments Prod")
	if err != nil {
		t.Fatalf("edit failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `Updated "Payments Prod": environment is now dev`) {
		t.Errorf("unexpected output:\n%s", out)
	}
	if k := getKubeconfig(t, "Payments Prod"); k.Environment != store.EnvDev {
		t.Errorf("entry = %+v", k)
	}
}

func TestEditRename(t *testing.T) {
	dir := seedStore(t)
	file := filepath.Join(dir, "configs", "payments-prod.yaml")

	out, err := execute(t, "2\nStaging\n", "update", "Payments Prod") // the alias
	if err != nil {
		t.Fatalf("edit failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "New name [Payments Prod]: ") || !strings.Contains(out, `Updated "Payments Prod": renamed to "Staging"`) {
		t.Errorf("unexpected output:\n%s", out)
	}
	if k := getKubeconfig(t, "Staging"); k.Path != file || k.Environment != store.EnvProd || k.Color != "red" {
		t.Errorf("renamed entry = %+v; the file and metadata must be kept", k)
	}
	st, _ := store.Open()
	if _, err := st.Get("Payments Prod"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("old name still resolves: %v", err)
	}
}

func TestEditRenameRejectsTakenNameAndKeepsOwn(t *testing.T) {
	seedStore(t)

	out, err := execute(t, "2\nDEV CLUSTER\nStaging\n", "edit", "Payments Prod")
	if err != nil {
		t.Fatalf("edit failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `A kubeconfig named "dev cluster" already exists`) || !strings.Contains(out, `renamed to "Staging"`) {
		t.Errorf("unexpected output:\n%s", out)
	}

	// Keeping the current name is fine and changes nothing.
	out, err = execute(t, "2\n\n", "edit", "Staging")
	if err != nil {
		t.Fatalf("edit failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Nothing changed.") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestEditContents(t *testing.T) {
	dir := seedStore(t)
	file := filepath.Join(dir, "configs", "payments-prod.yaml")
	stubEditor(t, paste(testKubeconfig))

	out, err := execute(t, "1\n", "edit", "Payments Prod")
	if err != nil {
		t.Fatalf("edit failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `Updated "Payments Prod": kubeconfig contents replaced`) {
		t.Errorf("unexpected output:\n%s", out)
	}
	if data, _ := os.ReadFile(file); string(data) != testKubeconfig {
		t.Errorf("file content = %q", data)
	}
	if left := scratchFiles(t); len(left) != 0 {
		t.Errorf("scratch file left behind: %v", left)
	}
}

func TestEditContentsOpensCurrentFile(t *testing.T) {
	seedStore(t)
	var seen string
	stubEditor(t, func(path string) error {
		data, err := os.ReadFile(path)
		seen = string(data)
		return err
	})

	// The editor is not touched, so the contents stay as they are.
	out, err := execute(t, "contents\n", "edit", "Payments Prod")
	if err != nil {
		t.Fatalf("edit failed: %v\n%s", err, out)
	}
	if seen != editTemplate+"a: 1\n" {
		t.Errorf("editor saw %q, want the template followed by the current kubeconfig", seen)
	}
	if !strings.Contains(out, "Nothing changed.") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestEditContentsRejectsInvalidAndKeepsFile(t *testing.T) {
	dir := seedStore(t)
	file := filepath.Join(dir, "configs", "payments-prod.yaml")
	stubEditor(t, paste("foo: bar\n"))

	out, err := execute(t, "1\n", "edit", "Payments Prod")
	if err == nil || !strings.Contains(err.Error(), "does not look like a kubeconfig") {
		t.Fatalf("expected kubeconfig error, got %v", err)
	}
	if !strings.Contains(out, "is kept at") {
		t.Errorf("expected notice about the kept scratch file:\n%s", out)
	}
	if data, _ := os.ReadFile(file); string(data) != "a: 1\n" {
		t.Errorf("stored kubeconfig must be untouched, got %q", data)
	}
	if left := scratchFiles(t); len(left) != 1 {
		t.Errorf("expected one kept scratch file, got %v", left)
	}
}

func TestEditPicksFromMenu(t *testing.T) {
	seedStore(t)

	out, err := execute(t, "dev cluster\n4\ncyan\n", "edit")
	if err != nil {
		t.Fatalf("edit failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Which kubeconfig do you want to edit?") || !strings.Contains(out, `Updated "dev cluster": color is now cyan`) {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestEditUnknownName(t *testing.T) {
	seedStore(t)

	_, err := execute(t, "", "edit", "nope")
	if err == nil || !strings.Contains(err.Error(), `kubeconfig "nope" not found`) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestEditEmptyStore(t *testing.T) {
	t.Setenv("KSWITCH_CONFIG_DIR", t.TempDir())

	out, err := execute(t, "", "edit")
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	if !strings.Contains(out, "No kubeconfigs yet") {
		t.Errorf("unexpected output:\n%s", out)
	}
}
