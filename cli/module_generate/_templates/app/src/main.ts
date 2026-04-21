import { createRobotClient } from '@viamrobotics/sdk';
import { getCookie } from 'typescript-cookie';
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
        const machine = await createRobotClient({
            host,
            credentials,
            signalingAddress: getCookie('is_local') ? 'http://localhost:8080' : 'https://app.viam.com',
        });
        statusEl.textContent = 'Connected to ' + host;

        const names = await machine.resourceNames();
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
