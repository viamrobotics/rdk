<script lang="ts">

import RemoteControlCards from './components/remote-control-cards.svelte';

const {
  host,
  bakedAuth,
  supportedAuthTypes,
  webrtcEnabled,
  webrtcSignalingAddress,
} = window;

</script>

<RemoteControlCards
  {host}
  {bakedAuth}
  {supportedAuthTypes}
  {webrtcEnabled}
  signalingAddress={webrtcSignalingAddress}
/>