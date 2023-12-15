<script lang='ts'>

/* eslint-disable require-atomic-updates */
import { grpc } from '@improbable-eng/grpc-web';
import { Duration } from 'google-protobuf/google/protobuf/duration_pb';
import { onMount, onDestroy, createEventDispatcher } from 'svelte';
import { type Credentials, ConnectionClosedError } from '@viamrobotics/rpc';
import { Client, robotApi, commonApi, type ServiceError } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { StreamManager } from '@/lib/stream-manager';
import { getOperations, getResourceNames, getSessions } from '@/api/robot';
import { getSensors } from '@/api/sensors';
import { useRobotClient } from '@/hooks/robot-client';
import { setAsyncInterval } from '@/lib/schedule';
import { resourceNameToString, filterSubtype } from '@/lib/resource';

export let webrtcEnabled: boolean;
export let host: string;
export let signalingAddress: string;
export let bakedAuth: { authEntity?: string; creds?: Credentials; } = {};
export let supportedAuthTypes: string[] = [];

const {
  robotClient,
  operations,
  sessions,
  sessionsSupported,
  resources,
  sensorNames,
  statuses,
  statusStream,
  streamManager,
  rtt,
  connectionStatus,
  components,
} = useRobotClient();

const dispatch = createEventDispatcher<{
  'connection-error': unknown
}>();


const relevantSubtypesForStatus = [
  'arm',
  'gantry',
  'board',
  'servo',
  'motor',
  'input_controller',
] as const;

const apiKeyAuthType = "api-key"
const urlPort = location.port ? `:${location.port}` : ''
const impliedURL = `${location.protocol}//${location.hostname}${urlPort}`;

$robotClient = new Client(impliedURL, {
  enabled: webrtcEnabled,
  host,
  signalingAddress,
  rtcConfig: {
    iceServers: [
      {
        urls: 'stun:global.stun.twilio.com:3478',
      },
    ],
  },

  /*
   * TODO(RSDK-3183): Opt out of reconnection management in the Typescript
   * SDK because the Remote Control implements its own reconnection management.
   *
   * The Typescript SDK only manages reconnections for WebRTC connections - once
   * it can manage reconnections for direct gRPC connections, then we remove
   * reconnection management from the Remote Control panel entirely and just rely
   * on the Typescript SDK for that.
   */
  noReconnect: true,
});

const passwordByAuthType: Record<string, string> = {}
let apiKeyEntity = "";
for (const auth of supportedAuthTypes) {
  passwordByAuthType[auth] = ""
}

let lastStatusTS: number | null = null;
let resourcesOnce = false;

$streamManager = new StreamManager($robotClient);

const errors: Record<string, boolean> = {};

const handleError = (message: string, error: unknown, onceKey: string) => {
  if (onceKey) {
    if (errors[onceKey]) {
      return;
    }

    errors[onceKey] = true;
  }

  notify.danger(message);
  
  // eslint-disable-next-line no-console
  console.error(message, { error });
};

const handleCallErrors = (list: { resources: boolean; ops: boolean }, newErrors: unknown) => {
  const errorsList = document.createElement('ul');
  errorsList.classList.add('list-disc', 'pl-4');

  for (const key of Object.keys(list)) {
    switch (key) {
      case 'resources': {
        errorsList.innerHTML += '<li>Robot Resources</li>';
        break;
      }
      case 'ops': {
        errorsList.innerHTML += '<li>Current Operations</li>';
        break;
      }
    }
  }

  handleError(
    `Error fetching the following: ${errorsList.outerHTML}`,
    newErrors,
    'connection'
  );
};

const stringToResourceName = (nameStr: string) => {
  const [prefix, suffix] = nameStr.split('/');
  let name = '';

  if (suffix) {
    name = suffix;
  }

  const subtypeParts = prefix!.split(':');
  if (subtypeParts.length > 3) {
    throw new Error('more than 2 colons in resource name string');
  }

  if (subtypeParts.length < 3) {
    throw new Error('less than 2 colons in resource name string');
  }

  return {
    namespace: subtypeParts[0],
    type: subtypeParts[1],
    subtype: subtypeParts[2],
    name,
  };
};

const loadCurrentOps = async () => {
  const now = Date.now();
  const list = await getOperations($robotClient);
  const ops = [];

  $rtt = Math.max(Date.now() - now, 0);

  for (const op of list) {
    ops.push({
      op,
      elapsed: op.started ? Date.now() - (op.started.seconds * 1000) : -1,
    });
  }

  ops.sort((op1, op2) => {
    if (op1.elapsed === -1 || op2.elapsed === -1) {
      // move op with null start time to the back of the list
      return op2.elapsed - op1.elapsed;
    }
    return op1.elapsed - op2.elapsed;
  });

  return ops;
};

const fetchCurrentSessions = async () => {
  if (!$sessionsSupported) {
    return [];
  }

  try {
    const list = await getSessions($robotClient);
    list.sort((sess1, sess2) => (sess1.id < sess2.id ? -1 : 1));
    return list;
  } catch (error) {
    const serviceError = error as ServiceError;
    if (serviceError.code === grpc.Code.Unimplemented) {
      $sessionsSupported = false;
    }

    return [];
  }
};

