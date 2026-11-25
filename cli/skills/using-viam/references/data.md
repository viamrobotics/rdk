# Data, datasets and training

Machines capture data and sync it to the cloud, as one of two kinds:

- **Binary data:** images, point clouds and files. Each item has a binary data ID and can carry tags and bounding-box labels.
- **Tabular data:** sensor readings and other method results, stored as rows that SQL or MQL queries.

A **dataset** is a named set of binary data, or of sequences, that training reads. A **sequence** is a time range of one part's captured data across one or more resources. A **data pipeline** runs an MQL query on a schedule and stores the results where they can be queried.

## Query and export

- `viam data query tabular sql --sql='<query>'` and `viam data query tabular mql --mql='<json pipeline>'` (or `--mql-path`) query an organization's tabular data. MQL's `--data-source-type` is `standard`, `hot-storage` or `pipeline-sink`, and `pipeline-sink` also needs `--pipeline-id` or `--pipeline-name`.
- `viam data query binary filter` returns metadata for matching binary data, not the files.
- `viam data export binary filter` and `viam data export binary ids --binary-data-ids=<ids>` download files to `--destination`.
- `viam data export tabular` downloads one resource's readings: it needs `--part-id`, `--resource-name`, `--resource-subtype` and `--method`.

Query commands print to stdout unless `--destination` is set. Times are RFC 3339, such as `2026-11-01T00:00:00Z`.

Binary filters are `--org-ids`, `--location-ids`, `--machine-id`, `--part-id`, `--machine-name`, `--part-name`, `--component-type`, `--component-name`, `--method`, `--mime-types`, `--start`, `--end`, `--bbox-labels` and `--tags` (`tagged`, `untagged`, or a list). Every filter is optional, so a filter with only an org matches all of that org's binary data, which can be very large to export.

## Tags

- `viam data tag ids add|remove --tags=<tags> --binary-data-ids=<ids>` tags given items.
- `viam data tag filter add|remove --tags=<tags>` tags every item the filters match. Here `--tags` is the tags to add or remove, and the filter on existing tags is `--filter-tags`, not `--tags` as in the other binary commands.

`viam data query binary filter` with the same filters shows what a tag-by-filter will change.

## Delete

- `viam data delete binary` needs `--org-ids`, `--start` and `--end`, and deletes every item that matches them and any other filters.
- `viam data delete tabular --delete-older-than-days=<n>` deletes the org's tabular data older than n days; `0` deletes **all** of it.

Neither has an undo or a dry run.

## Datasets

- `viam dataset list`, `create --name=<name> --type=<binary|sequence>`, `rename`, `delete` and `merge --dataset-ids=<ids> --name=<name>` manage datasets. A merge makes a new dataset and leaves the originals.
- `viam dataset data add|remove ids` and `viam dataset data add|remove filter` change a binary dataset's contents; `viam dataset sequence add|remove --sequence-ids=<ids>` a sequence dataset's.
- `viam dataset export --dataset-id=<id>` downloads a binary dataset as images plus `dataset.jsonl`, and a sequence dataset as a Parquet zip. `--only-jsonl` and `--only-parquet` skip the large files.

Deleting a dataset, or removing items from one, keeps the data itself; only its membership goes. A binary dataset takes only binary data and a sequence dataset only sequences.

`viam data sequence create --part-id=<part> --resources=<resource>:<method> --start=<time> --end=<time>` makes a sequence; `get`, `update` and `delete` take its ID. `update` changes only the fields passed, but `--resources` and `--tags` each replace the whole list, and changing resources or times changes which captured data the sequence holds.

## Data pipelines

- `viam datapipelines list`, `describe --id=<id>`, `rename`, `enable`, `disable` and `delete` manage pipelines; `describe` also shows recent runs.
- `viam datapipelines create` needs `--name`, `--schedule` (a cron expression), `--mql` (or `--mql-path`) and `--data-source-type` (`standard` or `hot-storage`).

`--enable-backfill` runs a new pipeline over all of the org's past data as well as new data. A pipeline's results are queried with `viam data query tabular mql --data-source-type=pipeline-sink --pipeline-name=<name>`.

## Training

Training runs in the cloud on a dataset and bills the organization for GPU time, including the time a job used before it was cancelled.

- `viam train submit managed` trains with a Viam-managed script. It needs `--dataset-id`, `--model-org-id`, `--model-name`, `--model-version`, `--model-type` (`single_label_classification`, `multi_label_classification`, `object_detection`, `robot_policy` or `unspecified`), `--model-framework` (`tflite`, `tensorflow`, `pytorch`, `onnx`, `registry` or `unspecified`) and `--model-labels`.
- `viam train submit custom from-registry` trains with a published script (`--script-name`, `--version`) in a container from `viam train containers list`.
- `viam train submit custom with-upload` uploads a local script and trains with it in one step. `viam training-script upload` publishes a script without training, and `viam training-script test-local` runs one locally in Docker.
- `viam train list`, `get --job-id=<id>`, `logs --job-id=<id>` and `cancel --job-id=<id>` follow a job.

A finished job produces an ML model in the registry. The `deploy-ml-model` skill covers putting it on a machine.

## MCP tools

| Area | Tools |
|---|---|
| Datasets | `list_datasets`, `create_dataset`, `delete_dataset` |
| Sequences | `list_sequences`, `read_sequence`, `update_sequence`, `delete_sequences`, `add_sequences_to_dataset`, `remove_sequences_from_dataset` |
| Custom training | `list_training_scripts`, `list_training_containers`, `submit_training_job`, `list_training_jobs`, `read_training_job`, `read_training_job_logs`, `cancel_training_job` |

- No MCP tool queries or exports data, tags or deletes binary data, adds binary data to a dataset, manages pipelines, or submits a managed training job. Those are CLI only.
- `submit_training_job` starts a new billed job on every call, so a retry after a timeout can start a second one; `list_training_jobs` shows whether the first started.
- `read_training_job_logs` adds a data egress charge for every page.
- `add_sequences_to_dataset` and `remove_sequences_from_dataset` are all or none. `delete_sequences` tries each one and can succeed while listing some in `failed`.
