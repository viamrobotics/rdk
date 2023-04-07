<!-- eslint-disable require-atomic-updates -->
<script setup lang="ts">

import { onMounted, onUnmounted } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { Duration } from 'google-protobuf/google/protobuf/duration_pb';
import { type Credentials, ConnectionClosedError } from '@viamrobotics/rpc';
import { toast } from '../lib/toast';
import { displayError } from '../lib/error';
import { addResizeListeners } from '../lib/resize';
import { StreamManager } from './camera/stream-manager';
import {
  Client,
  ResponseStream,
  ServiceError,
  commonApi,
  robotApi,
  sensorsApi,
} from '@viamrobotics/sdk';

import {
  resourceNameToSubtypeString,
  resourceNameToString,
  filterResources,
  filterNonRemoteResources,
  filterRdkComponentsWithStatus,
  filterComponentsWithNames,
} from '../lib/resource';

import Arm from './arm.vue';
import AudioInput from './audio-input.vue';
import Base from './base.vue';
import Board from './board.vue';
import CamerasList from './camera/cameras-list.vue';
import OperationsSessions from './operations-sessions.vue';
import DoCommand from './do-command.vue';
import Gantry from './gantry.vue';
import Gripper from './gripper.vue';
import Gamepad from './gamepad.vue';
import InputController from './input-controller.vue';
import Motor from './motor-detail.vue';
import MovementSensor from './movement-sensor.vue';
import Navigation from './navigation.vue';
import ServoComponent from './servo.vue';
import Sensors from './sensors.vue';
import Slam from './slam.vue';

import {
  fixArmStatus,
  fixBoardStatus,
  fixGantryStatus,
  fixInputStatus,
  fixMotorStatus,
  fixServoStatus,
} from '../lib/fixers';

const props = defineProps<{
  host: string;
  bakedAuth?: {
    authEntity: string;
    creds: Credentials;
  },
  supportedAuthTypes: string[],
  webrtcEnabled: boolean,
  client: Client;
}>();

const relevantSubtypesForStatus = [
  'arm',
  'gantry',
  'board',
  'servo',
  'motor',
  'input_controller',
];

const password = $ref<string>('');
const bakedAuth = $computed(() => props.bakedAuth || {} as { authEntity: string, creds: Credentials });
const supportedAuthTypes = $computed(() => props.supportedAuthTypes);
const rawStatus = $ref<Record<string, robotApi.Status>>({});
const status = $ref<Record<string, robotApi.Status>>({});
const errors = $ref<Record<string, boolean>>({});

let statusStream: ResponseStream<robotApi.StreamStatusResponse> | null = null;
let lastStatusTS: number | null = null;
let disableAuthElements = $ref(false);
let currentOps = $ref<{ op: robotApi.Operation.AsObject, elapsed: number }[]>([]);
let currentSessions = $ref<robotApi.Session.AsObject[]>([]);
let sensorNames = $ref<commonApi.ResourceName.AsObject[]>([]);
let resources = $ref<commonApi.ResourceName.AsObject[]>([]);
let resourcesOnce = false;
let errorMessage = $ref('');
let connectedOnce = $ref(false);
let connectedFirstTimeResolve: (value: void) => void;
const streamManager = new StreamManager(props.client);

const connectedFirstTime = new Promise<void>((resolve) => {
  connectedFirstTimeResolve = resolve;
});

let appConnectionManager = $ref<{
  statuses: {
    resources: boolean;
    ops: boolean;
  };
  timeout: number;
  stop(): void;
  start(): void;
  isConnected(): boolean;
  rtt: number;
}>(null!);

const handleError = (message: string, error: unknown, onceKey: string) => {
  if (onceKey) {
    if (errors[onceKey]) {
      return;
    }

    errors[onceKey] = true;
  }

  toast.error(message);
  console.error(message, { error });
};

