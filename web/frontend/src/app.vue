<!-- eslint-disable require-atomic-updates -->
<script setup lang="ts">

import { onMounted } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { Duration } from 'google-protobuf/google/protobuf/duration_pb';
import type { Credentials } from '@viamrobotics/rpc';
import { ConnectionClosedError } from '@viamrobotics/rpc';
import { toast } from './lib/toast';
import { displayError } from './lib/error';
import { addResizeListeners } from './lib/resize';
import {
  Client,
  RobotService,
  cameraApi,
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
} from './lib/resource';

import Arm from './components/arm.vue';
import AudioInput from './components/audio-input.vue';
import Base from './components/base.vue';
import Board from './components/board.vue';
import Camera from './components/camera.vue';
import CurrentOperations from './components/current-operations.vue';
import DoCommand from './components/do-command.vue';
import Gantry from './components/gantry.vue';
import Gripper from './components/gripper.vue';
import Gamepad from './components/gamepad.vue';
import InputController from './components/input-controller.vue';
import Motor from './components/motor-detail.vue';
import MovementSensor from './components/movement-sensor.vue';
import Navigation from './components/navigation.vue';
import ServoComponent from './components/servo.vue';
import Sensors from './components/sensors.vue';
import Slam from './components/slam.vue';

import {
  fixArmStatus,
  fixBoardStatus,
  fixGantryStatus,
  fixInputStatus,
  fixMotorStatus,
  fixServoStatus,
} from './lib/fixers';

const relevantSubtypesForStatus = [
  'arm',
  'gantry',
  'board',
  'servo',
  'motor',
  'input_controller',
];

const password = $ref<string>('');
const supportedAuthTypes = $computed(() => window.supportedAuthTypes);
const rawStatus = $ref<Record<string, robotApi.Status>>({});
const status = $ref<Record<string, robotApi.Status>>({});
const errors = $ref<Record<string, boolean>>({});

let statusStream: grpc.Request | null = null;
let statusStreamOpID: string | undefined;
let lastStatusTS: number | null = null;
let disableAuthElements = $ref(false);
let cameraFrameIntervalId = $ref(-1);
let currentOps = $ref<{ op: robotApi.Operation.AsObject, elapsed: number }[]>([]);
let sensorNames = $ref<commonApi.ResourceName.AsObject[]>([]);
let resources = $ref<commonApi.ResourceName.AsObject[]>([]);
let resourcesOnce = false;
let errorMessage = $ref('');
let connectedOnce = $ref(false);
let connectedFirstTimeResolve: (value: void) => void;
const connectedFirstTime = new Promise<void>((resolve) => {
  connectedFirstTimeResolve = resolve;
});

const selectedMap = {
  Live: 'live',
  'Manual Refresh': 0,
  'Every 30 Seconds': 30,
  'Every 10 Seconds': 10,
  'Every Second': 1,
} as const;

const rtcConfig = {
  iceServers: [
    {
      urls: 'stun:global.stun.twilio.com:3478?transport=udp',
    },
  ],
};

if (window.webrtcAdditionalICEServers) {
  rtcConfig.iceServers = [...rtcConfig.iceServers, ...window.webrtcAdditionalICEServers];
}
const impliedURL = `${location.protocol}//${location.hostname}${location.port ? `:${location.port}` : ''}`;

const client = new Client(impliedURL, {
  enabled: window.webrtcEnabled,
  host: window.host,
  signalingAddress: window.webrtcSignalingAddress,
  rtcConfig,
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
const sessionOps = new Set<string>();

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
  client.sensorsService.getSensors(req, new grpc.Metadata(), (err, resp) => {
    if (err) {
      return displayError(err);
    }
    sensorNames = resp!.toObject().sensorNamesList;
  });
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
    } catch (error) {
      toast.error(
        `Couldn't fix status for ${resourceNameToString(nameObj)}`,
        error
      );
    }
  }
};

