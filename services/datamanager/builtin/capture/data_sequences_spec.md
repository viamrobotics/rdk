# Tech Spec: Data Sequences

**Author:** Gloria Cai
**Date:** 2026-04-30
**Status:** Draft

---

## Background & Motivation

Today, querying binary and tabular data requires constructing filters ad hoc on every request — specifying time ranges, part IDs, component names, and method names each time. There is no way to define a reusable, named slice of data that spans both binary and tabular stores.

**Data sequences** introduce a persistent, reusable time-range filter scoped to an organization. A sequence captures a time window and a set of resource constraints (part, component, method), and can be queried to retrieve all binary and tabular data matching those constraints. Sequences can be tagged for organization and discovery.

The primary motivation is VLA (Vision-Language-Action) training data: a sequence groups camera images and joint position readings captured together over a bounded time window, so they can be reviewed and exported as a unit.

---

## Concept Definition

A **sequence** is a persisted org-scoped record that defines:

- A **time range** (`start_at` → `end_at`) — the window of data the sequence covers
- A **list of resource filters** — the parts/components/methods whose data the sequence includes
- **Tags** — for organizing and filtering sequences

A sequence is not a data copy. It is a pointer — a stored filter that gates queries against existing binary and tabular data. Querying a sequence returns all data records that fall within the time range and match any of the resource filters (union semantics).

### SequenceResourceFilter

Each resource filter requires all three fields:

- `part_id` — required
- `component_name` — required
- `method_name` — required

All three fields must be specified. Partial filters (e.g. part_id only) are rejected at validation time. This constraint ensures the data type (binary vs tabular) is always deterministically derivable from the resource filter — see **Data Type Routing** below.

### Resources List Semantics

`repeated SequenceResourceFilter resources` uses **union semantics**: a data record is included in the sequence if it falls within the time range AND matches any one of the resource filters (`$or` across filters, `$and` with time range).

At least one resource filter is required.

### Data Type Routing

The data type (binary vs tabular) for each resource is derived from `method_name` at retrieval time using `DataTypeForMethod` — a Go map ported from `ui/src/lib/rdk/data-capture.ts` that lives in `domains/datamanagement/sequences/`. This means:

- The caller never specifies `data_type` explicitly
- A sequence may contain a mix of binary resources (e.g. `camera/GetImages`) and tabular resources (e.g. `arm/JointPositions`)
- At retrieval time, resources are bucketed by type and each bucket is queried against the appropriate store
- If a sequence has no resources of a given type, the corresponding store is never queried

---

## Proto API

### Messages

```protobuf
// SequenceResourceFilter identifies a data source within a sequence.
// All three fields are required.
message SequenceResourceFilter {
  string part_id = 1;
  string component_name = 2;
  string method_name = 3;
}

message Sequence {
  // Immutable metadata.
  string id = 1;
  string organization_id = 2;
  google.protobuf.Timestamp created_at = 4;
  google.protobuf.Timestamp updated_at = 5;

  // Mutable fields (updatable via field_mask).
  repeated string sequence_tags = 3;
  google.protobuf.Timestamp start_at = 6;
  google.protobuf.Timestamp end_at = 7;

  // Data matching any of these resource filters within the time range
  // is included in the sequence. Union (OR) semantics across filters.
  repeated SequenceResourceFilter resources = 8;
}

// CRUD

message CreateSequenceRequest {
  string organization_id = 1;
  repeated SequenceResourceFilter resources = 2;
  repeated string sequence_tags = 3;
  google.protobuf.Timestamp start_at = 4;
  google.protobuf.Timestamp end_at = 5;
}

message CreateSequenceResponse {
  string id = 1;
}

message GetSequenceRequest {
  string id = 1;
}

message GetSequenceResponse {
  Sequence sequence = 1;
}

// field_mask is required. Only fields listed will be updated; others are left unchanged.
// Mutable fields: start_at, end_at, sequence_tags, filters.
// Note: the repeated resources field is named `filters` in this request.
message UpdateSequenceRequest {
  string id = 1;
  repeated SequenceResourceFilter filters = 2;
  repeated string sequence_tags = 3;
  google.protobuf.Timestamp start_at = 4;
  google.protobuf.Timestamp end_at = 5;
  google.protobuf.FieldMask field_mask = 6; // required
}

message UpdateSequenceResponse {}

message DeleteSequenceRequest {
  string id = 1;
}

message DeleteSequenceResponse {}

message ListSequencesRequest {
  string organization_id = 1;
  string page_token = 2;
}

message ListSequencesResponse {
  repeated Sequence sequences = 1;
  string next_page_token = 2;
  int64 total_count = 3;
}
```

### RPCs (Public — DataServiceServer)

```protobuf
rpc CreateSequence(CreateSequenceRequest) returns (CreateSequenceResponse);
rpc GetSequence(GetSequenceRequest) returns (GetSequenceResponse);
rpc UpdateSequence(UpdateSequenceRequest) returns (UpdateSequenceResponse);
rpc DeleteSequence(DeleteSequenceRequest) returns (DeleteSequenceResponse);
rpc ListSequences(ListSequencesRequest) returns (ListSequencesResponse);
```

### RPCs (Internal — InternalDataServiceServer)

For UI use only. Not part of the public API surface. Binary and tabular data are fetched separately — each section of the UI paginates independently.

```protobuf
rpc GetSequenceBinaryData(GetSequenceBinaryDataRequest) returns (GetSequenceBinaryDataResponse);
rpc GetSequenceTabularData(GetSequenceTabularDataRequest) returns (GetSequenceTabularDataResponse);
```

