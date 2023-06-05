<!-- eslint-disable require-atomic-updates -->
<script setup lang="ts">
import RemoteControlCards from './components/remote-control-cards.vue';

const {
  host,
  bakedAuth,
  supportedAuthTypes,
  webrtcEnabled,
  webrtcSignalingAddress,
} = window;

</script>

<template>
  <RemoteControlCards
    :host="host"
    :baked-auth="bakedAuth"
    :supported-auth-types="supportedAuthTypes"
    :webrtc-enabled="webrtcEnabled"
    :signaling-address="webrtcSignalingAddress"
  />

  <div id="map" class="w-[500px] h-[500px]" />
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
