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
import robotApi, {
  type Operation,
  type Status,
  type StreamStatusResponse,
} from './gen/robot/v1/robot_pb.esm';
import { RobotService } from './gen/robot/v1/robot_pb_service.esm';
import type { ServiceError } from './gen/proto/stream/v1/stream_pb_service.esm';
import commonApi, { type ResourceName } from './gen/common/v1/common_pb.esm';
import cameraApi from './gen/component/camera/v1/camera_pb.esm';
import sensorsApi from './gen/service/sensors/v1/sensors_pb.esm';

import {
  resourceNameToSubtypeString,
  resourceNameToString,
  filterResources,
  filterNonRemoteResources,
  filterRdkComponentsWithStatus,
  filterComponentsWithNames,
  type Resource,
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
import { addStream, removeStream } from './lib/stream';

const relevantSubtypesForStatus = [
  'arm',
  'gantry',
  'board',
  'servo',
  'motor',
  'input_controller',
];

const passwordInput = $ref<HTMLInputElement>();
const supportedAuthTypes = $computed(() => window.supportedAuthTypes);
const rawStatus = $ref<Record<string, Status>>({});
const status = $ref<Record<string, Status>>({});
const errors = $ref<Record<string, boolean>>({});

let statusStream: grpc.Request | null = null;
let statusStreamOpID: string | undefined;
let baseCameraState = new Map<string, boolean>();
let lastStatusTS: number | null = null;
let disableAuthElements = $ref(false);
let cameraFrameIntervalId = $ref(-1);
let currentOps = $ref<{ op: Operation.AsObject, elapsed: number }[]>([]);
let sensorNames = $ref<ResourceName.AsObject[]>([]);
let resources = $ref<Resource[]>([]);
let resourcesOnce = false;
let errorMessage = $ref('');
let connectedFirstTimeResolve: (value: void) => void;
const connectedFirstTime = new Promise<void>((resolve) => {
  connectedFirstTimeResolve = resolve;
});
let connectionManager = $ref<{
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
      case 'streams': {
        errorsList.innerHTML += '<li>Streams</li>';
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
  window.sensorsService.getSensors(req, new grpc.Metadata(), (err, resp) => {
    if (err) {
      return displayError(err);
    }
    sensorNames = resp!.toObject().sensorNamesList;
  });
};

const fixRawStatus = (resource: Resource, statusToFix: unknown) => {
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
      // @ts-expect-error @TODO type needs to be fixed
      const fixed = fixRawStatus(nameObj, statusJs);
      // @ts-expect-error @TODO type needs to be fixed
      const name = resourceNameToString(nameObj);
      rawStatus[name] = statusJs as unknown as Status;
      status[name] = fixed as unknown as Status;
    } catch (error) {
      // @ts-expect-error @TODO type needs to be fixed
      toast.error(`Couldn't fix status for ${resourceNameToString(nameObj)}`, error);
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

  let newResources: Resource[] = [];

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

  const robotSvcWithOpts = window.robotService as unknown as svcWithOpts;
  statusStream = grpc.invoke(RobotService.StreamStatus, {
    request: streamReq,
    host: window.robotService.serviceHost,
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
      updateStatus((response as StreamStatusResponse).getStatusList());
    },
    onEnd: (endStatus, endStatusMessage, trailers) => {
      console.error('error streaming robot status', endStatus, ' ', endStatusMessage, ' ', trailers);
      statusStream = null;
    },
  });
};

// query metadata service every 0.5s
const queryMetadata = () => {
  return new Promise((resolve, reject) => {
    let resourcesChanged = false;
    let shouldRestartStatusStream = !(resourcesOnce && statusStream);

    window.robotService.resourceNames(new robotApi.ResourceNamesRequest(), new grpc.Metadata(), (err, resp) => {
      if (err) {
        reject(err);
        return;
      }

      if (!resp) {
        reject(new Error('An unexpected issue occured.'));
        return;
      }

      const { resourcesList } = resp.toObject();

      // if resource list has changed, flag that
      const differences = new Set(resources.map((name) => resourceNameToString(name)));
      // @ts-expect-error @TODO this is incorrectly typed.
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

      // @ts-expect-error @TODO type needs to be fixed
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
  return new Promise<Operation.AsObject[]>((resolve, reject) => {
    const req = new robotApi.GetOperationsRequest();

    const now = Date.now();
    window.robotService.getOperations(req, new grpc.Metadata(), (err, resp) => {
      if (err) {
        reject(err);
        return;
      }
      connectionManager.rtt = Math.max(Date.now() - now, 0);

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

const createConnectionManager = () => {
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
        if (error instanceof ConnectionClosedError) {
          statuses.resources = false;
        }
        newErrors.push(error);
      }

      if (statuses.resources) {
        try {
          await loadCurrentOps();

          if (!statuses.ops) {
            connectionRestablished = true;
          }

          statuses.ops = true;
        } catch (error) {
          if (error instanceof ConnectionClosedError) {
            statuses.ops = false;
          }
          newErrors.push(error);
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

      handleCallErrors(statuses, newErrors);
      errorMessage = 'Connection lost, attempting to reconnect ...';

      try {
        console.log('reconnecting');

        // reset status/stream state
        if (statusStream) {
          statusStream.close();
          statusStream = null;
        }
        resourcesOnce = false;

        await window.connect();
        await fetchCurrentOps();
        lastStatusTS = Date.now();
        console.log('reconnected');
      } catch (error) {
        console.error('failed to reconnect; retrying:', error);
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
connectionManager = createConnectionManager();

const resourceStatusByName = (resource: Resource) => {
  return status[resourceNameToString(resource)];
};

const rawResourceStatusByName = (resource: Resource) => {
  return rawStatus[resourceNameToString(resource)];
};

const hasWebGamepad = () => {
  // TODO (APP-146): replace these with constants
  return resources.some((elem) =>
    elem.namespace === 'rdk' &&
    elem.type === 'component' &&
    elem.subtype === 'input_controller' &&
    elem.name === 'WebGamepad');
};

const filteredInputControllerList = () => {

  /*
   * TODO (APP-146): replace these with constants
   * filters out WebGamepad
   */
  return resources.filter((elem) =>
    elem.namespace === 'rdk' &&
    elem.type === 'component' &&
    elem.subtype === 'input_controller' &&
    elem.name !== 'WebGamepad' &&
    resourceStatusByName(elem));
};

const viewCamera = async (name: string, isOn: boolean) => {
  if (isOn) {
    try {
      // only add stream if base camera is not active
      if (!baseCameraState.get(name)) {
        await addStream(name);
      }
    } catch (error) {
      displayError(error as ServiceError);
    }
  } else {
    try {
      // only remove stream if base camera is not active
      if (!baseCameraState.get(name)) {
        await removeStream(name);
      }
    } catch (error) {
      displayError(error as ServiceError);
    }
  }
};

const viewManualFrame = (cameraName: string) => {
  const req = new cameraApi.RenderFrameRequest();
  req.setName(cameraName);
  const mimeType = 'image/jpeg';
  req.setMimeType(mimeType);
  window.cameraService.renderFrame(req, new grpc.Metadata(), (err, resp) => {
    if (err) {
      return displayError(err);
    }

    const streamContainers = document.querySelectorAll(`[data-stream="${cameraName}"]`);
    for (const streamContainer of streamContainers) {
      streamContainer.querySelector('video')?.remove();
      streamContainer.querySelector('img')?.remove();
      const image = new Image();
      const blob = new Blob([resp!.getData_asU8()], { type: mimeType });
      image.src = URL.createObjectURL(blob);
      streamContainer.append(image);
    }
  });
};

const viewIntervalFrame = (cameraName: string, time: string) => {
  cameraFrameIntervalId = window.setInterval(() => {
    const req = new cameraApi.RenderFrameRequest();
    req.setName(cameraName);
    req.setMimeType('image/jpeg');
    window.cameraService.renderFrame(req, new grpc.Metadata(), (err, resp) => {
      if (err) {
        return displayError(err);
      }

      const streamContainers = document.querySelectorAll(`[data-stream="${cameraName}"]`);
      for (const streamContainer of streamContainers) {
        streamContainer.querySelector('video')?.remove();
        streamContainer.querySelector('img')?.remove();
        const image = new Image();
        const blob = new Blob([resp!.getData_asU8()], { type: 'image/jpeg' });
        image.src = URL.createObjectURL(blob);
        streamContainer.append(image);
      }
    });
  }, Number(time) * 1000);
};

const viewCameraFrame = (cameraName: string, time: string) => {
  window.clearInterval(cameraFrameIntervalId);
  if (time === 'manual') {
    viewCamera(cameraName, false);
    viewManualFrame(cameraName);
  } else if (time === 'live') {
    viewCamera(cameraName, true);
  } else {
    viewCamera(cameraName, false);
    viewIntervalFrame(cameraName, time);
  }
};

const nonEmpty = (object: object) => {
  return Object.keys(object).length > 0;
};

const doConnect = async (authEntity: string, creds: Credentials, onError?: () => void) => {
  console.debug('connecting');
  document.querySelector('#connecting')!.classList.remove('hidden');

  try {
    await window.connect(authEntity, creds);
  } catch (error) {
    console.error('failed to connect:', error);
    if (onError) {
      toast.error('failed to connect; retrying');
      setTimeout(onError, 1000);
    } else {
      toast.error('failed to connect');
    }
    return;
  }

  console.debug('connected');
  document.querySelector('#pre-app')!.classList.add('hidden');
  disableAuthElements = false;
  connectedFirstTimeResolve();
};

const doLogin = (authType: string) => {
  disableAuthElements = true;
  const creds = { type: authType, payload: passwordInput.value };
  doConnect('', creds);
};

const initConnect = () => {
  if (window.supportedAuthTypes.length === 0) {
    doConnect(window.bakedAuth.authEntity, window.bakedAuth.creds, initConnect);
  }
};

const updatedBaseCameraState = (event: Map<string, boolean>) => {
  baseCameraState = event;
};

onMounted(async () => {
  initConnect();
  await connectedFirstTime;

  connectionManager.start();

  addResizeListeners();
});

</script>

<template>
  <div id="pre-app">
    <div
      id="connecting-error"
      class="border-danger-500 hidden border-l-4 bg-gray-100 px-4 py-3"
      role="alert"
    />

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
      <span>{{ authType }}: </span>
      <div class="w-96">
        <input
          ref="passwordInput"
          :disabled="disableAuthElements"
          class="
            mb-2 block w-full appearance-none border p-2 text-gray-700
            transition-colors duration-150 ease-in-out placeholder:text-gray-400 focus:outline-none
          "
          type="password"
          @keyup.enter="doLogin(authType)"
        >
        <v-button
          :disabled="disableAuthElements"
          label="Login"
          @click="disableAuthElements ? undefined : doLogin(authType)"
        />
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
      :resources="resources"
      @base-camera-state="updatedBaseCameraState($event)"
    />

    <!-- ******* GANTRY *******  -->
    <Gantry
      v-for="gantry in filterRdkComponentsWithStatus(resources, status, 'gantry')"
      :key="gantry.name"
      :name="gantry.name"
      :status="(resourceStatusByName(gantry) as unknown as ReturnType<typeof fixGantryStatus>)"
    />

    <!-- ******* MovementSensor *******  -->
    <MovementSensor
      v-for="sensor in filterResources(resources, 'rdk', 'component', 'movement_sensor')"
      :key="sensor.name"
      :name="sensor.name"
    />

    <!-- ******* ARM *******  -->
    <Arm
      v-for="arm in filterResources(resources, 'rdk', 'component', 'arm')"
      :key="arm.name"
      :name="arm.name"
      :status="(resourceStatusByName(arm) as any)"
      :raw-status="(rawResourceStatusByName(arm) as any)"
    />

    <!-- ******* GRIPPER *******  -->
    <Gripper
      v-for="gripper in filterResources(resources, 'rdk', 'component', 'gripper')"
      :key="gripper.name"
      :name="gripper.name"
    />

    <!-- ******* SERVO *******  -->
    <ServoComponent
      v-for="servo in filterRdkComponentsWithStatus(resources, status, 'servo')"
      :key="servo.name"
      :name="servo.name"
      :status="(resourceStatusByName(servo) as any)"
      :raw-status="(rawResourceStatusByName(servo) as any)"
    />

    <!-- ******* MOTOR *******  -->
    <Motor
      v-for="motor in filterRdkComponentsWithStatus(resources, status, 'motor')"
      :key="motor.name"
      :name="motor.name"
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
      v-if="hasWebGamepad()"
    />

    <!-- ******* BOARD *******  -->
    <Board
      v-for="board in filterRdkComponentsWithStatus(resources, status, 'board')"
      :key="board.name"
      :name="board.name"
      :status="(resourceStatusByName(board) as any)"
    />

    <!-- ******* CAMERAS *******  -->
    <Camera
      v-for="camera in filterResources(resources, 'rdk', 'component', 'camera')"
      :key="camera.name"
      :camera-name="camera.name"
      :resources="resources"
      @toggle-camera="isOn => { viewCamera(camera.name, isOn) }"
      @refresh-camera="t => { viewCameraFrame(camera.name, t) }"
      @selected-camera-view="t => { viewCameraFrame(camera.name, t) }"
    />

    <!-- ******* NAVIGATION ******* -->
    <Navigation
      v-for="nav in filterResources(resources, 'rdk', 'service', 'navigation')"
      :key="nav.name"
      :resources="nav.resources"
      :name="nav.name"
    />

    <!-- ******* SENSORS ******* -->
    <Sensors
      v-if="nonEmpty(sensorNames)"
      :name="filterNonRemoteResources(resources, 'rdk', 'service', 'sensors')[0]!.name"
      :sensor-names="sensorNames"
    />

    <!-- ******* AUDIO INPUTS *******  -->
    <AudioInput
      v-for="audioInput in filterResources(resources, 'rdk', 'component', 'audio_input')"
      :key="audioInput.name"
      :name="audioInput.name"
    />

    <!-- ******* SLAM *******  -->
    <Slam
      v-for="slam in filterResources(resources, 'rdk', 'service', 'slam')"
      :key="slam.name"
      :name="slam.name"
      :resources="resources"
    />

    <!-- ******* DO ******* -->
    <DoCommand :resources="filterComponentsWithNames(resources)" />

    <!-- ******* CURRENT OPERATIONS ******* -->
    <CurrentOperations
      :operations="currentOps"
      :connection-manager="connectionManager"
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
