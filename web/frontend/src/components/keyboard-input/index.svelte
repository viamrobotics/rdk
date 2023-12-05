<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { Keys } from './types';

  import {
    mdiArrowUp as w,
    mdiRestore as a,
    mdiReload as d,
    mdiArrowDown as s,
  } from '@mdi/js';
  import Icon from '../icon/index.svelte';

  export let isActive: boolean;
  export let onKeyDown: (key: Keys) => void;
  export let onKeyUp: (key: Keys) => void;
  export let onUpdateKeyboardState: (isActive: boolean) => void;

  const keyIcons = { w, a, s, d };

  const pressedKeys = {
    w: false,
    a: false,
    s: false,
    d: false,
  };

  const keysLayout = [['a'], ['w', 's'], ['d']] as const;

  const normalizeKey = (key: string): Keys | null => {
    return (
      (
        {
          w: 'w',
          a: 'a',
          s: 's',
          d: 'd',
          arrowup: 'w',
          arrowleft: 'a',
          arrowdown: 's',
          arrowright: 'd',
        } as Record<string, Keys>
      )[key.toLowerCase()] ?? null
    );
  };

  const dispatchKeyDown = (key: Keys) => {
    pressedKeys[key] = true;
    onKeyDown(key);
  };

  const dispatchKeyUp = (key: Keys) => {
    pressedKeys[key] = false;
    onKeyUp(key);
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    event.preventDefault();
    event.stopPropagation();

    const key = normalizeKey(event.key);

    if (key === null || pressedKeys[key]) {
      return;
    }

    dispatchKeyDown(key);
  };

  const handleKeyUp = (event: KeyboardEvent) => {
    event.preventDefault();
    event.stopPropagation();

    const key = normalizeKey(event.key);
    if (key !== null) {
      dispatchKeyUp(key);
    }
  };

  const toggleKeyboard = (nowActive: boolean) => {
    if (nowActive) {
      window.addEventListener('keydown', handleKeyDown, false);
      window.addEventListener('keyup', handleKeyUp, false);
    } else {
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('keyup', handleKeyUp);
    }

    onUpdateKeyboardState(nowActive);
  };

  const handlePointerDown = (key: Keys) => {
    dispatchKeyDown(key);
  };

  const handlePointerUp = (key: Keys) => {
    dispatchKeyUp(key);
  };

  $: if (!isActive) {
    toggleKeyboard(false);
  }

  onDestroy(() => {
    toggleKeyboard(false);
  });
</script>

<div>
  <v-switch
    label="Enable keyboard"
    class="w-fit pr-4"
    value={isActive ? 'on' : 'off'}
    on:input={() => {
      toggleKeyboard(!isActive);
    }}
  />
  <div class="flex justify-center gap-2">
    {#each keysLayout as lineKeys, index (index)}
      <div class="my-1 flex flex-col gap-2 self-end">
        {#each lineKeys as key (key)}
          <button
            class="flex select-none items-center gap-1.5 border border-gray-500 px-3 py-1 uppercase outline-none"
            class:bg-gray-200={pressedKeys[key]}
            class:text-gray-800={pressedKeys[key]}
            class:bg-white={!pressedKeys[key]}
            on:pointerdown={() => {
              handlePointerDown(key);
            }}
            on:pointerup={() => {
              handlePointerUp(key);
            }}
            on:pointerleave={() => {
              handlePointerUp(key);
            }}
          >
            {key}
            <Icon path={keyIcons[key]} />
          </button>
        {/each}
      </div>
    {/each}
  </div>
</div>
