# Configuration

Customize the left sidebar and add plugin shortcuts with
`~/.config/ku/config.yaml`.

## Quick Start

```bash
ku config init          # write the default config to populate from
ku config init --force  # overwrite an existing config with the defaults
ku config path          # print the config file location
```

After seeding the file, edit it and restart `ku`. The running TUI reads config
once at startup and never writes it.

## Files

| File | Purpose |
| --- | --- |
| `~/.config/ku/config.yaml` | user-authored config file |
| `~/.config/ku/state.json` | auto-saved context and namespace state |

The config file is separate from session state. `config.yaml` is only written by
you or by `ku config init`; `state.json` is managed automatically. Note that
`ku config init --force` rewrites the whole file from the defaults, so back up
a hand-written `plugins:` section first.

## Sidebar

Today the config customizes the left sidebar menu. When a `sidebar:` list is
present it replaces the built-in default menu. Without a config file the built-in
defaults are used.

The Overview entry is always available. Resources the cluster does not expose
are dropped, and empty sections are hidden.

```yaml
sidebar:
  - section: Workloads
    items:
      - { label: Pods, resource: pods }
      - { label: Deployments, resource: deployments }
      - { label: HPAs, resource: horizontalpodautoscalers }
      - { label: ScaledObjects, resource: scaledobjects }
  - section: Network
    items:
      - { label: Services, resource: services }
```

The `resource` field accepts anything the resource picker resolves: a plural,
singular, kind, short name, or group-qualified key, such as
`scaledobjects.keda.sh`.

## Plugins

A plugin binds a key on the table to an external command. Press the key on a
row and ku runs the command with that row's coordinates. Plugins work in
read-only mode as well as edit mode: ku trusts what you put in your own config.

```yaml
plugins:
  - key: ctrl+o
    desc: open dashboard
    scopes: [pods, deployments]
    command: open
    args: ["https://dashboard.example.com/$CLUSTER/$NAMESPACE/$RESOURCE/$NAME"]
    background: true
  - key: b
    desc: describe in less
    scopes: [all]
    command: sh
    args: ["-c", "kubectl --context $CONTEXT describe $RESOURCE $NAME -n $NAMESPACE | less"]
  - key: X
    desc: restart runner
    scopes: [ephemeralrunners]
    command: my-restart-runner
    args: [$NAMESPACE, $NAME]
    background: true
    confirm: true
```

| Field | Required | Meaning |
| --- | --- | --- |
| `key` | yes | the key that runs the plugin, see the key format below |
| `desc` | no | label shown in the footer, palette and help; defaults to the command's base name |
| `scopes` | yes | resources the plugin applies to: any string the resource picker resolves (`pods`, `deploy`, `Deployment`, `scaledobjects.keda.sh`) or `all` |
| `command` | yes | program to run; looked up on `$PATH` unless it is a path |
| `args` | no | arguments, one list item each; variables are expanded here |
| `background` | no | `true` runs the command detached and reports the result as a notice; the default opens it in the embedded terminal |
| `confirm` | no | `true` asks before running |

Variables available in `args` and exported to the command's environment:

| Variable | Value |
| --- | --- |
| `$NAME` | the selected row's name |
| `$NAMESPACE` | the row's namespace; empty for cluster-scoped resources |
| `$RESOURCE` | the resource key, `pods` or `deployments.apps` |
| `$CONTEXT` | the kubeconfig context in use |
| `$CLUSTER` | the kubeconfig cluster that context points at |
| `$KUBECONFIG` | the explicit `--kubeconfig` path, when ku was given one |

Both `$VAR` and `${VAR}` work. Any other `$NAME` in `args` is read from ku's
own environment, so `$HOME/bin/tool` expands as expected.

The command is executed directly, not through a shell. Put each argument in its
own list item and do not quote for a shell. To use a pipeline or redirection,
run `sh` with `-c` as the example above does.

Key format: a single key (`b`, `X`, `f5`, `space`) or modifiers plus a key
(`ctrl+o`, `alt+x`, `ctrl+shift+x`). Write a shifted letter as the uppercase
letter. Keys the table already uses are refused; ku reports the conflict as a
startup notice and skips that plugin. When two plugins share a key, the first
one wins and the second is skipped with a notice.

A matching plugin shows in the footer hints, in the command palette
(`Ctrl+K`), and in the help screen (`?`) under Plugins. With `background: true`
the command's first output line becomes the notice on success, and its stderr
becomes the notice on failure. Without it, the command runs in the same
embedded terminal as the shell feature: any key closes the panel when it exits,
and `Ctrl+\` detaches.

## Built-In Defaults

The default sidebar includes Pods, Deployments, StatefulSets, DaemonSets,
ReplicaSets, Jobs, CronJobs, Services, Ingresses, Endpoints, ConfigMaps,
Secrets, ServiceAccounts, PVCs, PVs, StorageClasses, Nodes, Namespaces, and
Events.

HPAs, KEDA ScaledObjects, and OpenTelemetry collectors are not in the default
menu. Add them to `sidebar:` when your cluster exposes them. A freshly seeded
config lists them as commented opt-in examples.
