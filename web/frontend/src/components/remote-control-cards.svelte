<!-- eslint-disable require-atomic-updates -->
<script lang="ts">

import { onMount, onDestroy } from 'svelte';
import { grpc } from '@improbable-eng/grpc-web';
import { Duration } from 'google-protobuf/google/protobuf/duration_pb';
import { type Credentials, ConnectionClosedError } from '@viamrobotics/rpc';
import { notify } from '@viamrobotics/prime';
import { displayError } from '@/lib/error';
import { resources, components, services, statuses } from '@/stores/resources';
import { statusStream } from '@/stores/streams';
import { client as clientStore } from '@/stores/client';
import { StreamManager } from './camera/stream-manager';
import { fetchCurrentOps } from '@/api/robot';
import {
  Client,
  type ServiceError,
  commonApi,
  robotApi,
  sensorsApi,
} from '@viamrobotics/sdk';

import {
  resourceNameToString,
  filterWithStatus,
  filterSubtype,
} from '@/lib/resource';

import Arm from './arm/index.svelte';
import AudioInput from './audio-input/index.svelte';
import Base from './base/index.svelte';
import Board from './board/index.svelte';
import CamerasList from './camera/index.svelte';
import OperationsSessions from './operations-sessions/index.svelte';
import DoCommand from './do-command/index.svelte';
import Encoder from './encoder/index.svelte';
import Gantry from './gantry/index.svelte';
import Gripper from './gripper/index.svelte';
import Gamepad from './gamepad/index.svelte';
import InputController from './input-controller/index.svelte';
import Motor from './motor/index.svelte';
import MovementSensor from './movement-sensor/index.svelte';
import Navigation from './navigation/index.svelte';
import Servo from './servo/index.svelte';
import Sensors from './sensors/index.svelte';
import Slam from './slam/index.svelte';

export let host: string;
export let bakedAuth: { authEntity: string; creds: Credentials; } | undefined;
export let supportedAuthTypes: string[];
export let webrtcEnabled: boolean;
export let signalingAddress: string;

const relevantSubtypesForStatus = [
  'arm',
  'gantry',
  'board',
  'servo',
  'motor',
  'input_controller',
] as const;

const impliedURL = `${location.protocol}//${location.hostname}${location.port ? `:${location.port}` : ''}`;
const client = new Client(impliedURL, {
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
   * SDK because the Remote Control implements it's own reconnection management.
   *
   * The Typescript SDK only manages reconnections for WebRTC connections - once
   * it can manage reconnections for direct gRPC connections, then we remove
   * reconnection management from the Remote Control panel entirely and just rely
   * on the Typescript SDK for that.
   */
  noReconnect: true,
});

clientStore.set(client);

const streamManager = new StreamManager(client);

interface ConnectionManager {
  statuses: {
    resources: boolean;
    ops: boolean;
  };
  timeout: number;
  stop(): void;
  start(): void;
  isConnected(): boolean;
  rtt: number;
}

const errors: Record<string, boolean> = {};

let appConnectionManager: ConnectionManager | null = null;
let password = '';
let showAuth = true;
let isConnecting = false;
let lastStatusTS: number | null = null;
let disableAuthElements = false;
let currentOps: { op: robotApi.Operation.AsObject; elapsed: number }[] = [];
let currentSessions: robotApi.Session.AsObject[] = [];
let sensorNames: commonApi.ResourceName.AsObject[] = [];
let resourcesOnce = false;
let errorMessage = '';
let connectedOnce = false;
let sessionsSupported = true;
let connectedFirstTimeResolve: (value: void) => void;

const connectedFirstTime = new Promise<void>((resolve) => {
  connectedFirstTimeResolve = resolve;
});

const resourceStatusByName = (resource: commonApi.ResourceName.AsObject) => {
  return $statuses[resourceNameToString(resource)];
};

// TODO (APP-146): replace these with constants
$: filteredWebGamepads = $components.filter((component) => {
  const remSplit = component.name.split(':');
  return (
    component.subtype === 'input_controller' &&
    Boolean(component.name) &&
    remSplit[remSplit.length - 1] === 'WebGamepad'
  );
});

/*
 * TODO (APP-146): replace these with constants
 * filters out WebGamepad
 */
$: filteredInputControllerList = $components.filter((component) => {
  const remSplit = component.name.split(':');
  return (
    component.subtype === 'input_controller' &&
    Boolean(component.name) &&
    remSplit[remSplit.length - 1] !== 'WebGamepad' && resourceStatusByName(component)
  );
});