```protobuf
message GetSequenceBinaryDataRequest {
  string sequence_id = 1;
  string page_token = 2;
  int32 page_size = 3;
}

message GetSequenceBinaryDataResponse {
  repeated BinaryData data = 1;
  // next_page_token is the _id hex of the last returned document.
  // Pass back as page_token on the next request to continue pagination.
  // Empty string means no more pages.
  string next_page_token = 2;
  uint64 count = 3;
}

message GetSequenceTabularDataRequest {
  string sequence_id = 1;
  string page_token = 2;
  int32 page_size = 3;
}

message GetSequenceTabularDataResponse {
  repeated google.protobuf.Struct data = 1;
  string next_page_token = 2;
  uint64 count = 3;
}
```

---

## Storage

### Database & Collection

| Field          | Value                               |
| -------------- | ----------------------------------- |
| MongoDB client | `MainMongoClient`                   |
| Database       | `sequencesDB`                       |
| Collection     | `sequences`                         |
| Package        | `domains/datamanagement/sequences/` |

Sequences are org-scoped metadata, not data — `MainMongoClient` is the right client, consistent with orgs, locations, robots, and datasets.

### Go Schema

```go
// domains/datamanagement/sequences/sequence_db.go

type Sequence struct {
    ID             string                   `bson:"_id"`
    OrganizationID string                   `bson:"organization_id"`
    Tags           []string                 `bson:"tags"`
    CreatedAt      time.Time                `bson:"created_at"`
    UpdatedAt      time.Time                `bson:"updated_at"`
    StartAt        time.Time                `bson:"start_at"`
    EndAt          time.Time                `bson:"end_at"`
    Resources      []SequenceResourceFilter `bson:"resources"`
}

type SequenceResourceFilter struct {
    PartID        string `bson:"part_id"`
    ComponentName string `bson:"component_name"`
    MethodName    string `bson:"method_name"`
}
```

### Indexes

```go
var sequenceIndexes = []mongo.IndexModel{
    {
        // ListSequences sorted by recency
        Keys: bson.D{
            {Key: "organization_id", Value: 1},
            {Key: "created_at", Value: -1},
        },
    },
    {
        // Tag filtering on ListSequences
        Keys: bson.D{
            {Key: "organization_id", Value: 1},
            {Key: "tags", Value: 1},
        },
    },
}
```

---

## Authorization

### CRUD Operations

| Operation      | Permission                                                                  |
| -------------- | --------------------------------------------------------------------------- |
| CreateSequence | `PermissionWriteOrganizationDataManagement` on `organization_id`            |
| GetSequence    | `PermissionReadOrganizationDataManagement` on sequence's `organization_id`  |
| UpdateSequence | `PermissionWriteOrganizationDataManagement` on sequence's `organization_id` |
| DeleteSequence | `PermissionWriteOrganizationDataManagement` on sequence's `organization_id` |
| ListSequences  | `PermissionReadOrganizationDataManagement` on `organization_id`             |

Fetch the sequence first, then validate the caller has the required permission on the sequence's `organization_id`. For `CreateSequence`, validate on the request's `organization_id` directly.

`part_id` values in resources are not validated to exist at creation time. If a part is deleted after a sequence is created, data retrieval for that part returns empty results — consistent with how filters work elsewhere.

### Data Retrieval Authorization

#### Binary (`GetSequenceBinaryData`)

Auth follows `constructBaseQueryFromAuth` in `build_query_from_filter.go`: builds a broad `$or` across authorized org/location/robot IDs, naturally handling shared locations. The sequence's time range and resource filters are AND'd on top:

```go
baseAuthQuery := constructBaseQueryFromAuth(state.MyAuthorizations)

resourceClauses := make(bson.A, 0, len(seq.Resources))
for _, r := range seq.Resources {
    resourceClauses = append(resourceClauses, bson.M{
        "part_id":        r.PartID,
        "component_name": r.ComponentName,
        "method_name":    r.MethodName,
    })
}

query := bson.M{"$and": bson.A{
    baseAuthQuery,
    bson.M{"interval.start": bson.M{"$lte": seq.EndAt}},
    bson.M{"interval.end":   bson.M{"$gte": seq.StartAt}},
    bson.M{"$or": resourceClauses},
}}
```

#### Tabular (`GetSequenceTabularData`)

Routes through `tabularDataByQuery` → `createCursor` → `generateRoleAuthViewAndSharedLocationIds`. Shared location handling is automatic.

```go
matchStage, _ := bson.Marshal(bson.D{{"$match", buildSequenceMatchFilter(seq)}})
srv.dataServer.tabularDataByQuery(ctx, adf.ADFTarget{OrgID: seq.OrganizationID},
    "", [][]byte{matchStage}, nil, dataSource, logger)
```

Both handlers call `DataTypeForMethod` on each resource to bucket them by type before building the query. If a sequence has no resources of the requested type, the handler returns empty without querying the store.

---

## Query Construction

Since `component_name` and `method_name` are always present, resource clauses are always fully specified — no conditional field inclusion.

### Binary — `BuildSequenceQuery`

```go
func BuildSequenceQuery(seq *sequences.Sequence) bson.M {
    resourceClauses := make(bson.A, 0, len(seq.Resources))
    for _, r := range seq.Resources {
        resourceClauses = append(resourceClauses, bson.M{
            "part_id":        r.PartID,
            "component_name": r.ComponentName,
            "method_name":    r.MethodName,
        })
    }
    return bson.M{
        "interval.start": bson.M{"$lte": seq.EndAt},
        "interval.end":   bson.M{"$gte": seq.StartAt},
        "$or":            resourceClauses,
    }
}
```

