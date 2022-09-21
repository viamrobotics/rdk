<!-- eslint-disable require-atomic-updates -->
<script>

import { grpc } from '@improbable-eng/grpc-web';
import { toast } from './lib/toast';
import robotApi from './gen/proto/api/robot/v1/robot_pb.esm';
import commonApi from './gen/proto/api/common/v1/common_pb.esm';
import cameraApi from './gen/proto/api/component/camera/v1/camera_pb.esm';
import movementsensorApi from './gen/proto/api/component/movementsensor/v1/movementsensor_pb.esm';
import sensorsApi from './gen/proto/api/service/sensors/v1/sensors_pb.esm';
import servoApi from './gen/proto/api/component/servo/v1/servo_pb.esm';
import streamApi from './gen/proto/stream/v1/stream_pb.esm';

import {
  normalizeRemoteName,
  resourceNameToSubtypeString,
  resourceNameToString,
  filterResources,
  filterRdkComponentsWithStatus,
  filterResourcesWithNames,
} from './lib/resource';

import {
  MotorControlHelper,
  ServoControlHelper,
} from './rc/control_helpers';

import { addResizeListeners } from './lib/resize';
import Arm from './components/arm.vue';
import AudioInput from './components/audio-input.vue';
import BaseComponent from './components/base.vue';
import Board from './components/board.vue';
import Camera from './components/camera.vue';
import CurrentOperations from './components/current-operations.vue';
import DoCommand from './components/do-command.vue';
import Gantry from './components/gantry.vue';
import Gripper from './components/gripper.vue';
import Gamepad from './components/gamepad.vue';
import InputController from './components/input-controller.vue';
import MotorDetail from './components/motor-detail.vue';
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

