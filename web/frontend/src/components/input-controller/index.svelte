<script lang="ts">

  import type { inputControllerApi } from '@viamrobotics/sdk';
  import Collapse from '@/lib/components/collapse.svelte';

  // TODO (RSDK-4451): Figure out why value is not always defined
  type InputControllerEvent = (Omit<inputControllerApi.Event.AsObject, 'value'> & {value?: number});
  export let name: string;
  export let status: {
    events?: InputControllerEvent[] | undefined
  } = { events: [] };

  const controlOrder = [
    'AbsoluteX',
    'AbsoluteY',
    'AbsoluteRX',
    'AbsoluteRY',
    'AbsoluteZ',
    'AbsoluteRZ',
    'AbsoluteHat0X',
    'AbsoluteHat0Y',
    'ButtonSouth',
    'ButtonEast',
    'ButtonWest',
    'ButtonNorth',
    'ButtonLT',
    'ButtonRT',
    'ButtonLThumb',
    'ButtonRThumb',
    'ButtonSelect',
    'ButtonStart',
    'ButtonMenu',
    'ButtonEStop',
  ];

  $: events = status.events ?? [];
  $: connected = events.some(({ event }) => event !== 'Disconnect');

  const getValue = (
    eventsList: InputControllerEvent[],
    controlMatch: string
  ) => {
    for (const { control, value } of eventsList) {
      if (control === controlMatch) {
        if (value === undefined) {
          return '';
        }

        return control.includes('Absolute')
          ? value.toFixed(4)
          : value.toFixed(0);
      }
    }

    return '';
  };

  $: controls = ((eventsList: InputControllerEvent[]) => {
    const pendingControls: [control: string, value: string][] = [];

    for (const ctrl of controlOrder) {
      const value = getValue(eventsList, ctrl);
      if (value !== '') {
        pendingControls.push([
          ctrl.replace('Absolute', '').replace('Button', ''),
          value,
        ]);
      }
    }

    return pendingControls;
  })(events);
</script>

<Collapse title={name}>
  <v-breadcrumbs slot="title" crumbs="input_controller" />
  <div slot="header" class="flex flex-wrap items-center">
    {#if connected}
      <v-badge color="green" label="Connected" />
    {:else}
      <v-badge color="gray" label="Disconnected" />
    {/if}
  </div>
  <div class="border border-t-0 border-medium p-4">
    {#if connected}
      {#each controls as control (control[0])}
        <v-input
          readonly
          class='w-20'
          labelposition='left'
          label={control[0]}
          value={control[1]}
        />
      {/each}
    {/if}
  </div>
</Collapse>

<style>
  .subtitle {
    color: var(--black-70);
  }
</style>
