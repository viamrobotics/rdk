# {{ .AppName }}

A Viam app module that serves your web app from your machine via the Viam SDK.

## Build your frontend

This module doesn't provide any frontend - bring your own using whatever framework you like.

To connect to your machine, install the Viam SDK and cookie helper with your package manager:

```
npm install @viamrobotics/sdk typescript-cookie
```

A utility file is included at `auth.ts` that reads machine credentials from cookies. Use it to connect:

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

The default entrypoint is `dist/index.html`. If your frontend build outputs to a different location, update both:
- `ENTRYPOINT` in the `Makefile`
- `"entrypoint"` in the applications section of `meta.json`

After building your frontend, make sure to run:

```
make setup
make
```

This installs dependencies and builds the module.

## Test during development

Test your frontend against a real machine during development:

1. Start your frontend dev server (e.g. `npm run dev` on port 5173)
2. In another terminal:
   ```
   viam module local-app-testing --app-url=http://localhost:5173 --machine-id=<YOUR_MACHINE_ID>
   ```
3. Open http://localhost:8012/start in your browser

This injects real machine credentials into your dev server so you can test SDK connections without deploying.

To check that your HTML/CSS renders without a machine connection, just open your HTML file directly in a browser.

## Host locally

Add the module to a machine without uploading to the registry:

1. Go to app.viam.com → fleet → your machine → configure
2. Add the module as a local module, set executable path to `<full-path>/bin/{{ .ModuleName }}`
3. Add your webapp as a local component with triplet `{{ .Namespace }}:{{ .ModuleName }}:webapp`
   - If you have any other local components your webapp pulls from, they need to be added as well
4. Save your config, and the machine will restart to reconfigure with your module

## Upload to viamapplications.com

1. Upload `module.tar.gz` (not the binary) to the registry:
   ```
   viam module upload --upload=./module.tar.gz --platform=linux/amd64
   ```

2. Add the module to your machine on app.viam.com:
   - Go to app.viam.com → your machine → Config
   - Add the module by name (`{{ .Namespace }}:{{ .ModuleName }}`)
   - Add a component: type `generic`, model `{{ .Namespace }}:{{ .ModuleName }}:webapp`
   - Save config

3. Access your app at:
   ```
   https://{{ .AppName }}_{{ .Namespace }}.viamapplications.com
   ```

**Note:** After re-uploading a new version, you may need to hard-refresh (Cmd+Shift+R / Ctrl+Shift+R) to avoid seeing a cached version.

## Local server

When the module is running on a machine (via either host locally or registry upload above), it also serves your app on the local network:

```
http://<machine-ip>:8888
```

Accessible from any device on the same network. Credentials are injected automatically via cookies. The SDK connects through Viam's cloud for signaling, so internet is required.

To find your machine's IP: `ipconfig getifaddr en0` (macOS) or `hostname -I` (Linux).

### Offline mode (no internet)

To use the local server without internet, viam-server needs an HTTP signaling endpoint. Add this to your machine's network config on app.viam.com (Raw JSON):

```json
{
  "network": {
    "bind_address": "0.0.0.0:8081",
    "no_tls": true
  }
}
```

Then your frontend needs to detect local mode and use local signaling instead of cloud signaling:

```js
const isLocal = document.cookie.includes('is_local=true');
const signalingAddress = isLocal
    ? `http://${window.location.hostname}:8081`
    : 'https://app.viam.com';
```

**Note:** `no_tls` replaces the default HTTPS listener (port 8080) with an HTTP listener on 8081. This means traffic on the local network is unencrypted. This is acceptable for trusted networks (factory floor, home) but not recommended for public networks.

