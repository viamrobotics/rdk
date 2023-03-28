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
  if (!props.client) {
    const rtcConfig = {
      iceServers: [
        {
          urls: 'stun:global.stun.twilio.com:3478',
        },
      ],
    };

    if (window.webrtcAdditionalICEServers) {
      rtcConfig.iceServers = [
        ...rtcConfig.iceServers,
        ...window.webrtcAdditionalICEServers,
      ];
    }

    const impliedURL = `${location.protocol}//${location.hostname}${location.port ? `:${location.port}` : ''}`;

    props.client = new Client(impliedURL, {
      enabled: true,
      host: props.host,
      signalingAddress: window.webrtcSignalingAddress,
      rtcConfig,
    });
  }

  return createApp(RemoteControlCards, props);
};
