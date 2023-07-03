import { writable } from 'svelte/store';
import type { ResponseStream, robotApi } from '@viamrobotics/sdk';

export const statusStream = writable<null | ResponseStream<robotApi.StreamStatusResponse>>(null);