const handleCallErrors = (statuses: { resources: boolean; ops: boolean }, newErrors: unknown) => {
  const errorsList = document.createElement('ul');
  errorsList.classList.add('list-disc', 'pl-4');

  for (const key of Object.keys(statuses)) {
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
  const sensorsName = filterNonRemoteResources(resources, 'rdk', 'service', 'sensors')[0]?.name;
  if (sensorsName === undefined) {
    return;
  }
  const req = new sensorsApi.GetSensorsRequest();
  req.setName(sensorsName);
  props.client.sensorsService.getSensors(
    req,
    new grpc.Metadata(),
    (err: ServiceError, resp: sensorsApi.GetSensorsResponse) => {
      if (err) {
        return displayError(err);
      }
      sensorNames = resp!.toObject().sensorNamesList;
    }
  );
};

const fixRawStatus = (resource: commonApi.ResourceName.AsObject, statusToFix: unknown) => {
  switch (resourceNameToSubtypeString(resource)) {

    /*
     * TODO (APP-146): generate these using constants
     * TODO these types need to be fixed.
     */
    case 'rdk:component:arm': {
      return fixArmStatus(statusToFix as never);
    }
    case 'rdk:component:board': {
      return fixBoardStatus(statusToFix as never);
    }
    case 'rdk:component:gantry': {
      return fixGantryStatus(statusToFix as never);
    }
    case 'rdk:component:input_controller': {
      return fixInputStatus(statusToFix as never);
    }
    case 'rdk:component:motor': {
      return fixMotorStatus(statusToFix as never);
    }
    case 'rdk:component:servo': {
      return fixServoStatus(statusToFix as never);
    }
  }

  return statusToFix;
};

const updateStatus = (grpcStatuses: robotApi.Status[]) => {
  for (const grpcStatus of grpcStatuses) {
    const nameObj = grpcStatus.getName()!.toObject();
    const statusJs = grpcStatus.getStatus()!.toJavaScript();

    try {
      const fixed = fixRawStatus(nameObj, statusJs);
      const name = resourceNameToString(nameObj);
      rawStatus[name] = statusJs as unknown as robotApi.Status;
      status[name] = fixed as unknown as robotApi.Status;
    } catch {
      toast.error(`Couldn't fix status for ${resourceNameToString(nameObj)}`);
    }
  }
};

const restartStatusStream = () => {
  if (statusStream) {
    statusStream.cancel();
    statusStream = null;
  }

  let newResources: commonApi.ResourceName.AsObject[] = [];

  // get all relevant resources
  for (const subtype of relevantSubtypesForStatus) {
    newResources = [...newResources, ...filterResources(newResources, 'rdk', 'component', subtype)];
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

  statusStream = props.client.robotService.streamStatus(streamReq);
  if (statusStream !== null) {
    statusStream.on('data', (response: { getStatusList(): robotApi.Status[] }) => {
      updateStatus((response).getStatusList());
      lastStatusTS = Date.now();
    });
    statusStream.on('status', (newStatus: { details: unknown }) => {
      if (!ConnectionClosedError.isError(newStatus.details)) {
        console.error('error streaming robot status', newStatus);
      }
      statusStream = null;
    });
    statusStream.on('end', () => {
      console.error('done streaming robot status');
      statusStream = null;
    });
  }
};

// query metadata service every 0.5s
const queryMetadata = () => {
  return new Promise((resolve, reject) => {
    let resourcesChanged = false;
    let shouldRestartStatusStream = !(resourcesOnce && statusStream);

    props.client.robotService.resourceNames(
      new robotApi.ResourceNamesRequest(),
      new grpc.Metadata(),
      (err: ServiceError, resp: robotApi.ResourceNamesResponse) => {
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
          resources.map((name: commonApi.ResourceName.AsObject) =>
            resourceNameToString(name))
        );
        const resourceSet: Set<string> = new Set(
          resourcesList.map((name: string) => resourceNameToString(name))
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
              relevantSubtypesForStatus.includes(resource.subtype!)
            ) {
              shouldRestartStatusStream = true;
              break;
            }
          }
        }

        resources = resourcesList;
        resourcesOnce = true;
        if (resourcesChanged === true) {
          querySensors();
        }
        if (shouldRestartStatusStream === true) {
          restartStatusStream();
        }
        resolve(resources);
      }
    );
  });
};

