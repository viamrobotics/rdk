# Local test: world state store feeding the motion service

A throwaway harness for manually verifying that a configured `world_state_store` service feeds
obstacles into the motion service, so a `Move` avoids them without the caller passing a `WorldState`.

[`config.json`](./config.json) holds only the `components` and `services` a tester needs (no cloud
credentials):

- a `simulated` arm (`ur5e`) framed to `world`. Unlike the `fake` arm, the `simulated` model runs a
  trajectory generator and (with `simulate-time: true`) interpolates the joints over real time, so a
  `Move` plays out as visible motion in the visualizer instead of snapping to the goal. Tune `speed`
  (radians per second) to taste,
- a static `table` obstacle: a boxed `generic` component (1000 x 1000 x 10 mm) framed just under the arm
  base so its geometry is picked up as a fixed collision floor the arm plans around,
- a `fake` `world_state_store` in `depth_camera` mode (simulates a depth camera detecting three colored
  blocks; no real sensor involved),
- a `builtin` motion service with `world_state_store_service_name: "worldstate"`.

The flow is: paste these into a cloud machine, download the full config the app generates (it adds the
cloud block for you), and run this branch's viam-server against that download. This avoids hand-editing
credentials into a local file.

## 1. Create a machine and add the config

On [app.viam.com](https://app.viam.com), create a new machine. Open its **Config** tab, switch to the
raw **JSON** editor, and paste in the `components` and `services` from [`config.json`](./config.json)
(merge them into whatever is already there). Save.

## 2. Download the machine config

From the machine's part menu, download the viam-server config the way you normally would. The downloaded
file includes the `cloud` block (machine id, secret, api key) that the app manages for you. Save it
somewhere outside this repo, e.g. `~/machine.json`.

## 3. Run viam-server from this branch against it

From the repo root:

```bash
go run ./web/cmd/server -config ~/machine.json
```

(Run from source; don't commit a built binary, testers may be on a different OS/arch.) On startup you
should see `world state store "worldstate" wired into this motion service` in the logs.

## 4. Run the visualizer

Clone [viamrobotics/visualization](https://github.com/viamrobotics/visualization) (aka `motion-tools`)
next to this repo. Copy `.env.example` to `.env.local` and fill the `VITE_CONFIGS` entry from the
downloaded config and the machine page:

- `host` -> the machine address (from the machine page, e.g. `main.<hash>.viam.cloud`)
- `partId` -> `cloud.id` from the downloaded config
- `apiKeyId` / `apiKeyValue` -> `cloud.api_key.id` / `cloud.api_key.key` from the downloaded config
- `signalingAddress` -> `https://app.viam.com:443`

Then:

```bash
make setup   # first time only
make up      # serves http://localhost:5173
```

Open the app and connect to your machine. You should see the arm on its table plus the three streamed
blocks (red / green / blue). To see the lidar variant instead, set the world state store's
`input_sensor_type` to `lidar` and you get a fixed ring of point-cloud surfaces.

## 5. Test with the Move Frame plugin

Open the **Move Frame** plugin on the `arm`. Confirm the **Motion service** dropdown is set to
`builtin` (this matters, see below). Drag the move gizmo toward one of the blocks and click **Execute
move**:

- A destination clear of the obstacles plans and moves normally.
- A destination that would put the arm in a block (or below the table) is rejected with a
  constraint-violation error, because the store's obstacles and the configured geometries are both in the
  plan automatically.

### Gotcha: the motion service must be the one the visualizer calls

RDK always auto-creates a default motion service named `builtin`. If you configure your store-aware
motion service under any other name, that default (with no store) will shadow it and the visualizer will
call the store-less one, so the arm will happily drive through blocks. Keep the service named `builtin`
(as in `config.json`), or explicitly select your service in the plugin's Motion service dropdown.

## Scripted demo path (good for recording)

[`main.go`](./main.go) is a separate client: with viam-server already running (step 3) and the visualizer
open (step 4), run it in another terminal and it plays a single scripted path, then exits. Record the
visualizer while it runs. It has one path per fake sensor mode; run the mode that matches the store's
`input_sensor_type`:

```bash
go run ./_local_wss_test/main.go -mode depth_camera \
  -address <machine-address> -api-key-id <cloud.api_key.id> -api-key <cloud.api_key.key>
```

Because the machine runs the cloud config, the client must authenticate just like the visualizer. Pass
the machine address and API key (the same `host` / `apiKeyId` / `apiKeyValue` you put in the visualizer's
`.env.local`), or set them once as `VIAM_ADDRESS`, `VIAM_API_KEY_ID`, and `VIAM_API_KEY`. Without a key
the client dials `localhost:8080` insecurely, which only works against a cloudless viam-server.

- `-mode depth_camera` (default): the arm sweeps the three blocks and refuses to enter them.
- `-mode lidar`: the arm threads the point-cloud ring and refuses the surfaces.

Each clear target moves the arm; each obstacle target is refused (the arm holds and the script prints
`correctly refused`). The script also prints how many transforms the store serves (3 for depth_camera,
1 for lidar) so you can confirm the mode matches before recording. Flags: `-mode`, `-address`,
`-api-key-id`, `-api-key`, `-pause` (defaults to 1.5s). Lower the arm's `speed` in `config.json` for
slower, more cinematic motion.
