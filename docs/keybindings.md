# Keybindings

Press `?` in the app for this list, or `Ctrl+K` for a fuzzy command palette that
can run any of these and jump to any resource.

## Navigation

| Key | Action |
| --- | --- |
| `Up` / `k`, `Down` / `j` | move |
| `g` / `G` | top / bottom |
| `Ctrl+u` / `Ctrl+d` | half page up / down |
| `PgUp` / `PgDn` | page up / down |
| `Tab` | switch pane (nav / table) |
| `Left` / `h` | focus the nav from the table |
| `Right` / `l` | open the selected nav entry while the nav is focused |

## Row actions (table)

| Key | Action |
| --- | --- |
| `Enter` | config summary |
| `d` / `y` | YAML |
| `l` | logs (pods); `f` toggles follow |
| `e` | edit in `$EDITOR`, then confirm apply |
| `s` | shell into a pod, node shell on a node, or scale a workload |
| `p` | port-forward a Service |
| `R` | rollout restart (deployments, statefulsets, daemonsets) |
| `t` | trigger a CronJob once (with confirm) |
| `K` | cordon / uncordon a node (with confirm) |
| `D` | drain a node: cordon and evict its pods (with confirm) |
| `x` / `Delete` | delete (with confirm) |
| `O` | open Kubernetes docs for the current resource, when known |

The bottom bar adapts to the selected resource: pods show logs and shell,
Services show port-forward, nodes show node shell, cordon, and drain, workloads
show scale and restart, and CronJobs show trigger.

Draining cordons the node, then evicts its pods through the eviction API so
PodDisruptionBudgets are honored. DaemonSet and static (mirror) pods are left in
place, the same as `kubectl drain --ignore-daemonsets`.

## Views and cluster

| Key | Action |
| --- | --- |
| `:` | jump to any resource (`pods`, `deploy`, `svc`, ...) |
| `Ctrl+K` | command palette |
| `Shift+E` | toggle edit mode / read-only mode |
| `/` | filter the current table (`esc` clears) |
| `S` | sort by a column (re-pick to flip direction) |
| `r` / `Ctrl+r` | refresh now |
| `w` | toggle wide columns |
| `n` | switch namespace |
| `a` | toggle all namespaces |
| `c` | switch context |
| `C` | show the equivalent `kubectl` command |
| `O` | open Kubernetes docs for the current resource, when known |
| `?` | help |
| `q` | quit from the table or cockpit; back from config, YAML, and logs |
| `Ctrl+C` | quit outside the embedded shell/editor |

`ku` starts read-only unless launched with `--edit`. Mutating actions are hidden
until edit mode is enabled with `Shift+E` or the command palette.

## Overlays

| Context | Keys |
| --- | --- |
| Logs | `f` follow, `/` filter (regex), `w` (or `Ctrl+w` while filtering) wrap / truncate lines, `c` copy all, `Ctrl+l` clear, `v` select lines, `g` / `G` top / bottom, `esc` back |
| Logs (selecting) | `↑` / `↓` (and `g` / `G`, page keys) move the cursor, `m` mark, `y` / `Enter` copy, `esc` cancel |
| Config summary | scroll, `d` / `y` YAML, `e` edit, `t` trigger CronJob, `esc` back |
| Detail (YAML) | scroll, `Enter` config, `e` edit, `t` trigger CronJob, `esc` back |
| Shell / editor | keys go to the program; `Ctrl+Shift+V` pastes in shell mode; `Ctrl+V` is passed through; `Ctrl+\` detaches (cancels an edit) |
| Command overlay | `C`, `q`, or `esc` closes |
| Pickers / palette | move, type to filter, `Enter` select, `esc` cancel |
| Confirm | `y` / `Enter` confirm, `n` / `esc` cancel |

## Selecting and copying logs

Press `v` to enter selection mode, move the cursor with `↑` / `↓` (or `g` / `G`
and the page keys), press `m` to mark the start, then move to extend the range
and `y` or `Enter` to copy it to the clipboard (via OSC 52, so it works over SSH
too). Without marking, `y` copies the cursor line. `esc` cancels.

Selection is whole-line and copies the full, untruncated line text with terminal
styling stripped, so no escape codes end up on the clipboard.

To grab the whole buffer at once, press `c` to copy every buffered line to the
clipboard (the raw lines, so an active filter never hides anything). Press
`Ctrl+l` to clear the on-screen buffer; the stream keeps running, so new lines
flow back in right away.

## Shell paste and mouse

In shell mode, paste with `Ctrl+Shift+V`. `Ctrl+V` is not a paste shortcut; it is
sent to the running shell/program. Mouse input is only captured in shell mode, so
terminal programs that enable mouse tracking can receive it. The shell keeps the
same bordered TUI frame as the rest of ku.

## Service port-forward

Press `p` on a Service in edit mode. Pick a Service port, or type
`local:service-port` to choose your local machine port. Examples:

- `8080:http` forwards local port `8080` to Service port `http`.
- `18080:80` forwards local port `18080` to Service port `80`.

ClusterIP and NodePort Services are forwarded the same way. If you pick port
`80` directly, ku tries local port `80`; type `8080:80` to use local `8080`.

The forward runs in the background. Active forwards show as a `pf` count in the
header. To list or stop them, open the command palette with `Ctrl+K`, pick
`Port-forwards`, then select a forward. Selecting a forward stops it.