const fetchCurrentOps = () => {
  return new Promise<robotApi.Operation.AsObject[]>((resolve, reject) => {
    const req = new robotApi.GetOperationsRequest();

    const now = Date.now();
    props.client.robotService.getOperations(
      req,
      new grpc.Metadata(),
      (err: ServiceError, resp: robotApi.GetOperationsResponse) => {
        if (err) {
          reject(err);
          return;
        }
        appConnectionManager.rtt = Math.max(Date.now() - now, 0);

        if (!resp) {
          reject(new Error('An unexpected issue occurred.'));
          return;
        }

        const list = resp.toObject().operationsList;
        resolve(list);
      }
    );
  });
};

const loadCurrentOps = async () => {
  const list = await fetchCurrentOps();
  currentOps = [];

  const now = Date.now();
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

let sessionsSupported = $ref<boolean>(true);
const fetchCurrentSessions = () => {
  if (!sessionsSupported) {
    return [];
  }
  return new Promise<robotApi.Session.AsObject[]>((resolve, reject) => {
    const req = new robotApi.GetSessionsRequest();

    props.client.robotService.getSessions(
      req,
      new grpc.Metadata(),
      (err: ServiceError, resp: robotApi.GetSessionsResponse) => {
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

const isWebRtcEnabled = () => {
  return props.webrtcEnabled;
};

const createAppConnectionManager = () => {
  const checkIntervalMillis = 10_000;
  const statuses = {
    resources: false,
    ops: false,
    sessions: false,
  };

  let timeout = -1;
  let connectionRestablished = false;
  const rtt = 0;

  const isConnected = () => {
    return (
      statuses.resources &&
      statuses.ops &&
      // check status on interval if direct grpc
      (isWebRtcEnabled() || (Date.now() - lastStatusTS! <= checkIntervalMillis))
    );
  };

  const manageLoop = async () => {
    try {
      const newErrors = [];

      try {
        await queryMetadata();

        if (!statuses.resources) {
          connectionRestablished = true;
        }

        statuses.resources = true;
      } catch (error) {
        if (ConnectionClosedError.isError(error)) {
          statuses.resources = false;
        } else {
          newErrors.push(error);
        }
      }

      if (statuses.resources) {
        try {
          await loadCurrentOps();

          if (!statuses.ops) {
            connectionRestablished = true;
          }

          statuses.ops = true;
        } catch (error) {
          if (ConnectionClosedError.isError(error)) {
            statuses.ops = false;
          } else {
            newErrors.push(error);
          }
        }
      }

      if (statuses.ops) {
        try {
          currentSessions = await fetchCurrentSessions();

          if (!statuses.sessions) {
            connectionRestablished = true;
          }

          statuses.sessions = true;
        } catch (error) {
          if (ConnectionClosedError.isError(error)) {
            statuses.sessions = false;
          } else {
            newErrors.push(error);
          }
        }
      }

      if (isConnected()) {
        if (connectionRestablished) {
          toast.success('Connection established');
          connectionRestablished = false;
        }

        errorMessage = '';
        return;
      }

      if (newErrors.length > 0) {
        handleCallErrors(statuses, newErrors);
      }
      errorMessage = 'Connection lost, attempting to reconnect ...';

      try {
        console.log('reconnecting');

        // reset status/stream state
        if (statusStream) {
          statusStream.cancel();
          statusStream = null;
        }
        resourcesOnce = false;

        await props.client.connect();
        await fetchCurrentOps();
        lastStatusTS = Date.now();
        console.log('reconnected');
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
  };

  const start = () => {
    stop();
    lastStatusTS = Date.now();
    manageLoop();
  };

  return {
    statuses,
    timeout,
    stop,
    start,
    isConnected,
    rtt,
  };
};

appConnectionManager = createAppConnectionManager();

const resourceStatusByName = (resource: commonApi.ResourceName.AsObject) => {
  return status[resourceNameToString(resource)];
};

const rawResourceStatusByName = (resource: commonApi.ResourceName.AsObject) => {
  return rawStatus[resourceNameToString(resource)];
};

const filteredWebGamepads = () => {
  // TODO (APP-146): replace these with constants
  return filterComponentsWithNames(resources).filter((elem) => {
    if (!(elem.namespace === 'rdk' && elem.type === 'component' && elem.subtype === 'input_controller')) {
      return false;
    }
    const remSplit = elem.name.split(':');
    return remSplit[remSplit.length - 1] === 'WebGamepad';
  });
};

const filteredInputControllerList = () => {

  /*
   * TODO (APP-146): replace these with constants
   * filters out WebGamepad
   */
  return filterComponentsWithNames(resources).filter((elem) => {
    if (!(elem.namespace === 'rdk' && elem.type === 'component' && elem.subtype === 'input_controller')) {
      return false;
    }
    const remSplit = elem.name.split(':');
    return remSplit[remSplit.length - 1] !== 'WebGamepad' && resourceStatusByName(elem);
  });
};

const nonEmpty = (object: object) => {
  return Object.keys(object).length > 0;
};

const doConnect = async (authEntity: string, creds: Credentials, onError?: (reason?: unknown) => void) => {
  console.debug('connecting');
  document.querySelector('#connecting')!.classList.remove('hidden');

  try {
    await props.client.connect(authEntity, creds);
  } catch (error) {
    console.error('failed to connect:', error);
    if (onError) {
      onError(error);
    } else {
      toast.error('failed to connect');
    }
    return;
  }

  console.debug('connected');
  document.querySelector('#pre-app')!.classList.add('hidden');
  disableAuthElements = false;
  connectedOnce = true;
  connectedFirstTimeResolve();
};

const doLogin = (authType: string) => {
  disableAuthElements = true;
  const creds = { type: authType, payload: password };
  doConnect(props.host, creds, (error) => {
    document.querySelector('#connecting')!.classList.add('hidden');
    disableAuthElements = false;
    console.error(error);
    toast.error(`failed to connect: ${error}`);
  });
};

const initConnect = () => {
  if (supportedAuthTypes.length === 0) {
    doConnect(bakedAuth.authEntity, bakedAuth.creds, () => {
      toast.error('failed to connect; retrying');
      setTimeout(initConnect, 1000);
    });
  }
};

onMounted(async () => {
  initConnect();
  await connectedFirstTime;

  appConnectionManager.start();

  addResizeListeners();
});

onUnmounted(() => {
  appConnectionManager.stop();
});

</script>

<template>
  <div id="pre-app">
    <div
      id="connecting"
      class="border-greendark hidden border-l-4 bg-gray-100 px-4 py-3"
    >
      Connecting via <template v-if="isWebRtcEnabled()">
        WebRTC
      </template><template v-else>
        gRPC
      </template>...
    </div>

    <template
      v-for="authType in supportedAuthTypes"
      :key="authType"
    >
      <div class="px-4 py-3">
        <span>{{ authType }}: </span>
        <div class="w-96">
          <input
            v-model="password"
            :disabled="disableAuthElements"
            class="
                mb-2 block w-full appearance-none border p-2 text-gray-700
                transition-colors duration-150 ease-in-out placeholder:text-gray-400 focus:outline-none
              "
            type="password"
            autocomplete="off"
            @keyup.enter="doLogin(authType)"
          >
          <v-button
            :disabled="disableAuthElements"
            label="Login"
            @click="disableAuthElements ? undefined : doLogin(authType)"
          />
        </div>
      </div>
    </template>
  </div>

  <div class="flex flex-col gap-4 p-3">
    <div
      v-if="errorMessage"
      class="border-l-4 border-red-500 bg-gray-100 px-4 py-3"
    >
      {{ errorMessage }}
    </div>

    <!-- ******* BASE *******  -->
    <Base
      v-for="base in filterResources(resources, 'rdk', 'component', 'base')"
      :key="base.name"
      :name="base.name"
      :client="client"
      :resources="resources"
      :stream-manager="streamManager"
    />

    <!-- ******* GANTRY *******  -->
    <Gantry
      v-for="gantry in filterRdkComponentsWithStatus(resources, status, 'gantry')"
      :key="gantry.name"
      :name="gantry.name"
      :client="client"
      :status="(resourceStatusByName(gantry) as unknown as ReturnType<typeof fixGantryStatus>)"
    />

    <!-- ******* MovementSensor *******  -->
    <MovementSensor
      v-for="sensor in filterResources(resources, 'rdk', 'component', 'movement_sensor')"
      :key="sensor.name"
      :name="sensor.name"
      :client="client"
    />

    <!-- ******* ARM *******  -->
    <Arm
      v-for="arm in filterResources(resources, 'rdk', 'component', 'arm')"
      :key="arm.name"
      :name="arm.name"
      :client="client"
      :status="(resourceStatusByName(arm) as any)"
      :raw-status="(rawResourceStatusByName(arm) as any)"
    />

    <!-- ******* GRIPPER *******  -->
    <Gripper
      v-for="gripper in filterResources(resources, 'rdk', 'component', 'gripper')"
      :key="gripper.name"
      :name="gripper.name"
      :client="client"
    />

    <!-- ******* SERVO *******  -->
    <ServoComponent
      v-for="servo in filterRdkComponentsWithStatus(resources, status, 'servo')"
      :key="servo.name"
      :name="servo.name"
      :client="client"
      :status="(resourceStatusByName(servo) as any)"
      :raw-status="(rawResourceStatusByName(servo) as any)"
    />

    <!-- ******* MOTOR *******  -->
    <Motor
      v-for="motor in filterRdkComponentsWithStatus(resources, status, 'motor')"
      :key="motor.name"
      :name="motor.name"
      :client="client"
      :status="(resourceStatusByName(motor) as any)"
    />

    <!-- ******* INPUT VIEW *******  -->
    <InputController
      v-for="controller in filteredInputControllerList()"
      :key="controller.name"
      :name="controller.name"
      :status="(resourceStatusByName(controller) as any)"
      class="input"
    />

    <!-- ******* WEB CONTROLS *******  -->
    <Gamepad
      v-for="gamepad in filteredWebGamepads()"
      :key="gamepad.name"
      :name="gamepad.name"
      :client="client"
    />

    <!-- ******* BOARD *******  -->
    <Board
      v-for="board in filterRdkComponentsWithStatus(resources, status, 'board')"
      :key="board.name"
      :name="board.name"
      :client="client"
      :status="(resourceStatusByName(board) as any)"
    />

    <!-- ******* CAMERAS *******  -->
    <CamerasList
      parent-name="app"
      :client="client"
      :stream-manager="streamManager"
      :resources="filterResources(resources, 'rdk', 'component', 'camera')"
    />

    <!-- ******* NAVIGATION ******* -->
    <Navigation
      v-for="nav in filterResources(resources, 'rdk', 'service', 'navigation')"
      :key="nav.name"
      :resources="resources"
      :name="nav.name"
      :client="client"
    />

    <!-- ******* SENSORS ******* -->
    <Sensors
      v-if="nonEmpty(sensorNames)"
      :name="filterNonRemoteResources(resources, 'rdk', 'service', 'sensors')[0]!.name"
      :client="client"
      :sensor-names="sensorNames"
    />

    <!-- ******* AUDIO INPUTS *******  -->
    <AudioInput
      v-for="audioInput in filterResources(resources, 'rdk', 'component', 'audio_input')"
      :key="audioInput.name"
      :name="audioInput.name"
      :client="client"
    />

    <!-- ******* SLAM *******  -->
    <Slam
      v-for="slam in filterResources(resources, 'rdk', 'service', 'slam')"
      :key="slam.name"
      :name="slam.name"
      :client="client"
      :resources="resources"
    />

    <!-- ******* DO ******* -->
    <DoCommand
      v-if="nonEmpty(filterResources(resources, 'rdk', 'component', 'generic'))"
      :client="client"
      :resources="filterResources(resources, 'rdk', 'component', 'generic')"
    />

    <!-- ******* OPERATIONS AND SESSIONS ******* -->
    <OperationsSessions
      v-if="connectedOnce"
      :client="client"
      :operations="currentOps"
      :sessions="currentSessions"
      :sessions-supported="sessionsSupported"
      :connection-manager="appConnectionManager"
    />
  </div>
</template>

<style>
#source {
  position: relative;
  width: 50%;
  height: 50%;
}

h3 {
  margin: 0.1em;
  margin-block-end: 0.1em;
}
</style>
