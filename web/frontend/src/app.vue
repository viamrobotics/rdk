<!-- eslint-disable require-atomic-updates -->
<script setup lang="ts">
import RemoteControlCards from './components/remote-control-cards.vue';

const host = $computed(() => `${location.protocol}//${location.hostname}${location.port ? `:${location.port}` : ''}`);
const bakedAuth = $computed(() => window.bakedAuth);
const rcDebug = $computed(() => window.rcDebug);
const supportedAuthTypes = $computed(() => window.supportedAuthTypes);
const webrtcAdditionalICEServers = $computed(() => window.webrtcAdditionalICEServers);
const webrtcEnabled = $computed(() => window.webrtcEnabled);
const webrtcSignalingAddress = $computed(() => window.webrtcSignalingAddress);

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
