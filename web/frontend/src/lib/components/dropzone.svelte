<script lang='ts'>

import { createEventDispatcher } from 'svelte'

export let format: 'string' | 'arrayBuffer' = 'string'

type $$Events = { drop: 'string' | 'arrayBuffer' }

const dispatch = createEventDispatcher<$$Events>()

const handleDrop = (event: DragEvent) => {
  const reader = new FileReader()

  reader.addEventListener('load', () => {
    dispatch('drop', reader.result as typeof format)
  })

  if (event.dataTransfer === null) {
    return
  }

  const [file] = event.dataTransfer.files

  if (file === undefined) {
    return
  }

  if (format === 'string') {
    reader.readAsBinaryString(file)
  } else if (format === 'arrayBuffer') {
    reader.readAsArrayBuffer(file)
  } else {
    throw new Error ('Unsupported dropzone format.')
  }
}

</script>

<div
  on:dragenter|preventDefault
  on:dragover|preventDefault
  on:drop|preventDefault={handleDrop}
  {...$$restProps}
>
  <slot />
</div>
