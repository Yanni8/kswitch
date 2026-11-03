package cli

import "github.com/Yanni8/kubeswitch/internal/palette"

// colorSwatch previews the named palette color, or returns "" if the
// name is not in the palette.
func colorSwatch(name string) string {
	c, ok := palette.Find(name)
	if !ok {
		return ""
	}
	return c.Swatch()
}
