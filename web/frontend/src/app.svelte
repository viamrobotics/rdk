<script lang="ts">

import RemoteControlCards from './components/remote-control-cards.svelte';

const {
  host,
  bakedAuth,
  supportedAuthTypes,
  webrtcEnabled,
  webrtcSignalingAddress,
} = window;

let selected = 'one'

const handleSelect = (event: CustomEvent) => {
  selected = event.detail.value
}

</script>

<v-tabs tabs='one,two' {selected} on:input={handleSelect} />

{#if selected === 'one'}
<RemoteControlCards
  {host}
  {bakedAuth}
  {supportedAuthTypes}
  {webrtcEnabled}
  signalingAddress={webrtcSignalingAddress}
/>
{/if}