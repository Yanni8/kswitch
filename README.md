# KSwitch

Fast switcher between kubeconfigs, with a terminal background color per
config so prod is never mistaken for dev.

## Commands

### `kswitch add`

Adds a kubeconfig. It opens your editor (`$VISUAL`, then `$EDITOR`,
default `vim`) on an empty scratch file; paste the kubeconfig, save, and
quit.
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

### `kswitch import aws [cluster]`

Imports the kubeconfig of an AWS EKS cluster instead of pasting one.
Without a cluster name, the clusters in the region are listed to pick
from. The kubeconfig is fetched with `aws eks update-kubeconfig`, so the
AWS CLI must be installed and configured; `--region`, `--profile`, and
`--role-arn` are passed through to it. You are then asked for the name
(defaulting to the cluster name), the environment, and the color, as
with `add`. Authentication to the cluster goes through the AWS CLI
(`aws eks get-token`); the profile, if given, is remembered in the
kubeconfig. `kswitch import` on its own lists the available providers.

### `kswitch edit [name]`

Changes one thing about a kubeconfig (alias: `update`). Without a name
you pick one from a menu, then choose what to change: the kubeconfig
contents (opened in your editor, pre-filled with the current file), the
name, the environment, or the color. The current value is suggested or
preselected, so Enter keeps it. Renaming keeps the stored file's name,
so shells already using it are unaffected.

### `kswitch list`

Lists the configured kubeconfigs with their environment and color
(alias: `ls`). Color swatches are shown only when printing to a terminal.

### `kswitch remove [name]`

Removes a kubeconfig and deletes its stored file (alias: `rm`). Without
a name you pick one from a menu. It asks for confirmation unless you pass
`-y`/`--yes`.

### `kswitch use [name]`

Switches to a kubeconfig (alias: `switch`). Without a name you pick one
from a menu. kswitch starts a new shell (`$SHELL`) with `KUBECONFIG`
pointing at the chosen config and the terminal background set to its
color; the text color is set to light or dark, whichever reads better on
it. Type `exit` to return to the previous shell, which restores both
colors. Inside the shell `KSWITCH_NAME` and `KSWITCH_ENV` are set, handy
for your prompt. The shell's exit status is passed through.

The colors are deep shades (red is `#5a1515`, for example) so that a
whole terminal in them stays comfortable and ANSI-colored output remains
readable; white is the one light option. The swatches in the menus show
sample text in the exact combination you will get.

### `kswitch version`

Prints version information.
