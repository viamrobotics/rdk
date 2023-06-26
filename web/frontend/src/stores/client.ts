import type { Client } from '@viamrobotics/sdk';
import { writable } from 'svelte/store';

export const client = writable<Client>();
