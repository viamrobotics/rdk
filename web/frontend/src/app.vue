<!-- eslint-disable require-atomic-updates -->
<script setup lang="ts">
import RemoteControlCards from './components/remote-control-cards.vue';

const host = `${location.protocol}//${location.hostname}${location.port ? `:${location.port}` : ''}`;
const bakedAuth = window.bakedAuth;
const rcDebug = window.rcDebug;
const supportedAuthTypes = window.supportedAuthTypes;
const webrtcAdditionalICEServers = window.webrtcAdditionalICEServers;
const webrtcEnabled = window.webrtcEnabled;
const webrtcSignalingAddress = window.webrtcSignalingAddress;

</script>

<template>
  <RemoteControlCards
    :host=host
    :baked-auth=bakedAuth
    :rcDebug=rcDebug
    :supported-auth-types=supportedAuthTypes
    :webrtc-additional-i-c-e-servers=webrtcAdditionalICEServers
    :webrtc-enabled=webrtcEnabled
    :webrtcSignalingAddress=webrtcSignalingAddress
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
