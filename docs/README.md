# ku docs

A keyboard-driven Kubernetes TUI. These pages cover how to install and run it,
configure the sidebar and plugins, use the keys, and understand each feature.

- [Getting started](getting-started.md) - install, run, flags, themes, upgrade.
- [Configuration](configuration.md) - config file paths, sidebar examples, and plugins.
- [Keybindings](keybindings.md) - every key, by context.
- [Features](features.md) - cockpit, tables, config, YAML, logs, port-forward, shell, actions.

ku uses your default kubeconfig (`$KUBECONFIG`, then `~/.kube/config`) and the
current context unless you pass `--context`.

Created by [x.com/iamdothash](https://x.com/iamdothash).