const updateStatus = (grpcStatuses: robotApi.Status[]) => {
  for (const grpcStatus of grpcStatuses) {
    const nameObj = grpcStatus.getName()!.toObject();
    const status = grpcStatus.getStatus()!.toJavaScript();
    const name = resourceNameToString(nameObj);

    $statuses[name] = status;
  }
};

const restartStatusStream = () => {
  if ($statusStream) {
    $statusStream.cancel();
    $statusStream = null;
  }

  let newResources: commonApi.ResourceName.AsObject[] = [];

  // get all relevant resources
  for (const subtype of relevantSubtypesForStatus) {
    newResources = [...newResources, ...filterSubtype($components, subtype)];
  }

  const names = newResources.map((name) => {
    const resourceName = new commonApi.ResourceName();
    resourceName.setNamespace(name.namespace);
    resourceName.setType(name.type);
    resourceName.setSubtype(name.subtype);
    resourceName.setName(name.name);
    return resourceName;
  });

  const streamReq = new robotApi.StreamStatusRequest();
  streamReq.setResourceNamesList(names);
  streamReq.setEvery(new Duration().setNanos(500_000_000));

  $statusStream = $robotClient.robotService.streamStatus(streamReq);
  $statusStream.on('data', (response: { getStatusList(): robotApi.Status[] }) => {
    updateStatus(response.getStatusList());
    lastStatusTS = Date.now();
  });
  $statusStream.on('status', (newStatus?: { details: unknown }) => {
    if (!ConnectionClosedError.isError(newStatus!.details)) {
      // eslint-disable-next-line no-console
      console.error('error streaming robot status', newStatus);
    }
    $statusStream = null;
  });
  $statusStream.on('end', () => {
    // eslint-disable-next-line no-console
    console.error('done streaming robot status');
    $statusStream = null;
  });
};

// query metadata service every 0.5s
const queryMetadata = async () => {
  let resourcesChanged = false;
  let shouldRestartStatusStream = !(resourcesOnce && $statusStream);

  const resourcesList = await getResourceNames($robotClient);

  const differences = new Set<string>(
    $resources.map((name) => resourceNameToString(name))
  );
  const resourceSet = new Set<string>(
    resourcesList.map((name) => resourceNameToString(name))
  );

  for (const elem of resourceSet) {
    if (differences.has(elem)) {
      differences.delete(elem);
    } else {
      differences.add(elem);
    }
  }

  if (differences.size > 0) {
    resourcesChanged = true;

    // restart status stream if resource difference includes a resource we care about
    for (const elem of differences) {
      const resource = stringToResourceName(elem);
      if (
        resource.namespace === 'rdk' &&
        resource.type === 'component' &&
        relevantSubtypesForStatus.includes(resource.subtype as typeof relevantSubtypesForStatus[number])
      ) {
        shouldRestartStatusStream = true;
        break;
      }
    }
  }

  $resources = resourcesList;

  resourcesOnce = true;
  if (resourcesChanged) {
    const sensorsName = filterSubtype(resources.current, 'sensors', { remote: false })[0]?.name;

    $sensorNames = sensorsName === undefined ? [] : (await getSensors($robotClient, sensorsName));

  }

  if (shouldRestartStatusStream) {
    restartStatusStream();
  }
};

const checkIntervalMillis = 10_000;

const connections = {
  resources: false,
  ops: false,
  sessions: false,
};

let cancelTick: undefined | (() => void);

const isConnected = () => {
  return (
    connections.resources &&
    connections.ops &&
    // check status on interval if direct grpc
    (webrtcEnabled || (Date.now() - lastStatusTS! <= checkIntervalMillis))
  );
};

// eslint-disable-next-line sonarjs/cognitive-complexity
const tick = async () => {
  const newErrors: unknown[] = [];

  try {
    await queryMetadata();
    connections.resources = true;
  } catch (error) {
    if (ConnectionClosedError.isError(error)) {
      connections.resources = false;
    } else {
      newErrors.push(error);
    }
  }

  if (connections.resources) {
    try {
      $operations = await loadCurrentOps();
      connections.ops = true;
    } catch (error) {
      if (ConnectionClosedError.isError(error)) {
        connections.ops = false;
      } else {
        newErrors.push(error);
      }
    }
  }

  if (connections.ops) {
    try {
      $sessions = await fetchCurrentSessions();
      connections.sessions = true;
    } catch (error) {
      if (ConnectionClosedError.isError(error)) {
        connections.sessions = false;
      } else {
        newErrors.push(error);
      }
    }
  }

  if (isConnected()) {
    $connectionStatus = 'connected';
    return;
  }

  if (newErrors.length > 0) {
    handleCallErrors(connections, newErrors);
  }

  $connectionStatus = 'reconnecting';

  try {
    // eslint-disable-next-line no-console
    console.debug('reconnecting');

    // reset status/stream state
    if ($statusStream) {
      $statusStream.cancel();
      $statusStream = null;
    }
    resourcesOnce = false;

    await $robotClient.connect({ priority: 1 });

    const now = Date.now();

    $rtt = Math.max(Date.now() - now, 0);

    lastStatusTS = Date.now();
    // eslint-disable-next-line no-console
    console.debug('reconnected');
    $streamManager.refreshStreams();
  } catch (error) {
    if (ConnectionClosedError.isError(error)) {
      // eslint-disable-next-line no-console
      console.error('failed to reconnect; retrying');
    } else {
      // eslint-disable-next-line no-console
      console.error('failed to reconnect; retrying:', error);
    }
  }
};