### Tabular — `buildSequenceMatchFilter`

```go
func buildSequenceMatchFilter(seq *sequences.Sequence) bson.D {
    resourceClauses := make(bson.A, 0, len(seq.Resources))
    for _, r := range seq.Resources {
        resourceClauses = append(resourceClauses, bson.D{
            {Key: "part_id",        Value: r.PartID},
            {Key: "component_name", Value: r.ComponentName},
            {Key: "method_name",    Value: r.MethodName},
        })
    }
    return bson.D{
        {Key: "interval.start", Value: bson.D{{Key: "$lte", Value: seq.EndAt}}},
        {Key: "interval.end",   Value: bson.D{{Key: "$gte", Value: seq.StartAt}}},
        {Key: "$or",            Value: resourceClauses},
    }
}
```

---

## UI

### List View (`/data/sequences`)

A table of sequences for the org. Each row shows:

| Column       | Value                                    |
| ------------ | ---------------------------------------- |
| Time range   | `start_at → end_at` (primary identifier) |
| Tags         | Rendered as badges                       |
| Created date | Formatted date                           |
| ID           | Truncated, copy-on-click                 |

Sorted by `created_at` descending (matches API default). Clicking a row navigates to `/data/sequences/:id`. Paginated with a "Load more" button at the bottom.

Creating sequences is out of scope for v1 UI.

**Route registration:** add `"data/sequences"` to `getRouteNames()` in `ui/router.go`.

### Detail View (`/data/sequences/:id`)

#### Data Loading

On page load, the UI calls `GetSequence(id)` to get the sequence header (time range, tags, resources), then fires `GetSequenceBinaryData` and `GetSequenceTabularData` in parallel — no sequential dependency, no `GetSequenceSummary` needed.

```
GetSequence(id)                                              → sequence metadata
GetSequenceBinaryData(sequence_id, page_token, page_size)   → [BinaryData...], next_page_token
GetSequenceTabularData(sequence_id, page_token, page_size)  → [Struct...],     next_page_token
```

The backend short-circuits both retrieval calls immediately if the sequence has no resources of that type — the empty call costs nothing. The UI hides a section when its first-page response is empty.

**Loading strategy:**

- `GetSequence` fires first; `GetSequenceBinaryData` and `GetSequenceTabularData` fire in parallel once `sequence_id` is known (which it is from the URL — they can all fire simultaneously on page load)
- First page of each renders as soon as it arrives
- Subsequent pages lazy-load via intersection observer on the last item in each section
- Default `page_size`: 50

#### Layout

Top/bottom split — each half is `50vh` with `overflow-y: auto`, scrolling independently.

**Top half — Image grid**

Binary records displayed as thumbnails using `BinaryMetadata.uri` as `<img src>`, sorted chronologically by `interval.start`. Hidden if first binary response is empty.

**Bottom half — Readings table**

Tabular records displayed as rows. Columns derived from the keys of the first record. Sorted by time. Hidden if first tabular response is empty.

If both sections are hidden (sequence has no data in range), show a single empty state: "No data found for this sequence's time range and resources."

No interaction between sections in v1. Temporal alignment (nearest-timestamp association between a clicked image and its closest readings row) is a v2 feature — the data needed to implement it client-side is already present.

---

## Wiring

### `DataServer`

Add `sequencesDB sequences.SequencesDB` for CRUD RPCs.

### `InternalDataServer`

Add `sequencesDB sequences.SequencesDB` for retrieval RPCs.

---

## Sequence Creation from RDK (Capture Control Sensor)

So far the spec describes how sequences are stored, queried, and surfaced in the UI. This section covers how RDK robots can **create** sequences at capture time, via the existing capture control sensor.

### Motivation

For VLA training, the natural session boundary aligns with capture enable/disable: an operator enables capture, performs a demo, disables capture. The capture control sensor already drives those transitions. A sequence should be created automatically covering exactly that window, so the operator does not need a separate UI step. v1 UI does not support sequence creation, so this is the only path to create sequences from a robot.

### Sensor Reading Shape

The capture control sensor exposes **two separate keys** in its readings map: one for capture configs (existing) and one for active sequence recordings (new). Each is independently configurable.

```json
{
  "capture_configs": [
    {"resource_name": "camera-1", "method": "GetImages",      "capture_frequency_hz": 10},
    {"resource_name": "arm-1",    "method": "JointPositions", "capture_frequency_hz": 5}
  ],
  "sequences": [
    {
      "sequence_tags": ["walking"],
      "resources": [
        {"resource_name": "camera-1", "method": "GetImages"},
        {"resource_name": "arm-1",    "method": "JointPositions"}
      ]
    }
  ]
}
```

**Capture configs** are unchanged — same shape and semantics as today.

**Sequences** is a list of currently-active sequence recordings. While an entry is present in the list, that recording is open. When the entry disappears (or `sequences` is empty/missing), the recording closes.

Each sequence entry contains:

- `sequence_tags []string` — tags to attach to the resulting sequence. Optional.
- `resources []SequenceResourceReading` — the resources that should be included in the sequence. Each resource is `{resource_name, method}`, matching the shape used in `capture_configs` (consistent ergonomics for sensor authors who can share a constant between both arrays).

