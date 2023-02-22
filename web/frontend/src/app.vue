<!-- eslint-disable require-atomic-updates -->
<script setup lang="ts">
import { Client } from '@viamrobotics/sdk';
import RemoteControlCards from './components/remote-control-cards.vue';

const {
  host,
  bakedAuth,
  supportedAuthTypes,
  webrtcAdditionalICEServers,
  webrtcEnabled,
  webrtcSignalingAddress,
} = window;

const rtcConfig = {
  iceServers: [
    {
      urls: 'stun:global.stun.twilio.com:3478',
    },
  ],
};

if (webrtcAdditionalICEServers) {
  rtcConfig.iceServers = [...rtcConfig.iceServers, ...webrtcAdditionalICEServers];
}

const impliedURL = `${location.protocol}//${location.hostname}${location.port ? `:${location.port}` : ''}`;
const client = new Client(impliedURL, {
  enabled: webrtcEnabled,
  host,
  signalingAddress: webrtcSignalingAddress,
  rtcConfig,
});

</script>

<template>
  <RemoteControlCards
    :host="host"
    :baked-auth="bakedAuth"
    :supported-auth-types="supportedAuthTypes"
    :webrtc-enabled="webrtcEnabled"
    :client="client"
  />
</template>

<style>
#source {
  position: relative;
  width: 50%;
  height: 50%;
}
h3 {
  margin: 0.1em;
  margin-block-end: 0.1em;
}
</style>
