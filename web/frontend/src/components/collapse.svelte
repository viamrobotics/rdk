<script lang="ts">

import { createEventDispatcher, tick } from 'svelte';

export let title = '';
export let open = false;

const dispatch = createEventDispatcher();

const handleClick = async (event: Event) => {
  if ((event.target as HTMLElement).getAttribute('slot') === 'header') {
    return;
  }

  open = !open;

  await tick();

  dispatch('toggle', { open });
};

</script>

<div class="relative w-full">
  <div
    class='border border-light bg-white w-full py-2 px-4 flex flex-reverse items-center justify-between text-default cursor-pointer'
    on:click={handleClick}
    on:keyup|stopPropagation|preventDefault={handleClick}
  >
    <div class="flex flex-wrap gap-x-3 gap-y-1 items-center">
      {#if title}
        <h2 class="m-0 text-sm">{title}</h2>
      {/if}

      <slot name="title" />
    </div>

    <div class="h-full flex items-center gap-3">
      <slot name="header" />

      <v-icon
        class:rotate-0={!open}
        class:rotation-180={open}
        name="chevron-down"
        size="2xl"
      />
    </div>
  </div>

  {#if open}
    <slot />
  {/if}
</div>