Resources listed in a `sequences` entry are expected to also be enabled in `capture_configs` (otherwise no data will be captured for them during the recording). The data manager logs a warning at parse time if a sequence references a resource that is not currently enabled — fixable config bug.

### Configuration

`CaptureControlSensorConfig` gains an optional `SequencesKey` field:

```go
type CaptureControlSensorConfig struct {
    Name         string `json:"name"`
    Key          string `json:"key"`                     // existing — capture configs key
    SequencesKey string `json:"sequences_key,omitempty"` // new — sequences key, optional
}
```

If `SequencesKey` is unset or empty, sequence parsing is skipped entirely (feature off). Opting in is one config line:

```json
"capture_control_sensor": {
  "name":          "my-sensor",
  "key":           "capture_configs",
  "sequences_key": "sequences"
}
```

### State Tracking

The data manager tracks open recordings by a **content-hash identity** — no per-resource transition state machine is needed. State lives on `Capture`:

```go
// openRecordings maps a stable content hash (sha256 of sorted resources + sorted tags)
// to the recording's state.
openRecordings map[string]*openSequenceRecording

type openSequenceRecording struct {
    hash      string                    // content hash, also the pending file's uuid component
    startAt   time.Time                 // first time we saw this entry
    resources []SequenceResourceReading // frozen at open time
    tags      []string                  // frozen at open time
}
```

Each poll tick (after applying capture configs, still under `collectorsMu`):

1. **Build the set of hashes from the new `sequences` array.** For each entry, compute `hash = sha256(canonicalize(resources, tags))`.
2. **Opens:** for each new hash not present in `openRecordings`, create an `openSequenceRecording` with `startAt = now`, register it.
3. **Closes:** for each hash present in `openRecordings` but NOT in the new set, close the recording: write the pending sequence JSON to `<capture_dir>/pending_sequences/<hash>-<endAt-unix-nanos>.json` with `start_at = rec.startAt`, `end_at = now`, `resources = rec.resources`, `sequence_tags = rec.tags`. Delete from `openRecordings`.

That's it — no per-resource diffing, no transition classification, no ordering constraints between opens and closes (they're disjoint sets by definition).

Content-hash identity means two back-to-back recordings with **identical** contents will be tracked as the **same** recording if the sensor doesn't have a closed-tick between them. To produce two separate sequences with the same resources/tags, the sensor must emit `sequences: []` (or omit it) for at least one tick between them. This is intuitive: from the data manager's view, "still here" = "still recording."

### Failure-Mode Behavior

- **Sensor read error** → poller passes `nil` to `SetCaptureConfigs`. Sequences parsing is also skipped → all open recordings close. Matches existing behavior of reverting capture overrides on sensor error. Acceptable for v1; operators should ensure their sensor module is reliable.
- **Parse error on `sequences`** → log warning, skip the sequence section (do not close existing recordings). Capture-config parsing is independent so it still applies. This is gentler than treating a parse error as "stop everything" — sequence parse errors are recoverable.
- **Single-tick blip** (sensor returns a malformed reading for one tick, then recovers) → because parse errors don't force-close, the recording survives a transient blip. Only a sustained absence closes it.
- **Sequence references a resource not in `capture_configs`** → log warning at parse time, accept the sequence anyway (data retrieval will simply return empty for that resource). Don't reject — the sensor might be in the middle of a config transition.

### Filling in `part_id` and `organization_id`

The sensor only specifies `resource_name` and `method` per resource. The data manager fills in:

- `part_id` — from `cloud.ConnectionService.AcquireConnection`, which already returns the machine's part ID.
- `organization_id` — from `cloud.ConnectionService.PrimaryOrgID()`, a new method added to the `ConnectionService` interface. Returns `""` for robots that are not cloud managed (in which case sequence creation is skipped, since the upload would fail anyway).

This keeps the sensor module portable across machines and orgs — no hard-coded IDs in the sensor module. See `internal/cloud/service.go` and `internal/testutils/inject/cloud_connection.go` for the interface change.

### Durability Model

Following the existing `.prog` → `.capture` pattern used by capture files (see `data/capture_buffer.go`):

- **Active recordings** (open, not yet closed) live in memory only. Lost on machine restart. This matches how partial `.prog` files are abandoned today and is acceptable because VLA training is supervised — the operator will notice and re-run a demo.
- **Pending sequences** (closed but not yet uploaded) are written to `<capture_dir>/pending_sequences/<uuid>.json` synchronously when the recording closes. They survive restarts and indefinite offline periods.

### Pending Sequence File Schema

```json
{
  "start_at":        "2026-04-30T12:00:00Z",
  "end_at":          "2026-04-30T12:00:30Z",
  "organization_id": "org-uuid",
  "part_id":         "part-uuid",
  "resources": [
    {"component_name": "camera-1", "method_name": "GetImages"},
    {"component_name": "arm-1",    "method_name": "JointPositions"}
  ],
  "sequence_tags": []
}
```

`organization_id` and `part_id` are resolved at close time (when the file is written) so machine reconfiguration mid-recording cannot cause mismatches.

### Retry Worker

A background goroutine in `builtIn` periodically scans `<capture_dir>/pending_sequences/`:

1. Acquire a cloud connection via `cloudConnSvc.AcquireConnection` (skip the tick if unavailable).
2. For each pending file, call `DataServiceClient.CreateSequence` with the decoded request.
3. On success: delete the file.
4. On transient error (`Unavailable`, etc.): leave the file, retry on next tick with exponential backoff (mirror the existing sync retry: 60s base, doubling up to 1hr).
5. On permanent error (e.g., `InvalidArgument`): move the file to `<capture_dir>/pending_sequences/failed/` so it doesn't block the queue and an operator can inspect it.

