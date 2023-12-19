
<script lang="ts">
import { onMount, createEventDispatcher, tick, setContext } from 'svelte';
import { Button } from '@viamrobotics/prime-core';
import { type CollapseContext, collapseContextKey } from './collapse';

/**
 * The name of the component.
 */
export let title = '';

/**
 * Component metadata.
 */
export let crumbs = '';

/**
 * Whether the card body is displayed or not.
 */
export let open = Boolean(localStorage.getItem(`rc.collapse.${title}.open`));

/**
 * Whether the component has a stop command. Will render a button to issue commands.
 */
export let stop = false

let stopCallback: (() => void) | undefined

setContext<CollapseContext>(collapseContextKey, {
  onStop: (callback: () => void) => {
    stopCallback = callback
  }
})

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

const handleStopClick = (event: MouseEvent) => {
  event.stopPropagation()
  stopCallback?.()
}

onMount(() => {
  if (open) {
    dispatch('toggle', { open: true });
  }
});

</script>

<div class="relative w-full">
  <button
    class='
      border border-light bg-white w-full py-2 px-4
      flex flex-reverse items-center justify-between text-default cursor-pointer
    '
    aria-label='Toggle card'
    on:click={handleClick}
    on:keyup|stopPropagation|preventDefault={handleClick}
  >
    <div class="flex flex-wrap gap-x-3 gap-y-1 items-center">
      {#if title}
        <h2 class="m-0 text-sm">{title}</h2>
      {/if}

      {#if crumbs}
        <v-breadcrumbs {crumbs} />
      {/if}
    </div>

    <div class="h-full flex items-center gap-3">
      {#if stop}
        <Button
          variant="danger"
          icon="stop-circle-outline"
          tabindex='0'
          on:click={handleStopClick}
        >
          Stop
        </Button>
      {/if}

      <v-icon
        class:rotate-0={!open}
        class:rotation-180={open}
        name="chevron-down"
        size="2xl"
      />
    </div>
  </button>

  {#if open}
    <slot />
  {/if}
</div>
