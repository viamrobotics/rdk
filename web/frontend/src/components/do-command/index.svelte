<script lang="ts">

import { type Client, commonApi } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { resourceNameToString } from '@/lib/resource';
import { doCommand } from '@/api/do-command';
import Collapse from '@/components/collapse.svelte';

export let resources: commonApi.ResourceName.AsObject[];
export let client: Client;

let selectedComponent = '';
let input = '';
let output = '';
let executing = false;

const handleDoCommand = async (name: string, command: string) => {
  if (!name || !command) {
    return;
  }

  executing = true;

  try {
    const outputObject = await doCommand(client, name, command);

    if (outputObject) {
      output = JSON.stringify(outputObject, null, '\t');
    } else {
      notify.error(`Invalid response when executing command on ${name}`);
    }
  } catch (error) {
    notify.error(`Error executing command on ${name}: ${error}`);
  }

  executing = false;
};

const handleSelectComponent = (event: CustomEvent) => {
  selectedComponent = event.detail.value;
};

const handleEditorInput = (event: CustomEvent) => {
  input = event.detail.value;
};

const namesToPrettySelect = (resourcesToPretty: commonApi.ResourceName.AsObject[]): string => {
  const simple = new Map<string, number>();

  for (const resource of resourcesToPretty) {
    if (!simple.has(resource.name)) {
      simple.set(resource.name, 0);
    }
    simple.set(resource.name, simple.get(resource.name)! + 1);
  }

  return resourcesToPretty.map((res) => {
    if (simple.get(res.name) === 1) {
      return res.name;
    }
    return resourceNameToString(res);
  }).join(',');
};

</script>

<Collapse title="DoCommand()">
  <div class="h-full w-full border border-t-0 border-medium p-4">
    <v-select
      label="Selected component"
      placeholder="Select a component"
      options={namesToPrettySelect(resources)}
      value={selectedComponent}
      disabled={executing ? 'true' : 'false'}
      class="mb-4"
      on:input={handleSelectComponent}
    />
    <div class="flex h-full w-full flex-row flex-wrap gap-2">
      <div class="h-full w-full">
        <p class="text-sm">
          Input
        </p>
        <div class="h-[250px] w-full max-w-full border border-medium p-2">
          <v-code-editor
            language="json"
            value={'{}'}
            on:input={handleEditorInput}
          />
        </div>
      </div>
      <div class="flex min-w-[90px] flex-col justify-center">
        <v-button
          variant="inverse-primary"
          label={executing ? 'RUNNING...' : 'DO'}
          disabled={!selectedComponent || !input || executing ? 'true' : 'false'}
          on:click={() => handleDoCommand(selectedComponent, input)}
        />
      </div>
      <div class="h-full w-full">
        <p class="text-sm">
          Output
        </p>
        <div class="h-[250px] w-full border border-medium p-2">
          <v-code-editor
            language="json"
            value={output}
            readonly="true"
          />
        </div>
      </div>
    </div>
  </div>
</Collapse>