const getStatus = (statusMap: Record<string, unknown>, resource: commonApi.ResourceName.AsObject) => {
  const key = resourceNameToString(resource);
  // todo(mp) Find a way to fix this type error
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return key ? statusMap[key] as any : undefined;
};

const handleError = (message: string, error: unknown, onceKey: string) => {
  if (onceKey) {
    if (errors[onceKey]) {
      return;
    }

    errors[onceKey] = true;
  }

  notify.danger(message);
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

const querySensors = () => {
  const sensorsName = filterSubtype($resources, 'sensors', { remote: false })[0]?.name;
  if (sensorsName === undefined) {
    return;
  }
  const req = new sensorsApi.GetSensorsRequest();
  req.setName(sensorsName);
  client.sensorsService.getSensors(
    req,
    new grpc.Metadata(),
    (err: ServiceError | null, resp: sensorsApi.GetSensorsResponse | null) => {
      if (err) {
        return displayError(err);
      }
      sensorNames = resp!.toObject().sensorNamesList;
    }
  );
};

const updateStatus = (grpcStatuses: robotApi.Status[]) => {
  for (const grpcStatus of grpcStatuses) {
    const nameObj = grpcStatus.getName()!.toObject();
    const statusJs = grpcStatus.getStatus()!.toJavaScript();
    const name = resourceNameToString(nameObj);

    $statuses[name] = statusJs;
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

  $statusStream = client.robotService.streamStatus(streamReq);
  if ($statusStream !== null) {
    $statusStream.on('data', (response: { getStatusList(): robotApi.Status[] }) => {
      updateStatus(response.getStatusList());
      lastStatusTS = Date.now();
    });
    $statusStream.on('status', (newStatus?: { details: unknown }) => {
      if (!ConnectionClosedError.isError(newStatus!.details)) {
        console.error('error streaming robot status', newStatus);
      }
      $statusStream = null;
    });
    $statusStream.on('end', () => {
      console.error('done streaming robot status');
      $statusStream = null;
    });
  }
};

// query metadata service every 0.5s
const queryMetadata = () => {
  return new Promise((resolve, reject) => {
    let resourcesChanged = false;
    let shouldRestartStatusStream = !(resourcesOnce && $statusStream);

    client.robotService.resourceNames(
      new robotApi.ResourceNamesRequest(),
      new grpc.Metadata(),
      (err: ServiceError | null, resp: robotApi.ResourceNamesResponse | null) => {
        if (err) {
          reject(err);
          return;
        }

        if (!resp) {
          reject(new Error('An unexpected issue occured.'));
          return;
        }

        const { resourcesList } = resp.toObject();

        const differences: Set<string> = new Set(
          $resources.map((name) => resourceNameToString(name))
        );
        const resourceSet: Set<string> = new Set(
          resourcesList.map((name: commonApi.ResourceName.AsObject) => resourceNameToString(name))
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
        if (resourcesChanged === true) {
          querySensors();
        }
        if (shouldRestartStatusStream === true) {
          restartStatusStream();
        }

        resolve(null);
      }
    );
  });
};

const loadCurrentOps = async () => {
  let now = Date.now();
  const list = await fetchCurrentOps(client);

  if (appConnectionManager) {
    appConnectionManager.rtt = Math.max(Date.now() - now, 0);
  }
  currentOps = [];

  now = Date.now();

  for (const op of list) {
    currentOps.push({
      op,
      elapsed: op.started ? now - (op.started.seconds * 1000) : -1,
    });
  }

  currentOps.sort((op1, op2) => {
    if (op1.elapsed === -1 || op2.elapsed === -1) {
      // move op with null start time to the back of the list
      return op2.elapsed - op1.elapsed;
    }
    return op1.elapsed - op2.elapsed;
  });

  return currentOps;
};

const fetchCurrentSessions = () => {
  if (!sessionsSupported) {
    return [];
  }
  return new Promise<robotApi.Session.AsObject[]>((resolve, reject) => {
    const req = new robotApi.GetSessionsRequest();

    client.robotService.getSessions(
      req,
      new grpc.Metadata(),
      (err: ServiceError | null, resp: robotApi.GetSessionsResponse | null) => {
        if (err) {
          if (err.code === grpc.Code.Unimplemented) {
            sessionsSupported = false;
          }
          reject(err);
          return;
        }

        if (!resp) {
          reject(new Error('An unexpected issue occurred.'));
          return;
        }

        const list = resp.toObject().sessionsList as { id: string }[];
        list.sort((sess1, sess2) => {
          return sess1.id < sess2.id ? -1 : 1;
        });
        resolve(list);
      }
    );
  });
};

const createAppConnectionManager = () => {
  const checkIntervalMillis = 10_000;
  const connections = {
    resources: false,
    ops: false,
    sessions: false,
  };

  let timeout = -1;
  let connectionRestablished = false;
  const rtt = 0;

  const isConnected = () => {
    return (
      connections.resources &&
      connections.ops &&
      // check status on interval if direct grpc
      (webrtcEnabled || (Date.now() - lastStatusTS! <= checkIntervalMillis))
    );
  };

  const manageLoop = async () => {
    try {
      const newErrors: unknown[] = [];

      try {
        await queryMetadata();

        if (!connections.resources) {
          connectionRestablished = true;
        }

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
          await loadCurrentOps();

          if (!connections.ops) {
            connectionRestablished = true;
          }

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
          currentSessions = await fetchCurrentSessions();

          if (!connections.sessions) {
            connectionRestablished = true;
          }

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
        if (connectionRestablished) {
          notify.success('Connection established');
          connectionRestablished = false;
        }

        errorMessage = '';
        return;
      }

      if (newErrors.length > 0) {
        handleCallErrors(connections, newErrors);
      }
      errorMessage = 'Connection lost, attempting to reconnect ...';

      try {
        console.debug('reconnecting');

        // reset status/stream state
        if ($statusStream) {
          $statusStream.cancel();
          $statusStream = null;
        }
        resourcesOnce = false;

        await client.connect();

        const now = Date.now();
        await fetchCurrentOps(client);
        if (appConnectionManager) {
          appConnectionManager.rtt = Math.max(Date.now() - now, 0);
        }

        lastStatusTS = Date.now();
        console.debug('reconnected');
        streamManager.refreshStreams();
      } catch (error) {
        if (ConnectionClosedError.isError(error)) {
          console.error('failed to reconnect; retrying');
        } else {
          console.error('failed to reconnect; retrying:', error);
        }
      }
    } finally {
      timeout = window.setTimeout(manageLoop, 500);
    }
  };

  const stop = () => {
    window.clearTimeout(timeout);
    $statusStream?.cancel();
    $statusStream = null;
  };

  const start = () => {
    stop();
    lastStatusTS = Date.now();
    manageLoop();
  };

  return {
    statuses: connections,
    timeout,
    stop,
    start,
    isConnected,
    rtt,
  };
};

appConnectionManager = createAppConnectionManager();

const nonEmpty = (object: object) => {
  return Object.keys(object).length > 0;
};

const doConnect = async (authEntity: string, creds: Credentials, onError?: (reason?: unknown) => void) => {
  console.debug('connecting');
  isConnecting = true;

  try {
    await client.connect(authEntity, creds);

    console.debug('connected');
    showAuth = false;
    disableAuthElements = false;
    connectedOnce = true;
    connectedFirstTimeResolve();
  } catch (error) {
    console.error('failed to connect:', error);
    if (onError) {
      onError(error);
    } else {
      notify.danger('failed to connect');
    }
  }
};

const doLogin = (authType: string) => {
  disableAuthElements = true;
  const creds = { type: authType, payload: password };
  doConnect(host, creds, (error) => {
    isConnecting = false;
    disableAuthElements = false;
    console.error(error);
    notify.danger(`failed to connect: ${error}`);
  });
};

const initConnect = () => {
  if (supportedAuthTypes.length === 0) {
    doConnect(bakedAuth!.authEntity, bakedAuth!.creds, () => {
      notify.danger('failed to connect; retrying');
      setTimeout(initConnect, 1000);
    });
  }
};

const handleUnload = () => {
  console.debug('disconnecting');
  appConnectionManager?.stop();
  streamManager?.close();
  client.disconnect();
};

onMount(async () => {
  initConnect();
  await connectedFirstTime;

  appConnectionManager?.start();

  window.addEventListener('beforeunload', handleUnload);
});

onDestroy(() => {
  handleUnload();
  window.removeEventListener('beforeunload', handleUnload);
});

</script>

<div class="remote-control">
  {#if showAuth}
    <div>
      {#if isConnecting}
        <v-notify
          variant='info'
          title={`Connecting via ${webrtcEnabled ? 'WebRTC' : 'gRPC'}...`}
        />
      {/if}

      {#each supportedAuthTypes as authType (authType)}
        <div class="px-4 py-3">
          <span>{authType}: </span>
          <div class="w-96">
            <input
              bind:value={password}
              disabled={disableAuthElements}
              class="
                mb-2 block w-full appearance-none border p-2 text-gray-700
                transition-colors duration-150 ease-in-out placeholder:text-gray-400 focus:outline-none
              "
              type="password"
              autocomplete="off"
              on:keyup={(event) => event.key === 'Enter' && doLogin(authType)}
            >
            <v-button
              disabled={disableAuthElements}
              label="Login"
              on:click={disableAuthElements ? undefined : () => doLogin(authType)}
            />
          </div>
        </div>
      {/each}
    </div>
  {/if}

  <div class="flex flex-col gap-4 p-3">
    {#if errorMessage}
      <v-notify
        variant='danger'
        title={errorMessage}
      />
    {/if}

    <!-- ******* BASE *******  -->
    {#each filterSubtype($components, 'base') as { name } (name)}
      <Base
        {name}
        {client}
        {streamManager}
        statusStream={$statusStream}
      />
    {/each}

    <!-- ******* ENCODER *******  -->
    {#each filterSubtype($components, 'encoder') as { name } (name)}
      <Encoder
        {name}
        {client}
        statusStream={$statusStream}
      />
    {/each}

    <!-- ******* GANTRY *******  -->
    {#each filterWithStatus($components, $statuses, 'gantry') as gantry (gantry.name)}
      <Gantry
        name={gantry.name}
        {client}
        status={getStatus($statuses, gantry)}
      />
    {/each}

    <!-- ******* MOVEMENT SENSOR *******  -->
    {#each filterSubtype($components, 'movement_sensor') as { name } (name)}
      <MovementSensor
        {name}
        {client}
        statusStream={$statusStream}
      />
    {/each}

    <!-- ******* ARM *******  -->
    {#each filterSubtype($components, 'arm') as arm (arm.name)}
      <Arm
        name={arm.name}
        {client}
        status={getStatus($statuses, arm)}
      />
    {/each}

    <!-- ******* GRIPPER *******  -->
    {#each filterSubtype($components, 'gripper') as { name } (name)}
      <Gripper
        {name}
        {client}
      />
    {/each}

    <!-- ******* SERVO *******  -->
    {#each filterWithStatus($components, $statuses, 'servo') as servo (servo.name)}
      <Servo
        name={servo.name}
        {client}
        status={getStatus($statuses, servo)}
      />
    {/each}

    <!-- ******* MOTOR *******  -->
    {#each filterWithStatus($components, $statuses, 'motor') as motor (motor.name)}
      <Motor
        name={motor.name}
        {client}
        status={getStatus($statuses, motor)}
      />
    {/each}

    <!-- ******* INPUT VIEW *******  -->
    {#each filteredInputControllerList as controller (controller.name)}
      <InputController
        name={controller.name}
        status={getStatus($statuses, controller)}
      />
    {/each}

    <!-- ******* WEB CONTROLS *******  -->
    {#each filteredWebGamepads as { name } (name)}
      <Gamepad
        {name}
        {client}
        statusStream={$statusStream}
      />
    {/each}

    <!-- ******* BOARD *******  -->
    {#each filterWithStatus($components, $statuses, 'board') as board (board.name)}
      <Board
        name={board.name}
        {client}
        status={getStatus($statuses, board)}
      />
    {/each}

    <!-- ******* CAMERA *******  -->
    <CamerasList
      {client}
      {streamManager}
      statusStream={$statusStream}
      resources={filterSubtype($components, 'camera')}
    />

    <!-- ******* NAVIGATION *******  -->
    {#each filterSubtype($services, 'navigation') as { name } (name)}
      <Navigation
        {name}
        {client}
      />
    {/each}

    <!-- ******* SENSOR *******  -->
    {#if nonEmpty(sensorNames)}
      <Sensors
        name={filterSubtype($resources, 'sensors', { remote: false })[0]?.name ?? ''}
        {client}
        {sensorNames}
      />
    {/if}

    <!-- ******* AUDIO *******  -->
    {#each filterSubtype($components, 'audio_input') as { name } (name)}
      <AudioInput
        {name}
        {client}
      />
    {/each}

    <!-- ******* SLAM *******  -->
    {#each filterSubtype($services, 'slam') as { name } (name)}
      <Slam
        {name}
        {client}
        statusStream={$statusStream}
        operations={currentOps}
      />
    {/each}

    <!-- ******* DO *******  -->
    {#if filterSubtype($components, 'generic').length > 0}
      <DoCommand
        {client}
        resources={filterSubtype($components, 'generic')}
      />
    {/if}

    <!-- ******* OPERATIONS AND SESSIONS *******  -->
    {#if connectedOnce && appConnectionManager}
      <OperationsSessions
        {client}
        {sessionsSupported}
        operations={currentOps}
        sessions={currentSessions}
        connectionManager={appConnectionManager}
      />
    {/if}
  </div>
</div>