export default {
  components: {
    Arm,
    AudioInput,
    BaseComponent,
    Board,
    Camera,
    CurrentOperations,
    DoCommand,
    Gantry,
    Gamepad,
    Gripper,
    InputController,
    MotorDetail,
    MovementSensor,
    Navigation,
    ServoComponent,
    Sensors,
    Slam,
  },
  data() {
    return {
      supportedAuthTypes: window.supportedAuthTypes,
      error: '',
      res: {},
      rawStatus: {},
      status: {},
      resources: [],
      sensorNames: [],
      cameraFrameIntervalId: null,
      objects: null,
      value: 0,
      movementsensorData: {},
      currentOps: [],
      connectionManager: null,
      errors: {},
    };
  },
  async mounted() {
    this.grpcCallback = this.grpcCallback.bind(this);
    await this.waitForClientAndStart();

    this.movementsensorRefresh();

    this.connectionManager = this.createConnectionManager();
    this.connectionManager.start();

    addResizeListeners();
  },
  methods: {
    filterResources,
    filterRdkComponentsWithStatus,
    resourceNameToString,
    filterResourcesWithNames,

    handleError(message, error, onceKey) {
      if (onceKey) {
        if (this.errors[onceKey]) {
          return;
        }

        this.errors[onceKey] = true;
      }

      toast.error(message);
      console.error(message, { error });
    },

    createConnectionManager() {
      const statuses = {
        resources: true,
        ops: true,
      };

      let interval = -1;
      let connectionRestablished = false;

      const handleCallErrors = (errors) => {
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

        this.handleError(
          `Error fetching the following: ${errorsList.outerHTML}`,
          errors,
          'connection'
        );
      };

      const makeCalls = async () => {
        const errors = [];

        try {
          await this.queryMetadata();

          if (!statuses.resources) {
            connectionRestablished = true;
          }

          statuses.resources = true;
        } catch (error) {
          statuses.resources = false;
          errors.push[error];
        }

        try {
          await this.loadCurrentOps();

          if (!statuses.ops) {
            connectionRestablished = true;
          }

          statuses.ops = true;
        } catch (error) {
          statuses.ops = false;
          errors.push[error];
        }

        if (isConnected()) {
          if (connectionRestablished) {
            toast.success('Connection established');
            connectionRestablished = false;
            this.errors.connection = false;
          }
          
          this.error = null;
          return;
        }

        handleCallErrors(errors);
        this.error = 'Connection error, attempting to reconnect ...';
      };

      const isConnected = () => {
        return (
          statuses.resources && 
          statuses.ops
        );
      };

      const stop = () => {
        window.clearInterval(interval);
      };

      const start = () => {
        stop();
        interval = window.setInterval(makeCalls, 500);
      };

      return {
        statuses,
        interval,
        stop,
        start,
        isConnected,
      };
    },
    
    fixRawStatus(name, status) {
      switch (resourceNameToSubtypeString(name)) {
      // TODO (APP-146): generate these using constants
        case 'rdk:component:arm':
          return fixArmStatus(status);
        case 'rdk:component:board':
          return fixBoardStatus(status);
        case 'rdk:component:gantry':
          return fixGantryStatus(status);
        case 'rdk:component:input_controller':
          return fixInputStatus(status);
        case 'rdk:component:motor':
          return fixMotorStatus(status);
        case 'rdk:component:servo':
          return fixServoStatus(status);
      }

      return status;
    },
    grpcCallback (err, resp, stringify) {
      if (err) {
        this.error = err;
        return;
      }
      if (stringify === undefined || stringify) {
        try {
          this.res = resp.toJavaScript ? JSON.stringify(resp.toJavaScript()) : JSON.stringify(resp.toObject());
        } catch {
          this.error = err;
        }
      }
    },
    stringToResourceName(nameStr) {
      const nameParts = nameStr.split('/');
      let name = '';

      if (nameParts.length === 2) {
        name = nameParts[1];
      }

      const subtypeParts = nameParts[0].split(':');
      if (subtypeParts.length > 3) {
        throw 'more than 2 colons in resource name string';
      }
      if (subtypeParts.length < 3) {
        throw 'less than 2 colons in resource name string';
      }
      return { namespace: subtypeParts[0], type: subtypeParts[1], subtype: subtypeParts[2], name };
    },
    resourceStatusByName(name) {
      return this.status[resourceNameToString(name)];
    },
    rawResourceStatusByName(name) {
      return this.rawStatus[resourceNameToString(name)];
    },
    servoMove(name, amount) {
      const servo = this.rawResourceStatusByName(name);
      const oldAngle = servo.position_deg || 0;
      const angle = oldAngle + amount;
      const req = new servoApi.MoveRequest();
      req.setName(name.name);
      req.setAngleDeg(angle);
      window.servoService.move(req, {}, this.grpcCallback);
    },
    servoStop(name) {
      ServoControlHelper.stop(name, (err, resp) => this.grpcCallback(err, resp));
    },
    motorCommand(name, inputs) {
      switch (inputs.type) {
        case 'go':
          MotorControlHelper.setPower(name, inputs.power * inputs.direction / 100, this.grpcCallback);
          break;
        case 'goFor':
          MotorControlHelper.goFor(name, inputs.rpm * inputs.direction, inputs.revolutions, this.grpcCallback);
          break;
        case 'goTo':
          MotorControlHelper.goTo(name, inputs.rpm, inputs.position, this.grpcCallback);
          break;
      }
    },
    motorStop(name) {
      MotorControlHelper.stop(name, this.grpcCallback);
    },
    hasWebGamepad() {
      // TODO (APP-146): replace these with constants
      return this.resources.some((elem) =>
        elem.namespace === 'rdk' &&
        elem.type === 'component' &&
        elem.subtype === 'input_controller' &&
        elem.name === 'WebGamepad'
      );
    },
    filteredInputControllerList() {
      // TODO (APP-146): replace these with constants
      // filters out WebGamepad
      return this.resources.filter((elem) =>
        elem.namespace === 'rdk' &&
        elem.type === 'component' &&
        elem.subtype === 'input_controller' &&
        elem.name !== 'WebGamepad' &&
        this.resourceStatusByName(elem)
      );
    },
    inputInject(req) {
      window.inputControllerService.triggerEvent(req, new grpc.Metadata(), this.grpcCallback);
    },
    renderFrame(cameraName) {
      const req = new cameraApi.RenderFrameRequest();
      req.setName(cameraName);
      const mimeType = 'image/jpeg';
      req.setMimeType(mimeType);
      window.cameraService.renderFrame(req, new grpc.Metadata(), (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        const blob = new Blob([resp.getData_asU8()], { type: mimeType });
        window.open(URL.createObjectURL(blob), '_blank');
      });
    },
    viewCameraFrame(cameraName, time) {
      clearInterval(this.cameraFrameIntervalId);
      if (time === 'manual') {
        this.viewCamera(cameraName, false);
        this.viewManualFrame(cameraName);
      } else if (time === 'live') {
        this.viewCamera(cameraName, true);
      } else {
        this.viewCamera(cameraName, false);
        this.viewIntervalFrame(cameraName, time);
      }
    },
    viewManualFrame(cameraName) {
      const req = new cameraApi.RenderFrameRequest();
      req.setName(cameraName);
      const mimeType = 'image/jpeg';
      req.setMimeType(mimeType);
      window.cameraService.renderFrame(req, new grpc.Metadata(), (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        const streamName = normalizeRemoteName(cameraName);
        const streamContainer = document.querySelector(`#stream-${streamName}`);
        if (streamContainer && streamContainer.querySelectorAll('video').length > 0) {
          streamContainer.querySelectorAll('video')[0].remove();
        }
        if (streamContainer && streamContainer.querySelectorAll('img').length > 0) {
          streamContainer.querySelectorAll('img')[0].remove();
        }
        const image = new Image();
        const blob = new Blob([resp.getData_asU8()], { type: mimeType });
        image.src = URL.createObjectURL(blob);
        streamContainer.append(image);
      });
    },
    viewIntervalFrame(cameraName, time) {
      this.cameraFrameIntervalId = window.setInterval(() => {
        const req = new cameraApi.RenderFrameRequest();
        req.setName(cameraName);
        req.setMimeType('image/jpeg');
        window.cameraService.renderFrame(req, new grpc.Metadata(), (err, resp) => {
          this.grpcCallback(err, resp, false);
          if (err) {
            return;
          }
          const streamName = normalizeRemoteName(cameraName);
          const streamContainer = document.querySelector(`#stream-${streamName}`);
          if (streamContainer && streamContainer.querySelectorAll('video').length > 0) {
            streamContainer.querySelectorAll('video')[0].remove();
          }
          if (streamContainer && streamContainer.querySelectorAll('img').length > 0) {
            streamContainer.querySelectorAll('img')[0].remove();
          }
          const image = new Image();
          const blob = new Blob([resp.getData_asU8()], { type: 'image/jpeg' });
          image.src = URL.createObjectURL(blob);
          streamContainer.append(image);
        });
      }, Number(time) * 1000);
    },
    processFunctionResults(err, resp) {
      const el = document.querySelector('#function_results');

      this.grpcCallback(err, resp, false);
      if (err) {
        if (el) {
          el.value = `${err}`;
        }
        return;
      }
      const results = resp.getResultsList();

      let resultStr = '';
      if (results.length > 0) {
        resultStr += 'Results: \n';
        for (let i = 0; i < results.length && i < results.length; i++) {
          const result = results[i];
          resultStr += `${i}: ${JSON.stringify(result.toJavaScript())}\n`;
        }
      }
      resultStr += `StdOut: ${resp.getStdOut()}\n`;
      resultStr += `StdErr: ${resp.getStdErr()}\n`;
      
      if (el) {
        el.value = resultStr;
      }
    },
    nonEmpty(object) {
      return Object.keys(object).length > 0;
    },
    hasKey(d, key) {
      if (!d) {
        return false;
      }
      if (Array.isArray(d)) {
        for (const element of d) {
          if (element === key || (element.length > 0 && element.length > 0 && element[0] === key)) {
            return true;
          }
        }
        return false;
      }
      return d.hasOwn(key);
    },

    viewCamera(name, isOn) {
      const streamName = normalizeRemoteName(name);
      const streamContainer = document.querySelector(`#stream-${streamName}`);

      if (isOn) {
        const req = new streamApi.AddStreamRequest();
        req.setName(name);
        window.streamService.addStream(req, new grpc.Metadata(), (err, resp) => {
          this.grpcCallback(err, resp, false);
          if (streamContainer && streamContainer.querySelectorAll('img').length > 0) {
            streamContainer.querySelectorAll('img')[0].remove();
          }
          if (err) {
            this.error = 'no live camera device found';
            
          }
        });
        document.querySelector(`#stream-preview-${name}`)?.removeAttribute('hidden');
        return;
      }

      document.querySelector(`#stream-preview-${name}`)?.setAttribute('hidden', true);

      const req = new streamApi.RemoveStreamRequest();
      req.setName(name);
      window.streamService.removeStream(req, new grpc.Metadata(), (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (streamContainer && streamContainer.querySelectorAll('img').length > 0) {
          streamContainer.querySelectorAll('img')[0].remove();
        }
        if (err) {
          this.error = 'no live camera device found';
        }
      });
    },
    listenAudioInput(name, isOn) {
      if (isOn) {
        const req = new streamApi.AddStreamRequest();
        req.setName(name);
        window.streamService.addStream(req, new grpc.Metadata(), (err, resp) => {
          this.grpcCallback(err, resp, false);
          if (err) {
            this.error = 'no live audio input device found';
          }
        });
        return;
      }

      const req = new streamApi.RemoveStreamRequest();
      req.setName(name);
      window.streamService.removeStream(req, new grpc.Metadata(), (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          this.error = 'no live audio input device found';
        }
      });
    },
    displayRadiansInDegrees (r) {
      let d = r * 180;
      while (d < 0) {
        d += 360;
      }
      while (d > 360) {
        d -= 360;
      }
      return d.toFixed(1);
    },
    isWebRtcEnabled() {
      return window.webrtcEnabled;
    },
    // query metadata service every 0.5s
    queryMetadata() {
      return new Promise((resolve, reject) => {
        let resourcesChanged = false;
        let shouldRestartStatusStream = false;

        window.robotService.resourceNames(new robotApi.ResourceNamesRequest(), new grpc.Metadata(), (err, resp) => {
          if (err) {
            reject(err);
            return;
          }

          if (!resp) {
            reject(null);
            return;
          }

          const resources = resp.toObject().resourcesList;

          // if resource list has changed, flag that
          const differences = new Set(this.resources.map((name) => resourceNameToString(name)));
          const resourceSet = new Set(resources.map((name) => resourceNameToString(name)));

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
              const name = this.stringToResourceName(elem);
              if (name.namespace === 'rdk' && name.type === 'component' && relevantSubtypesForStatus.includes(name.subtype)) {
                shouldRestartStatusStream = true;
                break;
              }
            }
          }
          
          this.resources = resources;
          if (resourcesChanged === true) {
            this.querySensors();
            if (shouldRestartStatusStream === true) {
              this.restartStatusStream();
            }
          }
          resolve(this.resources);
        });
      });
    },
    querySensors() {
      const req = new sensorsApi.GetSensorsRequest();
      req.setName('builtin');
      window.sensorsService.getSensors(req, new grpc.Metadata(), (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        this.sensorNames = resp.toObject().sensorNamesList;
      });
    },
    loadCurrentOps () {
      return new Promise((resolve, reject) => {  
        const req = new robotApi.GetOperationsRequest();

        window.robotService.getOperations(req, new grpc.Metadata(), (err, resp) => {
          if (err) {
            reject(err);
            return;
          }

          if (!resp) {
            reject(null);
            return;
          }

          const lst = resp.toObject().operationsList;
          this.currentOps = lst;

          const now = Date.now();
          for (const op of this.currentOps) {
            op.elapsed = now - (op.started.seconds * 1000);
          }

          resolve(this.currentOps);
        });
      });
    },
    async doConnect(authEntity, creds, onError) {
      console.debug('connecting');
      document.querySelector('#connecting').classList.remove('hidden');
      try {
        await window.connect(authEntity, creds);
      } catch (error) {
        toast.error(`failed to connect: ${error}`);
        if (onError) {
          setTimeout(onError, 1000);
        }
        return;
      }
      console.debug('connected');
      document.querySelector('#pre-app').classList.add('hidden');
    },
    async waitForClientAndStart() {
      if (window.supportedAuthTypes.length === 0) {
        await this.doConnect(window.bakedAuth.authEntity, window.bakedAuth.creds, this.waitForClientAndStart);
        return;
      }

      const authElems = [];
      const disableAll = () => {
        for (const elem of authElems) {
          elem.disabled = true;
        }
      };
      const enableAll = () => {
        for (const elem of authElems) {
          elem.disabled = false;
        }
      };
      for (const authType of window.supportedAuthTypes) {
        const authDiv = document.querySelector(`#auth-${authType}`);
        const input = authDiv.querySelectorAll('input')[0];
        const button = authDiv.querySelectorAll('button')[0];
        authElems.push(input, button);
        const doLogin = () => {
          disableAll();
          const creds = { type: authType, payload: input.value };
          this.doConnect('', creds, '', '', () => enableAll());
        };
        button.addEventListener('click', () => doLogin());
        input.addEventListener('keyup', (event) => {
          if (event.key.toLowerCase() !== 'enter') {
            return;
          }
          doLogin();
        });
      }
    },
    movementsensorRefresh() {
      for (const x of filterResources(this.resources, 'rdk', 'component', 'movement_sensor')) {
        const name = x.name;

        if (!this.movementsensorData[name]) {
          this.movementsensorData[name] = {};
        }

        {
          const req = new movementsensorApi.GetOrientationRequest();
          req.setName(name);

          window.movementsensorService.getOrientation(req, new grpc.Metadata(), (err, resp) => {
            if (err) {
              console.log(err);
              return;
            }
            this.movementsensorData[name].orientation = resp.toObject().orientation;
          });
        }

        {
          const req = new movementsensorApi.GetAngularVelocityRequest();
          req.setName(name);

          window.movementsensorService.getAngularVelocity(req, new grpc.Metadata(), (err, resp) => {
            if (err) {
              console.log(err);
              return;
            }
            this.movementsensorData[name].angularVelocity = resp.toObject().angularVelocity;
          });
        }

        {
          const req = new movementsensorApi.GetLinearVelocityRequest();
          req.setName(name);

          window.movementsensorService.getLinearVelocity(req, new grpc.Metadata(), (err, resp) => {
            if (err) {
              console.log(err);
              return;
            }
            this.movementsensorData[name].linearVelocity = resp.toObject().linearVelocity;
          });
        }

        {
          const req = new movementsensorApi.GetCompassHeadingRequest();
          req.setName(name);

          window.movementsensorService.getCompassHeading(req, new grpc.Metadata(), (err, resp) => {
            if (err) {
              console.log(err);
              return;
            }
            this.movementsensorData[name].compassHeading = resp.toObject().value;
          });
        }

        {
          const req = new movementsensorApi.GetPositionRequest();
          req.setName(name);

          window.movementsensorService.getPosition(req, new grpc.Metadata(), (err, resp) => {
            if (err) {
              console.log(err);
              return;
            }
            const temp = resp.toObject();
            this.movementsensorData[name].coordinate = temp.coordinate;
            this.movementsensorData[name].altitudeMm = temp.altitudeMm;
          });
        }

        {
          const req = new movementsensorApi.GetPropertiesRequest();
          req.setName(name);

          window.movementsensorService.getProperties(req, new grpc.Metadata(), (err, resp) => {
            if (err) {
              console.log(err);
              return;
            }
            const temp = resp.toObject();
            this.movementsensorData[name].properties = temp;
          });
        }

      }

      setTimeout(this.movementsensorRefresh, 500);
    },
    updateStatus(grpcStatuses) {
      const rawStatus = {};
      const status = {};

      for (const grpcStatus of grpcStatuses) {
        const nameObj = grpcStatus.getName().toObject();
        const statusJs = grpcStatus.getStatus().toJavaScript();

        try {
          const fixed = this.fixRawStatus(nameObj, statusJs);
          const nameStr = resourceNameToString(nameObj);
          rawStatus[nameStr] = statusJs;
          status[nameStr] = fixed;
        } catch (error) {
          toast.error(`Couldn't fix status for ${resourceNameToString(nameObj)}`, error);
        }
      }

      this.rawStatus = rawStatus;
      this.status = status;
    },
    async restartStatusStream () {
      if (statusStream) {
        statusStream.cancel();
        try {
          console.log('reconnecting');
          await window.connect();
        } catch (error) {
          console.error('failed to reconnect; retrying:', error);
          setTimeout(() => this.restartStatusStream(), 1000);
        }
      }
      let resourceNames = [];
      // get all relevant resource names
      for (const subtype of relevantSubtypesForStatus) {
        resourceNames = [...resourceNames, ...filterResources(this.resources, 'rdk', 'component', subtype)];
      }

      const names = resourceNames.map((name) => {
        const resourceName = new commonApi.ResourceName();
        resourceName.setNamespace(name.namespace);
        resourceName.setType(name.type);
        resourceName.setSubtype(name.subtype);
        resourceName.setName(name.name);
        return resourceName;
      });
      const streamReq = new robotApi.StreamStatusRequest();
      streamReq.setResourceNamesList(names);
      streamReq.setEvery(new proto.google.protobuf.Duration().setNanos(500_000_000)); // 500ms
      statusStream = window.robotService.streamStatus(streamReq);
      let firstData = true;
      statusStream.on('data', (response) => {
        lastStatusTS = Date.now();
        this.updateStatus(response.getStatusList());
        if (firstData) {
          firstData = false;
          this.checkLastStatus();
        }
      });
      statusStream.on('status', (status) => {
        console.log('error streaming robot status');
        console.log(status);
        console.log(status.code, ' ', status.details);
      });
      statusStream.on('end', () => {
        console.log('done streaming robot status');
        setTimeout(() => this.restartStatusStream(), 1000);
      });
    },
    checkLastStatus () {
      const checkIntervalMillis = 3000;
      if (Date.now() - lastStatusTS > checkIntervalMillis) {
        this.restartStatusStream();
        return;
      }
      setTimeout(this.checkLastStatus, checkIntervalMillis);
    },
    handleSelectCamera (event, cameras) {
      for (const camera of cameras) {
        this.viewCamera(camera.name, event.includes(camera.name));
      }
    },
  },
};

