# Machines, parts and their config

Commands for creating machines and parts, changing a part's config from the CLI, and reaching into a running machine. Commands that take `--machine` or `--part` accept a name or an ID; a name that is not unique across the account also needs `--organization` and `--location`.

## Machines and parts

- `viam machines create --name=<name> --location=<location>` creates a machine with one part, its main part. `--organization` picks between locations of the same name in different organizations.
- `viam machines update --machine=<machine>` renames a machine (`--new-name`) or moves it to another location (`--new-location`).
- `viam machines delete --machine=<machine>` deletes the machine, all of its parts and their config. There is no undo.
- `viam machines part create --machine=<machine> --part-name=<name>` adds a part; `viam machines part list --machine=<machine>` lists them; `viam machines part delete` removes one.
- `viam locations list` lists the locations a machine can be created in or moved to.

A new machine or part exists only in the cloud until `viam-server` runs on a computer with that part's credentials. Until then its status is awaiting setup, and config saved to it is applied when it first connects.

## Reading a part's config

- `viam machines part config --part=<part>` prints the part's saved config JSON. `--at=<time>` prints the config that was in effect at that time.
- `viam machines part history --part=<part>` lists config saves with when and by whom; `--filter-by-email`, `--start`, `--end` and `--count` narrow it.
- `viam fragment list`, `viam fragment get --fragment=<id>` (`--version` for a revision or tag) and `viam fragment history --fragment=<id>` read fragments.

`part config` shows the part's own config: its `fragments` imports are listed, but the resources they provide are not expanded into it. MCP's `read_machine_config` returns both the part's config and each import resolved.

## Changing a part's config

- `viam machines part add-resource --part=<part> --name=<name> --model-name=<model triple>` adds a component or service, with `--resource-subtype` (such as `arm`) for a built-in API or `--api=<triple>` for a custom one. It is added with no attributes.
- `viam resource update --part=<part> --resource-name=<name> --config='<json or file>'` sets a resource's attributes. The JSON **replaces** all of them: attributes left out are deleted, and `{}` deletes every attribute. A full update starts from the current attributes in `part config`.
- `viam machines part remove-resource` removes a resource.
- `viam resource disable` and `viam resource enable` with one or more `--resource-name` keep a resource in the config but stop it from starting, or start it again.
- `viam machines part fragments add --fragment=<id>` and `remove` import or drop a fragment. The CLI has no command to change a fragment's contents; MCP's `update_fragment_config_item` does that.
- `viam machines part add-job --config='<json>'`, `update-job --name=<name> --config='<json>'` and `delete-job --name=<name>` manage scheduled jobs, which call a method on a resource at an interval. `update-job` changes only the fields given.
- `viam machines part add-trigger --config='<json>'` and `delete-trigger` manage triggers, which send a webhook or email on events such as a part coming online or data being synced.

`add-job` and `add-trigger` open a form when `--config` is left out, so without a terminal they need it.

Every change saves straight to the cloud config, and the part applies it within seconds when online. A save can succeed and still fail on the machine: a missing dependency, a bad attribute, or a model whose module is not in the config. The part's status and logs show it. When the config sets a `maintenance` sensor, the part holds new config until that sensor reports maintenance is allowed.

Editing a fragment changes every machine that imports it without a pinned version. With the MCP server connected, `retrieve_skill` with name `set-up-obstacles` and path `references/fragments.md` covers fragment imports, variables, prefixes and builder overrides in detail.

## Inside a running machine

These need the part online. `shell`, `cp` and `get-ftdc` also need a shell service (`rdk:service:shell`) in the part's config.

- `viam machines part shell --part=<part>` opens an interactive shell as the user `viam-server` runs as, often root. It needs a terminal.
- `viam machines part cp --part=<part> <source> <destination>` copies files; the `machine:` prefix marks a path on the machine (`machine:~/file`, or `machine:` alone for that user's home). `-r` copies directories and `-p` keeps modes and times. It overwrites existing files without asking.
- `viam machines part tunnel --part=<part> --local-port=<port> --destination-port=<port>` forwards a local port to a port on the machine. The machine allows only ports listed in `network.traffic_tunnel_endpoints` in its config.
- `viam machines part get-ftdc --part=<part>` downloads FTDC files, viam-server's recorded performance metrics.
- `viam machines part restart --part=<part>` restarts `viam-server` on the part. Everything the part runs stops while it restarts.
- `viam machines part motion print-config`, `print-status` and `get-pose --component=<name>` read the motion service's frame config and component poses.

## Logs and status

- `viam machines part logs --part=<part>` prints recent logs; `--errors` shows only errors, `--tail` follows new ones, and `--start`, `--end` and `--count` narrow them.
- `viam machines logs --machine=<machine>` reads logs across all parts, with `--levels`, `--keyword`, `--range` (such as `10m`) and `--format=json`.
- `viam machines status --machine=<machine>` and `viam machines part status --part=<part>` show each part's last access time.

Log reads count toward the organization's data egress.

## MCP equivalents

| CLI | MCP |
|---|---|
| `machines part config`, `fragment get` | `read_machine_config`, `read_fragment_config` |
| `machines part add-resource` + `resource update` | `add_machine_config_item` with attributes in one call |
| `resource update` | `update_machine_config_item`, which merges a patch instead of replacing all attributes |
| `machines part remove-resource` | `delete_machine_config_item` |
| (none) | `add_fragment_config_item`, `update_fragment_config_item`, `delete_fragment_config_item`, `create_fragment` |
| `machines create` | `create_machine` |
| `machines part logs`, `machines part status` | `read_machine_logs`, `read_part_status` |

The MCP config tools check a resource's model and attributes when saving and refuse what they can tell will not work. The CLI saves what it is given.

No MCP tool creates parts, deletes machines, manages jobs or triggers, opens a shell, copies files, tunnels or restarts a part.
