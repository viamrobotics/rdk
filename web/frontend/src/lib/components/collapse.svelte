<!--
  Rendering embedded maps inside of a shadow dom is currently broken for MapLibreGL.
  This Collapse component is taken from PRIME but removes all web component behavior.
  Once we've completed the svelte migration it can be removed and replaced with the PRIME component.
-->
<script lang="ts">
import { onMount, createEventDispatcher, tick } from 'svelte';

export let title = '';
export let open = Boolean(localStorage.getItem(`rc.collapse.${title}.open`));

const dispatch = createEventDispatcher();

const handleClick = async (event: Event) => {
  if ((event.target as HTMLElement).getAttribute('slot') === 'header') {
    return;
  }

  open = !open;

  if (open) {
    localStorage.setItem(`rc.collapse.${title}.open`, 'true');
  } else {
    localStorage.removeItem(`rc.collapse.${title}.open`);
  }

  await tick();

  dispatch('toggle', { open });
};

onMount(() => {
  if (open) {
    dispatch('toggle', { open: true });
  }
});
</script>

<div class="relative w-full" id={title}>
  <div
    class="
      flex-reverse flex w-full cursor-pointer items-center justify-between
      border border-light bg-white px-4 py-2 text-default
    "
    on:click={handleClick}
    on:keyup|stopPropagation|preventDefault={handleClick}
  >
    <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
      {#if title}
        <h2 class="m-0 text-sm">{title}</h2>
      {/if}

      <slot name="title" />
    </div>

    <div class="flex h-full items-center gap-3">
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