The retry worker acquires its own connection per drain rather than sharing sync's connection — simpler lifecycle, no coupling.

---

## Implementation Plan

PRs 2a and 3 can be worked in parallel. PR 2b follows 2a. PR 4 is blocked on PR 2b (but can ship images-only after PR 2a if needed). PR 5 (RDK) is independent and can be worked in parallel with PR 2a once `CreateSequence` is shipped from PR 1.

```
PR 1 (done) ──► PR 2a (binary API) ──► PR 2b (tabular API) ──► PR 4 (detail view)
           ├──► PR 3 (list view)
           └──► PR 5 (RDK capture control sensor)
```

---

### PR 1 — Storage + CRUD (current branch)

**Delivers:** `SequencesDB`, five public CRUD RPCs, wiring into `DataServer` and all `DatabasesFor*` structs.

**Files:**

- `domains/datamanagement/sequences/sequence_db.go`
- `domains/datamanagement/sequences.go`
- `domains/datamanagement/sequences_test.go`
- `domains/datamanagement/server.go`
- `domains/datamanagement/shared_test.go`
- `server/dependencies/databases.go`

---

### PR 2a — Internal API: Binary

**Delivers:** Shared infrastructure + `GetSequenceBinaryData` on `InternalDataServer`.

**Files to add/modify:**

- `pb/datamanagement/internalapi/v1/data.proto` — add both RPCs and all messages (do proto for both handlers here so 2b is just Go), regenerate
- `domains/datamanagement/sequences/sequence_db.go` — add `DataType` enum + `DataTypeForMethod` (port `CaptureMethodDataTypes` from `ui/src/lib/rdk/data-capture.ts`)
- `domains/datamanagement/internal_data_service.go` — add `sequencesDB sequences.SequencesDB` field, update `NewInternalDataServer`
- `domains/datamanagement/internal_get_sequence_binary_data.go` — new file
- `domains/datamanagement/internal_helpers.go` — register `GetSequenceBinaryData` method name

**Binary handler steps:**

1. Validate `sequence_id`
2. `sequencesDB.GetSequence` → 404 on miss
3. Auth: `validateOrgReadPermissions` on `seq.OrganizationID`
4. Bucket resources: filter to `DataTypeForMethod(r.MethodName) == DataTypeBinary`
5. If empty → return empty immediately, no binary store query
6. Build query: AND `constructBaseQueryFromAuth` with `BuildSequenceQuery` (tuple `$or` across binary resources + time range)
7. Pass as `prebuiltQuery` into `QueryBinaryData` — reuses cursor pagination, result building, annotations, and billing
8. Return `res.lastHex` as `next_page_token`, `res.binaryData` as `data`

**`QueryBinaryData` change (prerequisite):**

Add `prebuiltQuery *bson.M` to `BinaryDataQueryOptions`. In `QueryBinaryData`, short-circuit the filter-build block when it is set:

```go
if opts.prebuiltQuery != nil {
    query = opts.prebuiltQuery
} else if opts.useAuthForFilter {
    query, err = buildQueryFromFilterAndAuth(ctx, srv.appDB, opts.filter)
} else {
    query, err = buildQueryFromFilter(ctx, srv.appDB, opts.filter, dataActionRead, false)
}
```

Everything else in `QueryBinaryData` (cursor pagination via `last` hex, `buildQueryIntervals`, `FindBinaryData`, result building, billing) runs unchanged.

**Things to consider:**

- `constructBaseQueryFromAuth` is in `build_query_from_filter.go` — look at how `AddBinaryDataToDatasetByFilter` calls it.
- The `$or` across resource tuples is intentional — do not flatten to per-field `$in` queries, which would return a cartesian product superset.
- Billing events fire through `QueryBinaryData` — consistent with `GetBinaryDataByExclusionFilter` (the other internal binary handler) and the public images page.
- The `next_page_token` in `GetSequenceBinaryDataResponse` is the same `_id` hex cursor used by `BinaryDataByFilter` — the UI passes it back as-is on the next request.
- Add proto for both `GetSequenceBinaryData` and `GetSequenceTabularData` in this PR so PR 2b is purely a Go handler.

---

### PR 2b — Internal API: Tabular (blocked on PR 2a)

**Delivers:** `GetSequenceTabularData` on `InternalDataServer`.

**Files to add/modify:**

- `domains/datamanagement/internal_get_sequence_tabular_data.go` — new file
- `domains/datamanagement/internal_helpers.go` — register `GetSequenceTabularData` method name

**Tabular handler steps:**

1. Validate `sequence_id`
2. `sequencesDB.GetSequence` → 404 on miss
3. Auth: `validateOrgReadPermissions` on `seq.OrganizationID`
4. Bucket resources: filter to `DataTypeForMethod(r.MethodName) == DataTypeTabular`
5. If empty → return empty immediately, no ADF query
6. Marshal `buildSequenceTabularMatchFilter` as a `$match` stage
7. Build pipeline: `[$match, $sort, $skip, $limit]`
   - `$sort`: `{interval.start: 1}` — ascending by capture time; required for deterministic `$skip` pagination
   - `$skip`: numeric offset decoded from `page_token` (empty → 0)
   - `$limit`: `page_size` (default 50)
