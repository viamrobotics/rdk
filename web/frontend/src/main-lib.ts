import './index.css';
import { createApp } from 'vue';
import RemoteControlCards from './app.vue';
import type { Credentials } from '@viamrobotics/rpc';
import type { Client } from '@viamrobotics/sdk';

export const createRcApp = (props: {
  host: string;
  bakedAuth?: {
    authEntity: string;
    creds: Credentials;
  },
  supportedAuthTypes: string[],
  webrtcEnabled: boolean,
  client: Client;
}) => {
  return createApp(RemoteControlCards, props);
};
