<script lang="ts">
  import type { inputControllerApi } from "@viamrobotics/sdk";

  export let name: string;
  export let status: inputControllerApi.Status.AsObject;

  const controlOrder = [
    "AbsoluteX",
    "AbsoluteY",
    "AbsoluteRX",
    "AbsoluteRY",
    "AbsoluteZ",
    "AbsoluteRZ",
    "AbsoluteHat0X",
    "AbsoluteHat0Y",
    "ButtonSouth",
    "ButtonEast",
    "ButtonWest",
    "ButtonNorth",
    "ButtonLT",
    "ButtonRT",
    "ButtonLThumb",
    "ButtonRThumb",
    "ButtonSelect",
    "ButtonStart",
    "ButtonMenu",
    "ButtonEStop",
  ];

  $: connected = ((eventsList: inputControllerApi.Event.AsObject[]) => {
    for (const { event } of eventsList) {
      if (event !== "Disconnect") {
        return true;
      }
    }
    return false;
  })(status.eventsList);

  const getValue = (
    eventsList: inputControllerApi.Event.AsObject[],
    controlMatch: string
  ) => {
    for (const { control, value } of eventsList) {
      if (control === controlMatch) {
        return control.includes("Absolute")
          ? value.toFixed(4)
          : value.toFixed(0);
      }
    }

    return "";
  };

  $: controls = ((eventsList: inputControllerApi.Event.AsObject[]) => {
    const pendingControls: [control: string, value: string][] = [];

    for (const ctrl of controlOrder) {
      const value = getValue(eventsList, ctrl);
      if (value !== "") {
        pendingControls.push([
          ctrl.replace("Absolute", "").replace("Button", ""),
          value,
        ]);
      }
    }

    return pendingControls;
  })(status.eventsList);
</script>

<v-collapse title={name}>
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
        <div class="ml-0 flex w-[8ex] flex-col">
          <p class="subtitle m-0">
            {control[0]}
          </p>
          {control[1]}
        </div>
      {/each}
    {/if}
  </div>
</v-collapse>

<style scoped>
  .subtitle {
    color: var(--black-70);
  }
</style>