8. Call `srv.dataServer.tabularDataByQuery(ctx, adf.ADFTarget{OrgID: seq.OrganizationID}, "", [][]byte{matchStage, sortStage, skipStage, limitStage}, nil, nil, logger)`
9. Convert `[][]byte` results → `[]*structpb.Struct` via `datasync.BSONToStructPB`
10. If `len(results) == pageSize`, set `next_page_token = skip + pageSize`; otherwise empty string

**Things to consider:**

- `tabularDataByQuery` returns `[][]byte` (serialized BSON). Use `datasync.BSONToStructPB` for conversion — look at how existing tabular handlers do this before writing your own.
- Pagination is numeric skip/offset (not a cursor like binary). `page_token` encodes the skip count as a base-10 integer string.
- The `$sort` before `$skip` is required — without it ADF results are non-deterministic and `$skip` can return duplicates or miss records across pages.
- The `len(results) == pageSize` heuristic for `next_page_token` means one extra empty request if the total is exactly divisible by page size — acceptable for v1.
- ADF queries are slow. The empty short-circuit in step 5 is important — do not skip it.

---

### PR 3 — List View (unblocked after PR 1)

**Delivers:** `/data/sequences` page showing a table of sequences with "Load more" pagination.

**Files to add/modify:**

- `ui/router.go` — add `"data/sequences"` to `getRouteNames()`
- Web client — add `listSequences(orgID, pageToken)` call
- `ui/src/routes/data/sequences/+page.svelte` — new file (Svelte 5 runes)

**Page structure:**

- On mount: call `listSequences(orgID)` where org ID comes from app state (same pattern as other `/data/*` pages)
- Table columns: time range (`start_at → end_at`), tags (badges), created date, ID (truncated, copy-on-click)
- "Load more" button at bottom → append next page, update `page_token`
- Empty state: "No sequences found"
- Loading state: skeleton rows

**Things to consider:**

- Look at how other `/data/*` pages (`data/all`, `data/datasets`) get org ID from app state — use the same pattern.
- Check whether a badge component already exists in `ui/` for tags — reuse if so.
- Time range formatting: use the same date formatting utilities already used in binary/tabular data views.
- "Load more" is intentional over infinite scroll — simpler to implement and browser back-button preserves scroll position.

---

### PR 4 — Detail View (blocked on PR 2)

**Delivers:** `/data/sequences/:id` page with top/bottom split — image grid and readings table.

**Files to add/modify:**

- Web client — add `getSequenceBinaryData(sequenceID, pageToken, pageSize)` and `getSequenceTabularData(sequenceID, pageToken, pageSize)`
- `ui/src/routes/data/sequences/[id]/+page.svelte` — new file
- `ui/src/routes/data/sequences/[id]/SequenceBinaryGrid.svelte` — new file
- `ui/src/routes/data/sequences/[id]/SequenceTabularTable.svelte` — new file

**Page structure:**

- On mount: fire `GetSequence(id)`, `GetSequenceBinaryData(id)`, `GetSequenceTabularData(id)` simultaneously (sequence ID is in the URL)
- Header: time range, tags
- Layout: two `50vh` containers with `overflow-y: auto`, stacked top/bottom

**`SequenceBinaryGrid`:**

- Renders thumbnails as `<img src={metadata.uri}>` — verify the exact field name on `BinaryMetadata` in the proto before building
- Sorted by `interval.start`
- Intersection observer on last thumbnail → load next page, append
- Hidden when first response is empty

**`SequenceTabularTable`:**

- Columns derived from keys of the first record (all records assumed to have the same schema)
- Intersection observer on last row → load next page, append
- Hidden when first response is empty

**States:**

- Loading: skeleton in each half while first page is in-flight
- Per-section error: error banner with retry button, other section still renders
- Both empty: single centered message — "No data found for this sequence's time range and resources"

**Things to consider:**

- Verify `BinaryMetadata.uri` is the right field for the image URL before building the grid — check `BinaryMetadata` in the proto or look at how the existing binary data view renders images.
- Tabular columns from first record keys will be unordered (JS object keys). If ordering matters, sort alphabetically or pin a `time` column first.
- Both retrieval calls fire on mount without waiting for `GetSequence` to resolve — `sequence_id` is already in the URL params.
- `page_size: 50` for both sections. Images are larger payloads — consider whether 50 is too many for the binary grid on first load.

---

### PR 5 — RDK Capture Control Sensor Integration (blocked on PR 1)

**Delivers:** Automatic sequence creation driven by a new `sequences` key in the capture control sensor's readings. Pending-sequence persistence + retry worker for offline durability. Exposes `PrimaryOrgID()` on `cloud.ConnectionService` so the data manager can fill in `organization_id` without the sensor needing to know it.

#### Implementation order

Build in this order so each step is testable in isolation:

1. **Expose `PrimaryOrgID()` on `cloud.ConnectionService`.** Two-file change, unblocks everything downstream.
2. **Sensor reading types & parsing.** Define `SequenceReading`/`SequenceResourceReading`, extend `CaptureControlSensorConfig` with `SequencesKey`, parse the new key in `builtin.go`. Unit-testable without touching capture state.
3. **Recording state & close detection.** Add `openRecordings` to `Capture` (content-hash keyed), accept the parsed `[]SequenceReading` per tick, emit `[]PendingSequence` on close. Pure logic, table-driven tests.
4. **Disk persistence.** Write `pending_sequences/<uuid>.json` atomically on close (write `.tmp`, rename).
5. **Retry worker.** Background `StoppableWorkers` in `builtIn` that walks `pending_sequences/`, calls `DataServiceClient.CreateSequence`, deletes on success.
6. **Polish.** Warning logs when sequence resources don't appear in `capture_configs`, log-rate-limiting for sustained outages, `failed/` subdir handling.

