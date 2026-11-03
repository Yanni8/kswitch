package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yanni8/kswitch/internal/store"
)

// seedStore points the store at a temp dir holding two kubeconfigs and
// returns the dir.
func seedStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KSWITCH_CONFIG_DIR", dir)
	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		env   store.Environment
		color string
	}{
		{"Payments Prod", store.EnvProd, "red"},
		{"dev cluster", store.EnvDev, "green"},
	} {
		if _, err := st.Add(c.name, c.env, c.color, []byte("a: 1\n")); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func fileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

func TestRemoveByNameWithYes(t *testing.T) {
	dir := seedStore(t)
	file := filepath.Join(dir, "configs", "payments-prod.yaml")

	out, err := execute(t, "", "remove", "payments prod", "--yes") // case-insensitive
	if err != nil {
		t.Fatalf("remove failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `Removed "Payments Prod"`) {
		t.Errorf("unexpected output:\n%s", out)
	}
	if fileExists(t, file) {
		t.Error("kubeconfig file was not deleted")
	}
	if list, _ := execute(t, "", "list"); strings.Contains(list, "Payments Prod") || !strings.Contains(list, "dev cluster") {
		t.Errorf("unexpected list after removal:\n%s", list)
	}
}

func TestRemoveAsksForConfirmation(t *testing.T) {
	dir := seedStore(t)
	file := filepath.Join(dir, "configs", "payments-prod.yaml")

	// Option 1 is "No, keep it".
	out, err := execute(t, "1\n", "rm", "Payments Prod")
	if err != nil {
		t.Fatalf("remove failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `Remove "Payments Prod" (prod, red)?`) || !strings.Contains(out, "Nothing removed.") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if !fileExists(t, file) {
		t.Fatal("kubeconfig file was deleted despite answering no")
	}

	out, err = execute(t, "yes\n", "rm", "Payments Prod")
	if err != nil {
		t.Fatalf("remove failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `Removed "Payments Prod"`) || fileExists(t, file) {
		t.Errorf("kubeconfig should be removed after answering yes:\n%s", out)
	}
}

func TestRemovePicksFromMenu(t *testing.T) {
	dir := seedStore(t)

	// Choose by name in the fallback menu, then confirm with option 2.
	out, err := execute(t, "dev cluster\n2\n", "remove")
	if err != nil {
		t.Fatalf("remove failed: %v\n%s", err, out)
	}
	for _, want := range []string{"Which kubeconfig should be removed?", "dev cluster  (dev, green)", `Removed "dev cluster"`} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if fileExists(t, filepath.Join(dir, "configs", "dev-cluster.yaml")) {
		t.Error("kubeconfig file was not deleted")
	}
	if !fileExists(t, filepath.Join(dir, "configs", "payments-prod.yaml")) {
		t.Error("the other kubeconfig must be untouched")
	}
}

func TestRemoveAbortedMenuChangesNothing(t *testing.T) {
	dir := seedStore(t)

	_, err := execute(t, "", "remove") // input ends at the menu
	if err == nil || err.Error() != "aborted" {
		t.Fatalf("expected aborted, got %v", err)
	}
	for _, f := range []string{"payments-prod.yaml", "dev-cluster.yaml"} {
		if !fileExists(t, filepath.Join(dir, "configs", f)) {
			t.Errorf("%s was deleted", f)
		}
	}
}

func TestRemoveUnknownName(t *testing.T) {
	seedStore(t)

	_, err := execute(t, "", "remove", "nope", "-y")
	if err == nil || !strings.Contains(err.Error(), `kubeconfig "nope" not found`) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRemoveEmptyStore(t *testing.T) {
	t.Setenv("KSWITCH_CONFIG_DIR", t.TempDir())

	out, err := execute(t, "", "remove")
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if !strings.Contains(out, "No kubeconfigs to remove.") {
		t.Errorf("unexpected output:\n%s", out)
	}
}