const restartStatusStream = () => {
  if (statusStream) {
    statusStream.close();
    statusStream = null;
    if (statusStreamOpID) {
      sessionOps.delete(statusStreamOpID);
      statusStreamOpID = undefined;
    }
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

  interface svcWithOpts {
    options: {
      transport: grpc.TransportFactory;
      debug: boolean;
    };
  }

  const robotSvcWithOpts = client.robotService as unknown as svcWithOpts;
  statusStream = grpc.invoke(RobotService.StreamStatus, {
    request: streamReq,
    host: client.robotService.serviceHost,
    transport: robotSvcWithOpts.options.transport,
    debug: robotSvcWithOpts.options.debug,
    onHeaders: (headers: grpc.Metadata) => {
      if (headers.headersMap.opid && headers.headersMap.opid.length > 0) {
        const [opID] = headers.headersMap.opid;
        sessionOps.add(opID!);
        statusStreamOpID = opID;
      }
    },
    onMessage: (response) => {
      lastStatusTS = Date.now();
      updateStatus((response as robotApi.StreamStatusResponse).getStatusList());
    },
    onEnd: (endStatus, endStatusMessage, trailers) => {
      console.error(
        'error streaming robot status',
        endStatus,
        ' ',
        endStatusMessage,
        ' ',
        trailers
      );
      statusStream = null;
    },
  });
};

// query metadata service every 0.5s
const queryMetadata = () => {
  return new Promise((resolve, reject) => {
    let resourcesChanged = false;
    let shouldRestartStatusStream = !(resourcesOnce && statusStream);

    client.robotService.resourceNames(new robotApi.ResourceNamesRequest(), new grpc.Metadata(), (err, resp) => {
      if (err) {
        reject(err);
        return;
      }

      if (!resp) {
        reject(new Error('An unexpected issue occured.'));
        return;
      }

      const { resourcesList } = resp.toObject();

      const differences = new Set(resources.map((name) => resourceNameToString(name)));
      const resourceSet = new Set(resourcesList.map((name) => resourceNameToString(name)));

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
    });
  });
};

const fetchCurrentOps = () => {
  return new Promise<robotApi.Operation.AsObject[]>((resolve, reject) => {
    const req = new robotApi.GetOperationsRequest();

    const now = Date.now();
    client.robotService.getOperations(req, new grpc.Metadata(), (err, resp) => {
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
    });
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

const isWebRtcEnabled = () => {
  return window.webrtcEnabled;
};

const createAppConnectionManager = () => {
  const checkIntervalMillis = 10_000;
  const statuses = {
    resources: false,
    ops: false,
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
          statusStream.close();
          statusStream = null;
        }
        resourcesOnce = false;

        await client.connect();
        await fetchCurrentOps();
        lastStatusTS = Date.now();
        console.log('reconnected');
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

const viewFrame = (cameraName: string) => {
  const req = new cameraApi.RenderFrameRequest();
  req.setName(cameraName);
  req.setMimeType('image/jpeg');
  client.cameraService.renderFrame(req, new grpc.Metadata(), (err, resp) => {
    if (err) {
      return displayError(err);
    }

    const streamContainers = document.querySelectorAll(
      `[data-stream="${cameraName}"]`
    );
    for (const streamContainer of streamContainers) {
      streamContainer.querySelector('video')?.remove();
      streamContainer.querySelector('img')?.remove();
      const image = new Image();
      const blob = new Blob([resp!.getData_asU8()], { type: 'image/jpeg' });
      image.src = URL.createObjectURL(blob);
      streamContainer.append(image);
    }
  });
};

const clearFrameInterval = () => {
  window.clearInterval(cameraFrameIntervalId);
};

const viewCameraFrame = (cameraName: string, time: string) => {
  clearFrameInterval();
  const selectedInterval = selectedMap[time as keyof typeof selectedMap];

  if (time === 'Live') {
    return;
  }

  if (time === 'Manual Refresh') {
    viewFrame(cameraName);
  } else {
    cameraFrameIntervalId = window.setInterval(() => {
      viewFrame(cameraName);
    }, Number(selectedInterval) * 1000);
  }
};

const nonEmpty = (object: object) => {
  return Object.keys(object).length > 0;
};

const doConnect = async (authEntity: string, creds: Credentials, onError?: (reason?: unknown) => void) => {
  console.debug('connecting');
  document.querySelector('#connecting')!.classList.remove('hidden');

  try {
    await client.connect(authEntity, creds);
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
  doConnect(window.host, creds, (error) => {
    document.querySelector('#connecting')!.classList.add('hidden');
    disableAuthElements = false;
    console.error(error);
    toast.error(`failed to connect: ${error}`);
  });
};

const initConnect = () => {
  if (window.supportedAuthTypes.length === 0) {
    doConnect(window.bakedAuth.authEntity, window.bakedAuth.creds, () => {
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
    <Camera
      v-for="camera in filterResources(resources, 'rdk', 'component', 'camera')"
      :key="camera.name"
      :camera-name="camera.name"
      :client="client"
      :resources="resources"
      @selected-camera-view="t => { viewCameraFrame(camera.name, t) }"
      @clear-interval="clearFrameInterval"
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
      v-if="connectedOnce"
      :client="client"
      :resources="filterComponentsWithNames(resources)"
    />

    <!-- ******* OPERATIONS ******* -->
    <CurrentOperations
      v-if="connectedOnce"
      :client="client"
      :operations="currentOps"
      :connection-manager="appConnectionManager"
      :session-ops="sessionOps"
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
