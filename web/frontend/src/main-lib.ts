import './index.css';
import { createApp } from 'vue';
import RemoteControlCards from './components/remote-control-cards.vue';
import type { Credentials } from '@viamrobotics/rpc';
import { Client } from '@viamrobotics/sdk';

export const createRcApp = (props: {
  host: string;
  bakedAuth?: {
    authEntity: string;
    creds: Credentials;
  },
  supportedAuthTypes: string[],
  webrtcEnabled: boolean,
  client?: Client;
}) => {
  const manageClientConnection = props.client === undefined;
  if (manageClientConnection) {
    const rtcConfig = {
      iceServers: [
        {
          urls: 'stun:global.stun.twilio.com:3478',
        },
      ],
    };

    const impliedURL = `${location.protocol}//${location.hostname}${location.port ? `:${location.port}` : ''}`;

    props.client = new Client(impliedURL, {
      enabled: true,
      host: props.host,
      signalingAddress: window.webrtcSignalingAddress,
      rtcConfig,
    });
  }

  return createApp(RemoteControlCards, { ...props, manageClientConnection });
};
