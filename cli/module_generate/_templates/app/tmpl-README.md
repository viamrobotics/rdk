# {{ .AppName }}

A Viam app module that serves your web app from your machine via the Viam SDK.

## Build your frontend

This module doesn't provide any frontend - bring your own using whatever framework you like (e.g. Svelte, Vue, React). This README assumes `npm` usage. For other package managers, adapt accordingly.

> **Note:** Viam provides a set of utilities for Svelte that make it easy to integrate Viam into Svelte apps. See the [Viam TypeScript SDK](https://github.com/viamrobotics/viam-typescript-sdk) for details.

To connect to your machine, install the Viam SDK and cookie helper with your package manager. For example, if using `npm`:

```
npm install @viamrobotics/sdk typescript-cookie
```

A utility file is included at `auth.ts` in the project root. Copy it into your frontend project and import it:

```js
import { createRobotClient } from '@viamrobotics/sdk';
import { getHostAndCredentials } from './auth';

const { host, credentials } = getHostAndCredentials();
const machine = await createRobotClient({
    host,
    credentials,
    signalingAddress: 'https://app.viam.com',
});
const resources = await machine.resourceNames();
```

Viam Apps expects the entrypoint of your app to be at `dist/index.html`. If you would like to change this, update it in the `meta.json`, the `Makefile`, and the `module.go` file.

**Important:** Your frontend must use relative paths in its build output (e.g. `./static/js/main.js`, not `/static/js/main.js`). Absolute paths will break when served from viamapplications.com or the local server.

After building your frontend into `dist/`, run `make` to build the module.

## Test during development

Test your frontend against a real machine during development:

1. Start your frontend dev server (e.g. `npm run dev`) and note the port it starts on
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
   make
   ```

2. Upload `module.tar.gz` (not the binary) to the registry:
   ```
   viam module upload --upload=./module.tar.gz --platform=linux/amd64
   ```

3. Add the module to your machine on app.viam.com:
   - Go to app.viam.com → your machine → Config
   - Add the module by name (`{{ .Namespace }}:{{ .ModuleName }}`)
   - Add a component: type `generic`, model `{{ .Namespace }}:{{ .ModuleName }}:webapp`
   - Save config

4. Access your app at:
   ```
   https://{{ .AppName }}_{{ .Namespace }}.viamapplications.com
   ```

**Note:** After re-uploading a new version, you may need to hard-refresh (Cmd+Shift+R / Ctrl+Shift+R) to avoid seeing a cached version.

## Local server

When the module is running on a machine (via registry upload above), it also serves your app on the local network:

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
