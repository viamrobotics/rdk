---
name: using-viam
description: How to use Viam to work with robots and other machines, through the Viam MCP tools or the viam CLI. Covers how a fleet is organized (organizations, locations, machines, parts), which interface to use for a job, logging in and finding IDs, and the common workflows (checking a machine's health, debugging a machine from its status and logs, adding or changing components and services, writing and publishing modules, reading from a component, exporting data) with the safety rules for each. Relevant whenever a user mentions Viam, viam-server, a Viam machine or robot, a module or fragment, or asks to run a viam command.
---

# Using Viam

Viam runs robots and other machines from the cloud. This skill is the map: how a fleet is organized, which interface to use, and the usual order of steps for common jobs. For exact parameters, trust the live MCP tool schemas and `viam <command> --help` over this file. If a command, flag or tool named here is missing or behaves differently, follow the live version and tell the user this skill may be out of date.

## How Viam is organized

- **Organization → location → machine → part.** A machine has one or more parts, and each part runs `viam-server`. Most actions act on a part, not the whole machine.
- **Components and services** are the resources in a part's config. An arm, camera or motor is a component; motion, vision or navigation is a service.
- **Modules** add resource models that are not built in. They come from the Viam registry or are written by the user.
- **Fragments** are reusable blocks of config that many machines import.
- **Config lives in the cloud** and is pushed to the machine. A saved change can still fail on the machine, so check the machine after every change.

## Pick an interface

Use what you have, in this order:

1. **Viam MCP tools connected** (tools such as `list_machines` and `read_machine_config`): use them first for fleet lists, config, status, logs and machine APIs. They return structured data and validate config edits.
2. **A shell where `viam version` works:** use the CLI for everything the MCP tools don't cover, and for everything when they aren't connected.
3. **Neither:** tell the user how to connect the Viam MCP server or install the CLI. The Viam SDKs are for code the user is writing, not for your own actions.

| Job | Use |
| --- | --- |
| Write, build, upload or reload a module | CLI. MCP has no equivalent. |
| Copy files to a machine, open a shell, tunnel a port | CLI |
| Bulk data export, datasets, data pipelines, training scripts | CLI |
| API keys, OAuth apps, billing | CLI |
| Read or edit machine and fragment config | MCP when connected; CLI otherwise |
| Read logs, part status, fleet lists | Either |
| Read from a component on a live machine | Either: `read_machine_api`, or `viam machines part run` |

## Log in and find IDs

**MCP.** The user logs in when they connect the server; every tool runs with their permissions. Find IDs with `list_organizations`, `list_organization_locations` and `list_machines` (which also matches a machine by fuzzy name). `read_machine_metadata` gives a part's organization, location and host details.

**CLI.**

- Run `viam whoami` first. It prints the logged-in email, or `key-<id>` for an API key.
- `viam login` opens a browser. Without one, run `viam login --no-browser` and give the user the URL it prints, or log in with an API key the user provides: `viam login api-key --key-id=<id> --key=<key>`.
- Never create API keys yourself (`viam organizations|locations|machines api-key create`). If there is no login and no key, ask the user to run `viam login` or to give you a key.
- Find IDs in order: `viam organizations list`, `viam locations list`, `viam machines list --organization=<org> --location=<location>`, then `viam machines part list --machine=<machine>`.
- `viam defaults set-org` and `viam defaults set-location` save flags you would otherwise repeat.

Never guess an ID, and never print a key or secret into the conversation.

## Common workflows

### Check a machine's health

- **MCP:** `list_machines` to find it, then `read_part_status`. Status is cloud connectivity only (online, offline, awaiting setup, last online); use `read_machine_api` for the state of a resource.
- **CLI:** `viam machines status --machine=<machine>` prints the machine and each of its parts with a "Last Access" time. A recent time means the part is connected; one minutes or more old means it is probably offline. `viam machines part status --part=<part>` prints the same for one part.

### Debug a machine that misbehaves

Work from the outside in: is it online, what do the logs say, and what is it configured to run.

1. **Status:** `read_part_status`, or `viam machines status --machine=<machine>`. An offline part can't run anything; check its network and that `viam-server` is running.
2. **Logs:** `read_machine_logs` (newest first, last 24 hours, filter by level), or `viam machines part logs --part=<part> --errors`.
3. **Config:** `read_machine_config` returns each part's saved config, including fragment imports and their overwrites.
4. **Resource docs:** `get_registry_item_information` explains a model from the registry; `read_viam_docs` searches the Viam docs.

### Add or change a component or service

- **MCP:** find the model with `search_configuration_blocks`, add it with `add_machine_config_item`, change it with `update_machine_config_item`, remove it with `delete_machine_config_item`. These save immediately.
- **CLI:** `viam machines part add-resource` and `viam machines part remove-resource`. `viam resource update --config` **replaces** every attribute of the resource; it does not merge.
- Then check the machine: status, then logs for errors from the new resource.

For creating machines and parts, fragments, jobs, triggers, shell, file copy, tunnels and restarts, read [references/machines.md](references/machines.md).

### Write, test and publish a module

CLI only.

1. `viam module generate` scaffolds a module. Without a terminal it needs `--name`, `--language`, `--public-namespace`, `--resource-subtype` and `--model-name`.
2. Test on a machine with `viam module reload --part-id=<part>` (builds in the cloud) or `viam module reload-local --part-id=<part>`.
3. Publish a versioned release with `viam module upload --version=<version> --platform=<platform> <archive>`, or `viam module build start --version=<version>` for a cloud build. Watch it with `viam module build logs --id=<build id> --wait`.

`module reload` is for testing only. A machine that should keep the module needs a published version.

For `meta.json` fields, platforms, versions and modules that fail to start, read [references/modules.md](references/modules.md).

### Read from a component

Read positions, images, sensor readings and resource state from a live machine.

- **MCP:** `read_machine_api`.
- **CLI:** `viam machines part run --part=<part> --method=<full method name> --data='<json>'` with a read method such as `GetPosition`, `GetImages` or `GetReadings`. Method names are full proto names, such as `viam.robot.v1.RobotService.GetMachineStatus`.

### Data and ML

- Export: `viam data export binary filter --destination=<dir> --org-ids=<org>` with filters, or `viam data export tabular`.
- Query: `viam data query tabular sql --org-id=<org> --sql='<query>'`.
- Datasets and training: `viam dataset ...` and `viam train ...`. The MCP server covers sequence datasets and custom training jobs. Training bills GPU time, so confirm before submitting.
- Deploying a trained model to a machine: use the `deploy-ml-model` skill.

For filters, tags, deletes, datasets, sequences, data pipelines and training, read [references/data.md](references/data.md).

## Using the CLI from an agent

- Output is text written for people. Only `viam machines logs --format=json` (machine-level logs, not `machines part logs`), `machines part run`, `machines part config` and `fragment get` print JSON. Match on known lines such as `created new machine with id <uuid>`.
- The exit code is non-zero on failure.
- Commands that open a form when run in a terminal (`module generate`, `machines part add-trigger`) need all their flags instead. `machines part cp` needs `--no-progress`.

## Safety

- **Confirm with the user first** before deleting anything or changing config on a machine that is in use.
  - Commands with no undo: `viam machines delete`, `viam machines part delete`, `viam data delete binary`, and `viam data delete tabular --delete-older-than-days=0`, which deletes **all** tabular data.
- `read_machine_logs` records billable data egress for the organization; narrow the request instead of reading logs repeatedly.
- A permission error is for a person to fix. Report it; don't look for a way around it.

## Other Viam skills

These need the Viam MCP server: they are served only over MCP, through `retrieve_skill`, and their steps use MCP tools. When a task matches one and the server is connected, open that skill:

- `deploy-ml-model`: deploy, update or remove an ML model on a machine.
- `set-up-obstacles`: add or change collision obstacles for motion planning.
- `resource-notes`: a resource's notes field (the builder Scratchpad).