#### Step 1: Expose `PrimaryOrgID()` (already implemented in this PR)

- `internal/cloud/service.go` — added `PrimaryOrgID() string` to `ConnectionService` interface; `cloudManagedService` returns `cm.cloudCfg.PrimaryOrgID`. Returns `""` for non-cloud-managed robots.
- `internal/testutils/inject/cloud_connection.go` — added `PrimaryOrgIDValue string` field and method on the inject mock.

#### Step 2: Sensor reading types & parsing

- `services/datamanager/data_manager.go` — add the new types:

  ```go
  // SequenceReading represents one active sequence recording emitted by the capture control sensor.
  // While present in the sensor's sequences array, the recording is open; when it disappears the
  // recording closes and the data manager creates a sequence covering its time range.
  type SequenceReading struct {
      SequenceTags []string                  `json:"sequence_tags,omitempty"`
      Resources    []SequenceResourceReading `json:"resources"`
  }

  // SequenceResourceReading identifies one resource/method pair within a sequence recording.
  // Shape mirrors CaptureConfigReading's identity fields so sensor authors can reuse constants.
  type SequenceResourceReading struct {
      ResourceName string `json:"resource_name"`
      Method       string `json:"method"`
  }
  ```

- `services/datamanager/builtin/config.go` — add `SequencesKey string` to `CaptureControlSensorConfig` (json tag `sequences_key,omitempty`). Sequence feature is opt-in: empty means off.
- `services/datamanager/builtin/builtin.go` — add `parseSequencesFromReadings(readings, sequencesKey)` mirroring the existing `parseOverridesFromReadings`. Returns `[]datamanager.SequenceReading` or `nil`. Validate that `Resources` is non-empty; reject entries with zero resources at parse time with a warning.

#### Step 3: Recording state & close detection

- `services/datamanager/builtin/capture/capture_control.go` — extend `Capture` with `openRecordings map[string]*openSequenceRecording` (content-hash keyed). Add:

  ```go
  // SetActiveSequences accepts the currently-active sequence recordings from the latest
  // sensor reading. Returns the set of pending sequences that closed this tick (hashes
  // present last tick but absent now). Called under or after SetCaptureConfigs since
  // both touch Capture state under collectorsMu.
  func (c *Capture) SetActiveSequences(now time.Time, active []datamanager.SequenceReading) []PendingSequence
  ```

  Internally: hash each `active` entry, open new ones (`startAt = now`), close ones missing this tick (build `PendingSequence{StartAt, EndAt: now, Resources, Tags}`). Hash canonicalization: sort resources by `(resource_name, method)`, sort tags lexicographically, hash the JSON of the sorted tuple.

- `services/datamanager/builtin/capture/capture_control_test.go` — table tests:
  - New entry → opens, no closes returned
  - Same entry next tick → still open, no closes
  - Entry disappears → close returned with correct `startAt`/`endAt`
  - Two concurrent recordings, one disappears → only that one closes
  - Two back-to-back identical-content recordings with a gap (`sequences: []` between) → two separate closes
  - Empty `active` list → all open recordings close
  - Resource ordering within an entry doesn't affect identity (canonicalization works)
  - Tag ordering doesn't affect identity

#### Step 4: Disk persistence