const relevantSubtypesForStatus = ['arm', 'gantry', 'board', 'servo', 'motor', 'input_controller'];

let statusStream;
let lastStatusTS = Date.now();

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
      <div
        :id="`auth-${authType}`"
        class="w-96"
      >
        <input
          class="mb-2 block w-full appearance-none border p-2 text-gray-700 transition-colors duration-150 ease-in-out placeholder:text-gray-400 focus:outline-none"
          type="password"
        >
        <button
          class="font-button bg-primary relative cursor-pointer border border-black px-5 py-2 leading-tight text-black shadow-sm transition-colors duration-150 hover:border-black hover:bg-gray-200 focus:bg-gray-400 focus:outline-none active:bg-gray-400"
        >
          Login
        </button>
      </div>
    </template>
  </div>
  
  <div class="flex flex-col gap-4 p-3">
    <div
      v-if="error"
      class="border-l-4 border-red-500 bg-gray-100 px-4 py-3"
    >
      {{ error }}
    </div>

    <!-- ******* BASE *******  -->
    <BaseComponent
      v-for="base in filterResources(resources, 'rdk', 'component', 'base')"
      :key="base.name"
      :name="base.name"
      :resources="resources"
      @showcamera="handleSelectCamera($event, filterResources(resources, 'rdk', 'component', 'camera'))"
    />

    <!-- ******* GANTRY *******  -->
    <Gantry
      v-for="gantry in filterRdkComponentsWithStatus(resources, status, 'gantry')"
      :key="gantry.name"
      :name="gantry.name"
      :status="resourceStatusByName(gantry)"
    />

    <!-- ******* MovementSensor *******  -->
    <MovementSensor
      v-for="sensor in filterResources(resources, 'rdk', 'component', 'movement_sensor')"
      :key="sensor.name"
      :name="sensor.name"
      :data="movementsensorData[sensor.name]"
    />

    <!-- ******* ARM *******  -->
    <Arm
      v-for="arm in filterResources(resources, 'rdk', 'component', 'arm')"
      :key="arm.name"
      :name="arm.name"
      :status="resourceStatusByName(arm)"
      :raw-status="rawResourceStatusByName(arm)"
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
      :servo-name="servo.name"
      :servo-angle="resourceStatusByName(servo).positionDeg"
      :crumbs="['servo']"
      @servo-move="(amount) => servoMove(servo, amount)"
      @servo-stop="servoStop(servo.name)"
    />

    <!-- ******* MOTOR *******  -->
    <MotorDetail
      v-for="motor in filterRdkComponentsWithStatus(resources, status, 'motor')"
      :key="'new-' + motor.name" 
      :motor-name="motor.name" 
      :crumbs="['motor']" 
      :motor-status="resourceStatusByName(motor)"
      @motor-run="motorCommand(motor.name, $event)"
      @motor-stop="motorStop(motor.name)"
    />

    <!-- ******* INPUT VIEW *******  -->
    <InputController
      v-for="controller in filteredInputControllerList()"
      :key="'new-' + controller.name"
      :controller-name="controller.name"
      :controller-status="resourceStatusByName(controller)"
      :crumbs="['input_controller']"
      class="input"
    />

    <!-- ******* WEB CONTROLS *******  -->
    <Gamepad
      v-if="hasWebGamepad()"
      style="max-width: 1080px;"
      @execute="inputInject($event)"
    />

    <!-- ******* BOARD *******  -->
    <Board
      v-for="board in filterRdkComponentsWithStatus(resources, status, 'board')"
      :key="board.name"
      :name="board.name"
      :status="resourceStatusByName(board)"
    />

    <!-- ******* CAMERAS *******  -->
    <Camera
      v-for="camera in filterResources(resources, 'rdk', 'component', 'camera')"
      :key="camera.name"
      :camera-name="camera.name"
      :crumbs="[camera.name]"
      :resources="resources"
      @toggle-camera="isOn => { viewCamera(camera.name, isOn) }"
      @refresh-camera="t => { viewCameraFrame(camera.name, t) }"
      @selected-camera-view="t => { viewCameraFrame(camera.name, t) }"
      @download-screenshot="renderFrame(camera.name)"
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
      :name="filterResources(resources, 'rdk', 'service', 'sensors')[0]?.name"
      :sensor-names="sensorNames"
    />

    <!-- ******* AUDIO INPUTS *******  -->
    <AudioInput
      v-for="audioInput in filterResources(resources, 'rdk', 'component', 'audio_input')"
      :key="audioInput.name"
      :stream-name="audioInput.name"
      :crumbs="[audioInput.name]"
      @toggle-input="isOn => { listenAudioInput(audioInput.name, isOn) }"
    />

    <!-- ******* SLAM *******  -->
    <Slam
      v-for="slam in filterResources(resources, 'rdk', 'service', 'slam')"
      :key="slam.name"
      :name="slam.name"
      :resources="resources"
    />

    <!-- ******* DO ******* -->
    <DoCommand :resources="filterResourcesWithNames(resources)" />

    <!-- ******* CURRENT OPERATIONS ******* -->
    <CurrentOperations
      :operations="currentOps"
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
