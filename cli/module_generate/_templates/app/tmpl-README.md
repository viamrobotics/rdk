# {{ .AppName }}

A Viam app module that serves your web app from your machine via the Viam SDK.

For full documentation, see [Viam Apps](https://docs.viam.com/build-apps/hosting/overview/).

## Build your frontend

This module doesn't provide any frontend - bring your own using whatever framework you like (e.g. Svelte, Vue, React). This README assumes `npm` usage. For other package managers, adapt accordingly.

> **Note:** Viam provides a set of utilities for Svelte that make it easy to integrate Viam into Svelte apps. See the [Viam Svelte SDK](https://github.com/viamrobotics/viam-svelte-sdk) for details.

To connect to your machine, install the Viam SDK and cookie helper with your package manager:

```
npm install @viamrobotics/sdk typescript-cookie
```

A utility file is included at `auth.js` in the project root. Copy it into your frontend's source directory alongside the file that will import it.

### Single machine

```js
import { createRobotClient } from '@viamrobotics/sdk';
import { getHostAndCredentials } from './auth.js';

const { host, credentials } = getHostAndCredentials();
const machine = await createRobotClient({
    host,
    credentials,
    signalingAddress: 'https://app.viam.com',
});
const resources = await machine.resourceNames();
```

### Multi machine

```js
import * as VIAM from '@viamrobotics/sdk';
import { getMultiMachineCredentials } from './auth.js';

const { credentials } = getMultiMachineCredentials();
const client = await VIAM.createViamClient({ credentials });

const machine = await client.connectToMachine({ id: '<machine-id>' });
const resources = await machine.resourceNames();
```

Viam Apps expects the entrypoint of your app to be at `dist/index.html`. If your frontend builds into a subdirectory (e.g. `dist/build/index.html`), update the path in three places:
- `meta.json`: the `entrypoint` field under `applications`
- `Makefile`: the `ENTRYPOINT` variable
- `module.go`: both the `//go:embed` path and the `fs.Sub` path in `distFS()`

**Important:** Your frontend must use relative paths in its build output (e.g. `./static/js/main.js`, not `/static/js/main.js`). Absolute paths will break when served from viamapplications.com or the local server. For Create React App, add `"homepage": "."` to your `package.json` to enable this.

Multi machine apps don't include a built-in machine picker, but it's easy to set one up. See [Multi-machine applications](https://docs.viam.com/build-apps/hosting/hosting-reference/#multi-machine-applications) for details.

After building your frontend into `dist/`, run `make` to build the module.
```
make setup
make
```

## Test during development

Test your frontend against a real machine during development:

1. Start your frontend dev server from your frontend's directory and note the port it starts on (the command depends on your framework, e.g. `npm run dev` for Vite or `npm start` for Create React App)
2. In another terminal:
   ```
   viam module local-app-testing --app-url=http://localhost:<PORT> --machine-id=<YOUR_MACHINE_ID>
   ```
3. Open http://localhost:8012/start in your browser

This injects real machine credentials into your dev server so you can test SDK connections without deploying.

To check that your HTML/CSS renders without a machine connection, just open your HTML file directly in a browser.

## Upload to viamapplications.com

1. Build the module:
   ```
   make setup
   make
   ```

2. Upload `module.tar.gz` (not the binary) to the registry (`--version` is required and must be a valid semver). Use the platform matching your build machine:
   ```
   viam module upload --upload=./module.tar.gz --platform=linux/amd64 --version=0.1.0  # Linux x86
   viam module upload --upload=./module.tar.gz --platform=darwin/arm64 --version=0.1.0  # Apple Silicon
   ```

3. Access your app at:
   ```
   https://{{ .AppName }}_{{ .Namespace }}.viamapplications.com
   ```

**Note:** After re-uploading a new version, you may need to hard-refresh (Cmd+Shift+R / Ctrl+Shift+R) to avoid seeing a cached version.

## Local server

**Note:** Local server currently only works for single-machine apps.

To serve your app on the local network, add the module to a machine on app.viam.com:

1. Go to app.viam.com → your machine → Config
2. Add the module by name (`{{ .Namespace }}:{{ .ModuleName }}`)
3. Add a component: type `generic`, model `{{ .Namespace }}:{{ .ModuleName }}:webapp`
4. Save config

Your app will then be available at:

```
http://<machine-ip>:8888
```

The port defaults to 8888 and can be configured via the `port` attribute in your component's config.

### Offline mode (no internet)

To use the local server without internet, viam-server needs an HTTP signaling endpoint. Add this to your machine's network config on app.viam.com (Raw JSON):

```json
{
  "network": {
    "no_tls": true
  }
}
```

Then your frontend needs to detect local mode and use local signaling instead of cloud signaling:

```js
const isLocal = document.cookie.includes('is_local=true');
const signalingAddress = isLocal
    ? `http://${window.location.hostname}:8080`
    : 'https://app.viam.com';
```

**Note:** `no_tls` replaces the default HTTPS listener with an HTTP listener. This means traffic on the local network is unencrypted. This is acceptable for trusted networks (factory floor, home) but not recommended for public networks.