const stop = () => {
  cancelTick?.();
  $statusStream?.cancel();
  $statusStream = null;
};

const start = () => {
  stop();
  lastStatusTS = Date.now();
  tick();
  cancelTick = setAsyncInterval(tick, 500);
};

const connect = async (creds?: Credentials, authEntity?: string) => {
  $connectionStatus = 'connecting';

  await $robotClient.connect({ 
    authEntity: authEntity ?? bakedAuth.authEntity,
    creds: creds ?? bakedAuth.creds,
    priority: 1 
  });

  $connectionStatus = 'connected';
  start();
};

const login = async (authType: string) => {
  const creds = { type: authType, payload: passwordByAuthType[authType] ?? ""};

  const authEntity = authType === apiKeyAuthType ? apiKeyEntity: undefined
  try {
    await connect(creds, authEntity);
  } catch (error) {
    notify.danger(`failed to connect: ${error as string}`);
    $connectionStatus = 'idle';
  }
};

/*
 * If the component is unmounted during the init setTimeout evaluations,
 * nothing will stop init from calling setTimeout and trying to reconnect
 * again. This boolean is used to track whether the component is mounted
 * and explicitly stop trying to connect.
 */
let isMounted = false;

const init = async () => {
  try {
    await connect();
  } catch (error) {
    dispatch('connection-error', error);
    if (isMounted) {
      setTimeout(init);
    }
  }
};

const handleUnload = () => {
  stop();
  $streamManager.close();
  $robotClient.disconnect();
};

onMount(() => {
  isMounted = true;
  window.addEventListener('beforeunload', handleUnload);
});

onDestroy(() => {
  isMounted = false;
  handleUnload();
  window.removeEventListener('beforeunload', handleUnload);
});

if (supportedAuthTypes.length === 0) {
  init();
}
let selectedAuthType: string = supportedAuthTypes[0]!

</script>

{#if $connectionStatus === 'connecting'}
  <slot name='connecting' />
{:else if $connectionStatus === 'reconnecting'}
  <slot name='reconnecting' />
{/if}

{#if $connectionStatus === 'connected' || $connectionStatus === 'reconnecting'}
  <slot />
{:else if supportedAuthTypes.length > 0 }
  <div class="flex bg-[#f7f7f8] min-h-[100vh]">
  <div class="flex flex-col items-center w-full h-full md:max-w-[400px] md:h-auto bg-white border border-[#d7d7d9] m-auto p-6 pt-10">
      <img src="https://app.viam.com/static/images/viam-logo.svg" alt="Viam"
           class="mb-8 h-8" />
      <div class="flex flex-row w-full mb-8">
          {#each supportedAuthTypes as authType(authType)}
              <button
                class={`flex w-full h-10 items-center justify-center text-default font-medium text-sm disabled ${selectedAuthType===authType ? "bg-[#FBFBFC] border-[#C5C6CC]" : "text-[#4E4F52] bg-[#F1F1F4] border-[#E4E4E6]"} border`}
                disabled={selectedAuthType===authType} aria-disabled={selectedAuthType===authType}
                on:click={() => {selectedAuthType = authType;}}>
                  {authType}
              </button>
          {/each}
      </div>
      <div class="w-full">
        {#if selectedAuthType === apiKeyAuthType}
          <label class="block text-xs text-[#4E4F52] leading-3 mb-2">
            api key id
            <input
                bind:value={apiKeyEntity}
                disabled={$connectionStatus === 'connecting'}
                class="border border-[#E4E4E6] text-sm block w-full p-2.5 mt-2"
                autocomplete="off">
          </label>
        {/if}
        <label class="block text-xs text-[#4E4F52] leading-3">
            {selectedAuthType === apiKeyAuthType ? "api key" : "secret" }
          <input
             bind:value={passwordByAuthType[selectedAuthType]}
             disabled={$connectionStatus === 'connecting'}
             class="border border-[#E4E4E6] text-sm block w-full p-2.5 mt-2"
             type="password"
             autocomplete="off"
             on:keyup={async (event) => event.key === 'Enter' && login(selectedAuthType)}
          >
        </label>
        <button
          disabled={$connectionStatus === 'connecting'}
          class="block w-full h-10 p-2 mt-8 mb-2 bg-[#282829] text-sm text-white disabled:cursor-not-allowed disabled:opacity-50 disabled:pointer-events-none"
          on:click={$connectionStatus === 'connecting' ? undefined : async () => login(selectedAuthType)}
        >
          Log in
        </button>
      </div>
    </div>
  </div>
{/if}
