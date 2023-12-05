<script lang='ts'>

/* eslint-disable unicorn/prefer-blob-reading-methods */
import { createEventDispatcher } from 'svelte';

export let format: 'string' | 'arrayBuffer' = 'string';

const dispatch = createEventDispatcher();

const handleDrop = (event: DragEvent) => {
  const reader = new FileReader();

  reader.addEventListener('load', () => {
    dispatch('drop', reader.result);
  });

  if (event.dataTransfer === null) {
    return;
  }

  const [file] = event.dataTransfer.files;

  if (file === undefined) {
    return;
  }

  if (format === 'string') {
    reader.readAsBinaryString(file);
  } else {
    reader.readAsArrayBuffer(file);
  }
};

</script>

<div
  on:dragenter|preventDefault
  on:dragover|preventDefault
  on:drop|preventDefault={handleDrop}
  {...$$restProps}
>
  <slot />
</div>
