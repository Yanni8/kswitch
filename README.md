# KSwitch

Fast switcher between kubeconfigs, with a terminal background color per
config so prod is never mistaken for dev.

## Commands

### `kswitch add`

Adds a kubeconfig. It opens your editor (`$VISUAL`, then `$EDITOR`,
default `vim`) on a scratch file; paste the kubeconfig, save, and quit.
You are then asked for:

- a human-readable name (defaults to the kubeconfig's current context),
- whether it is a `prod` or a `dev` environment,
- a terminal background color (black, red, green, yellow, blue, magenta,
  cyan, white).

The name is a text field that shows the suggestion as a placeholder:
Enter accepts it, typing replaces it, and Tab (or `→`) copies it into
the field for editing. The other two are menus: move with `↑`/`↓` (or
`j`/`k`) and confirm with Enter. Esc or Ctrl-C aborts any prompt. When
stdin is not a terminal, the prompts turn into plain line-based ones
(`Name [suggestion]:` and numbered lists).

Kubeconfigs are stored with mode `0600` under `~/.config/kswitch/configs/`
(override the directory with `KSWITCH_CONFIG_DIR`), and the metadata in
`~/.config/kswitch/state.json`.

### `kswitch list`

Lists the configured kubeconfigs with their environment and color
(alias: `ls`). Color swatches are shown only when printing to a terminal.

### `kswitch remove [name]`

Removes a kubeconfig and deletes its stored file (alias: `rm`). Without
a name you pick one from a menu. It asks for confirmation unless you pass
`-y`/`--yes`.

### `kswitch version`

Prints version information.
