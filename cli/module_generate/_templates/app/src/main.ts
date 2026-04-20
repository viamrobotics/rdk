import { RobotClient } from '@viamrobotics/sdk';
import { getHostAndCredentials } from './lib/auth';

const statusEl = document.getElementById('status')!;
const resourcesEl = document.getElementById('resources')!;

async function main() {
    const { host, credentials } = getHostAndCredentials();
    if (!host) {
        statusEl.textContent = 'No credentials found. Are you running via the local server or app.viam.com?';
        return;
    }

    try {
        const machine = await RobotClient.atAddress(host, { credentials });
        statusEl.textContent = 'Connected to ' + host;

        const names = machine.resourceNames();
        for (const name of names) {
            const li = document.createElement('li');
            li.textContent = `${name.namespace}:${name.type}:${name.subtype}/${name.name}`;
            resourcesEl.appendChild(li);
        }
    } catch (err) {
        statusEl.textContent = 'Connection failed: ' + (err as Error).message;
    }
}

main();