- `services/datamanager/builtin/builtin.go` — after `runCaptureControlPoller` calls `SetActiveSequences`, for each returned `PendingSequence`:
  1. Resolve `part_id` via `cloudConnSvc.AcquireConnection` and `org_id` via `cloudConnSvc.PrimaryOrgID()`. If either fails or is empty, log a warning and drop (no point writing a sequence we can't ever upload).
  2. Build the JSON file (see "Pending Sequence File Schema" above).
  3. Generate a fresh UUID for the filename: `<capture_dir>/pending_sequences/<uuid>.json`.
  4. Write atomically: write to `<uuid>.json.tmp`, then `os.Rename` to `<uuid>.json`.
  5. Create the `pending_sequences/` directory at `BuiltInReconfigure` time if it doesn't exist.

#### Step 5: Retry worker

- `services/datamanager/builtin/sequence_retry_worker.go` — new file. Background `StoppableWorkers` ticker (30s base, exponential backoff to 1hr on transient errors):
  1. `os.ReadDir(<capture_dir>/pending_sequences/)` (skip if dir doesn't exist).
  2. For each `*.json` file (skip `.tmp` and the `failed/` subdir):
     - Read + unmarshal the JSON. On parse error, move to `failed/`.
     - Acquire `cloudConnSvc.AcquireConnection`. If `ErrNotCloudManaged`, abort this tick silently.
     - Build `*datapb.CreateSequenceRequest` (translate `resource_name → component_name`, `method → method_name`).
     - Call `datapb.NewDataServiceClient(conn).CreateSequence(...)`.
     - On success: `os.Remove` the file.
     - On gRPC code `InvalidArgument`/`PermissionDenied`/`NotFound` (permanent): move to `failed/`.
     - On any other error (transient): leave the file, log warning, continue.

- `services/datamanager/builtin/builtin.go` — start the retry worker in `BuiltInReconfigure` if `c.CaptureControlSensor != nil && c.CaptureControlSensor.SequencesKey != ""`, OR if the `pending_sequences/` directory exists with files (don't strand pending files when config changes). Stop it in `Close` and at the top of `BuiltInReconfigure`.

#### Step 6: Polish

- Warning at sequence-parse time if a sequence's `resources` references a `(resource_name, method)` pair not currently enabled in `capture_configs`.
- Rate-limit "failed to upload pending sequence" warnings: once at warning, then debug-level until next successful upload.
- Surface a count of pending sequences in `diskSummaryTracker` for operators monitoring the machine.

#### Things to consider

- **Two pollers, one mutex:** the capture control poller should call `SetCaptureConfigs` and `SetActiveSequences` within the same `collectorsMu` critical section so an enabling capture config + new sequence in the same tick are atomic.
- **Sensor not configured for sequences:** if `SequencesKey == ""`, skip sequence parsing entirely. Still start the retry worker if pending files exist on disk (don't strand a queue from a previous config).
- **`AcquireConnection` partID:** the existing `cloudConnSvc.AcquireConnection` returns `cloudCfg.ID` as the first return — that's the part ID.
- **Filename collision:** UUIDs avoid collisions across restarts.
- **Worker lifecycle:** retry worker uses its own `StoppableWorkers`, separate from `captureControlPoller`. Stopping must happen before `b.mu.Lock` in `Close` (same pattern as `stopCaptureControlPoller`).
- **Disk space:** pending files are tiny (~hundreds of bytes). Disk-full cleanup logic should leave `pending_sequences/` alone.
- **Failed/ files:** never auto-clean. Operators inspect them and decide.

---

## Out of Scope (v1)

| Item                                      | Notes                                                                                                                       |
| ----------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| Creating sequences via UI                 | CRUD is available via API; UI creation is a follow-up                                                                       |
| Recent/hot data                           | `destination_ids` routes recent data to fragment owner orgs; sequences v1 only covers standard binary and tabular sync data |
| Sub-org scoping (location, robot level)   | Sequences are org-scoped only; resource filters go to part level                                                            |
| Sequence-to-sequence composition          | No nesting or inheritance between sequences                                                                                 |
| Real-time/streaming data                  | Sequences are defined over a fixed, bounded time range                                                                      |
| Notifications on sequence creation/update | No webhooks or pubsub events in v1                                                                                          |
| Interpolated tabular alignment            | Nearest-timestamp alignment is sufficient for v1; interpolation is a training pipeline concern                              |

---

## Open Questions

1. **Validation at creation time:** Should we validate that each `part_id` in `resources` belongs to the sequence's org at creation time, or accept any value and let data retrieval auth silently filter? Recommendation: no creation-time validation (consistent with existing filter behavior).

2. **Sequences proto location:** Does this land in `go.viam.com/api/app/data/v1` alongside existing data RPCs, or in a new `go.viam.com/api/app/sequences/v1` package?

---

## Rejected Alternatives

### Drop `organization_id` from `CreateSequenceRequest` and derive it server-side from `part_id`

**Proposal.** Instead of requiring the caller to send `organization_id`, the server would derive it from a single `part_id` lookup (hoisting `part_id` to the top of `CreateSequenceRequest` and removing it from each `SequenceResourceFilter`):

```protobuf
message SequenceResourceFilter {
  string component_name = 1;
  string method_name = 2;
}

message CreateSequenceRequest {
  string part_id = 1;
  repeated SequenceResourceFilter resources = 2;
  repeated string sequence_tags = 3;
  google.protobuf.Timestamp start_at = 4;
  google.protobuf.Timestamp end_at = 5;
}
```

The server would call `partsDB.FindByID(req.PartId)`, derive `organization_id`, store both on the sequence record, and authorize on the derived org. This mirrors how `DataCaptureUpload` works (robot sends `part_id`, server infers org from auth or part lookup).

**Why it was attractive.**
- Symmetric with `DataCaptureUpload` — the same data-plane pattern.
- No need for the robot to know its own org_id (one fewer responsibility on the RDK side).
- Single auth path on the server for both user-callers and part-callers (no optional fields, no auth-context branching).
- Hoisting `part_id` improves Mongo query construction (top-level AND on `part_id`, smaller `$or` over `(component_name, method_name)` tuples).
- Models the real-world constraint that a sequence is part-scoped (a single robot recording a single demo).

**Why it was rejected.**
- We chose to expose `PrimaryOrgID()` on `cloud.ConnectionService` independently (closing a long-standing gap where in-process services couldn't access cloud metadata). Once the RDK can supply `organization_id` directly, the server-side derivation is no longer needed to keep the robot's responsibilities small.
- Keeping `organization_id` in the request preserves consistency with `ListSequencesRequest` (which still uses `organization_id`) and with how the rest of the data management API surface is shaped — `organization_id` is the standard top-level scoping field across `data/v1`.
- Multi-part sequences (theoretical today, but possible in some future "coordinated demo" use case) remain expressible without a schema migration.
- The server-side change has a bigger blast radius than the RDK-side `PrimaryOrgID()` addition: it touches proto, handler, storage schema, query builders, and the public API surface. The RDK change is two files and ~15 lines.

**When to revisit.** If a future requirement makes server-side org derivation valuable — for example, a non-Viam-RDK client that wants to create sequences without managing org IDs, or a strong push to remove all redundant inputs from data-plane RPCs — this alternative becomes worth reconsidering. At that point the part_id hoist (independent of the org_id derivation) is also worth picking up on its own merits.
