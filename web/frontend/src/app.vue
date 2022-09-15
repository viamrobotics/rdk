<!-- eslint-disable require-atomic-updates -->
<script>

import * as THREE from 'three';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls';
import { grpc } from '@improbable-eng/grpc-web';
import { toast } from './lib/toast';
import robotApi from './gen/proto/api/robot/v1/robot_pb.esm';
import commonApi from './gen/proto/api/common/v1/common_pb.esm';
import armApi from './gen/proto/api/component/arm/v1/arm_pb.esm';
import cameraApi from './gen/proto/api/component/camera/v1/camera_pb.esm';
import gantryApi from './gen/proto/api/component/gantry/v1/gantry_pb.esm';
import gripperApi from './gen/proto/api/component/gripper/v1/gripper_pb.esm';
import movementsensorApi from './gen/proto/api/component/movementsensor/v1/movementsensor_pb.esm';
import motionApi from './gen/proto/api/service/motion/v1/motion_pb.esm';
import visionApi from './gen/proto/api/service/vision/v1/vision_pb.esm';
import sensorsApi from './gen/proto/api/service/sensors/v1/sensors_pb.esm';
import servoApi from './gen/proto/api/component/servo/v1/servo_pb.esm';
import slamApi from './gen/proto/api/service/slam/v1/slam_pb.esm';
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
  BoardControlHelper,
  ServoControlHelper,
} from './rc/control_helpers';

import { addResizeListeners } from './lib/resize';

import BaseComponent from './components/base.vue';
import Camera from './components/camera.vue';
import AudioInput from './components/audio-input.vue';
import DoCommand from './components/do-command.vue';
import Gamepad from './components/gamepad.vue';
import InputController from './components/input-controller.vue';
import MotorDetail from './components/motor-detail.vue';
import Navigation from './components/navigation.vue';
import ServoComponent from './components/servo.vue';
import Slam from './components/slam.vue';
import { roundTo2Decimals } from './lib/math';
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
    BaseComponent,
    Camera,
    AudioInput,
    DoCommand,
    Gamepad,
    InputController,
    MotorDetail,
    Navigation,
    ServoComponent,
    Slam,
  },
  data() {
    return {
      supportedAuthTypes: window.supportedAuthTypes,
      error: '',
      res: {},
      rawStatus: {},
      status: {},
      pcdClick: {},
      sensorReadings: {},
      resources: [],
      sensorNames: [],
      cameraFrameIntervalId: null,
      slamImageIntervalId: null,
      slamPCDIntervalId: null,
      segmenterNames: [],
      segmenterParameterNames: [],
      segmenterParameters: {},
      segmentAlgo: '',
      fullcloud: null,
      objects: null,
      armToggle: {},
      value: 0,
      movementsensorData: {},
      currentOps: [],
      setPin: '',
      getPin: '',
      pwm: '',
      pwmFrequency: '',
      imageMapTemp: '',
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

      let interval = null;
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
    parameterType(typeName) {
      if (typeName === 'int' || typeName === 'float64') {
        return 'number';
      } else if (typeName === 'string' || typeName === 'char') {
        return 'text';
      }
      return '';
    },
    getSegmenterNames() {
      const req = new visionApi.GetSegmenterNamesRequest();
      // We are deliberately just getting the first vision service to ensure this will not break.
      // May want to allow for more services in the future
      const visionName = filterResources(this.resources, 'rdk', 'services', 'vision')[0];
      
      req.setName(visionName);

      window.visionService.getSegmenterNames(req, new grpc.Metadata(), (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          console.log('error getting segmenter names');
          console.log(err);
          return;
        }
        this.segmenterNames = resp.getSegmenterNamesList();
      });
    },
    getSegmenterParameters(name) {
      this.segmentAlgo = name;
      const req = new visionApi.GetSegmenterParametersRequest();
      // We are deliberately just getting the first vision service to ensure this will not break.
      // May want to allow for more services in the future
      const visionName = filterResources(this.resources, 'rdk', 'services', 'vision')[0];

      req.setName(visionName);
      req.setSegmenterName(name);
      
      window.visionService.getSegmenterParameters(req, new grpc.Metadata(), (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          console.log(`error getting segmenter parameters for ${name}`);
          console.log(err);
          return;
        }
        this.segmenterParameterNames = resp.getSegmenterParametersList();
        this.segmenterParameters = {};
      });
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
    gantryInc(name, axis, amount) {
      const g = this.resourceStatusByName(name);
      const pos = [];
      for (let i = 0; i < g.parts.length; i++) {
        pos[i] = g.parts[i].pos;
      }
      pos[axis] += amount;

      const req = new gantryApi.MoveToPositionRequest();
      req.setName(name.name);
      req.setPositionsMmList(pos);
      window.gantryService.moveToPosition(req, new grpc.Metadata(), this.grpcCallback);
    },
    gantryStop(name) {
      const request = new gantryApi.StopRequest();
      request.setName(name);
      window.gantryService.stop(request, new grpc.Metadata(), this.grpcCallback);
    },
    armEndPositionInc(name, getterSetter, amount) {
      const adjustedAmount = getterSetter[0] === 'o' || getterSetter[0] === 'O' ? amount / 100 : amount;
      const arm = this.rawResourceStatusByName(name);
      const old = arm.end_position;
      const newPose = new commonApi.Pose();
      const fieldSetters = [
        ['x', 'X'],
        ['y', 'Y'],
        ['z', 'Z'],
        ['theta', 'Theta'],
        ['o_x', 'OX'],
        ['o_y', 'OY'],
        ['o_z', 'OZ'],
      ];
      for (const fieldSetter of fieldSetters) {
        const endPositionField = fieldSetter[0];
        const endPositionValue = old[endPositionField] || 0;
        const setter = `set${fieldSetter[1]}`;
        newPose[setter](endPositionValue);
      }

      const getter = `get${getterSetter}`;
      const setter = `set${getterSetter}`;
      newPose[setter](newPose[getter]() + adjustedAmount);
      const req = new armApi.MoveToPositionRequest();
      req.setName(name.name);
      req.setTo(newPose);
      window.armService.moveToPosition(req, new grpc.Metadata(), this.grpcCallback);
    },
    armJointInc(name, field, amount) {
      const arm = this.rawResourceStatusByName(name);
      const newPositionDegs = new armApi.JointPositions();
      const newList = arm.joint_positions.values;
      newList[field] += amount;
      newPositionDegs.setValuesList(newList);
      const req = new armApi.MoveToJointPositionsRequest();
      req.setName(name.name);
      req.setPositions(newPositionDegs);
      window.armService.moveToJointPositions(req, new grpc.Metadata(), this.grpcCallback);
    },
    armHome(name) {
      const arm = this.rawResourceStatusByName(name);
      const newPositionDegs = new armApi.JointPositions();
      const newList = arm.joint_positions.values;
      for (let i = 0; i < newList.length; i++) {
        newList[i] = 0;
      }
      newPositionDegs.setValuesList(newList);
      const req = new armApi.MoveToJointPositionsRequest();
      req.setName(name.name);
      req.setPositions(newPositionDegs);
      window.armService.moveToJointPositions(req, {}, this.grpcCallback);
    },
    armModifyAll(name) {
      const arm = this.resourceStatusByName(name);
      const n = {
        pos_pieces: [],
        joint_pieces: [],
      };
      for (let i = 0; i < arm.pos_pieces.length; i++) {
        n.pos_pieces.push({
          endPosition: arm.pos_pieces[i].endPosition,
          endPositionValue: roundTo2Decimals(arm.pos_pieces[i].endPositionValue),
        });
      }
      for (let i = 0; i < arm.joint_pieces.length; i++) {
        n.joint_pieces.push({
          joint: arm.joint_pieces[i].joint,
          jointValue: roundTo2Decimals(arm.joint_pieces[i].jointValue),
        });
      }
      this.armToggle[name.name] = n;
    },
    armModifyAllCancel(name) {
      delete this.armToggle[name.name];
    },
    armModifyAllDoEndPosition(name) {
      const newPose = new commonApi.Pose();
      const newPieces = this.armToggle[name.name].pos_pieces;

      for (const newPiece of newPieces) {
        const getterSetter = newPiece.endPosition[1];
        const setter = `set${getterSetter}`;
        newPose[setter](newPiece.endPositionValue);
      }

      const req = new armApi.MoveToPositionRequest();
      req.setName(name.name);
      req.setTo(newPose);
      armService.moveToPosition(req, {}, this.grpcCallback);
      delete this.armToggle[name.name];
    },
    armModifyAllDoJoint(name) {
      const arm = this.rawResourceStatusByName(name);
      const newPositionDegs = new armApi.JointPositions();
      const newList = arm.joint_positions.values;
      const newPieces = this.armToggle[name.name].joint_pieces;
      for (let i = 0; i < newPieces.length && i < newList.length; i++) {
        newList[newPieces[i].joint] = newPieces[i].jointValue;
      }

      newPositionDegs.setValuesList(newList);
      const req = new armApi.MoveToJointPositionsRequest();
      req.setName(name.name);
      req.setPositions(newPositionDegs);
      window.armService.moveToJointPositions(req, new grpc.Metadata(), this.grpcCallback);
      delete this.armToggle[name.name];
    },
    armStop(name) {
      const request = new armApi.StopRequest();
      request.setName(name);
      window.armService.stop(request, new grpc.Metadata(), this.grpcCallback);
    },
    gripperAction(name, action) {
      let req;
      switch (action) {
        case 'open':
          req = new gripperApi.OpenRequest();
          req.setName(name);
          window.gripperService.open(req, new grpc.Metadata(), this.grpcCallback);
          break;
        case 'grab':
          req = new gripperApi.GrabRequest();
          req.setName(name);
          window.gripperService.grab(req, new grpc.Metadata(), this.grpcCallback);
          break;
      }
    },
    gripperStop(name) {
      const request = new gripperApi.StopRequest();
      request.setName(name);
      window.gripperService.stop(request, new grpc.Metadata(), this.grpcCallback);
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
    killOp(id) {
      const req = new robotApi.CancelOperationRequest();
      req.setId(id);
      window.robotService.cancelOperation(req, new grpc.Metadata(), this.grpcCallback);
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
    renderPCD(cameraName) {
      this.$nextTick(() => {
        this.pcdClick.pcdloaded = false;
        this.pcdClick.foundSegments = false;
        this.initPCDIfNeeded();
        pcdGlobal.cameraName = cameraName;

        const req = new cameraApi.GetPointCloudRequest();
        req.setName(cameraName);
        req.setMimeType('pointcloud/pcd');
        window.cameraService.getPointCloud(req, new grpc.Metadata(), (err, resp) => {
          this.grpcCallback(err, resp, false);
          if (err) {
            return;
          }
          console.log('loading pcd');
          this.fullcloud = resp.getPointCloud_asB64();
          this.pcdLoad(`data:pointcloud/pcd;base64,${this.fullcloud}`);
        });
      });

      this.getSegmenterNames();
    },
    updateSLAMImageRefreshFrequency(name, time) {
      clearInterval(this.slamImageIntervalId);
      if (time === 'manual') {
        this.viewSLAMImageMap(name);
      } else if (time === 'off') {
        // do nothing
      } else {
        this.viewSLAMImageMap(name);
        this.slamImageIntervalId = window.setInterval(() => {
          this.viewSLAMImageMap(name);
        }, Number(time) * 1000);
      }
    },
    viewSLAMImageMap(name) {
      const req = new slamApi.GetMapRequest();
      req.setName(name);
      req.setMimeType('image/jpeg');
      req.setIncludeRobotMarker(true);
      window.slamService.getMap(req, new grpc.Metadata(), (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        const blob = new Blob([resp.getImage_asU8()], { type: 'image/jpeg' });
        this.imageMapTemp = URL.createObjectURL(blob);
      });
    },
    updateSLAMPCDRefreshFrequency(name, time, load) {
      clearInterval(this.slamPCDIntervalId);
      if (time === 'manual') {
        this.viewSLAMPCDMap(name, load);
      } else if (time === 'off') {
        // do nothing
      } else {
        this.viewSLAMPCDMap(name, load);
        this.slamPCDIntervalId = window.setInterval(() => {
          this.viewSLAMPCDMap();
        }, Number(time) * 1000);
      }
    },
    viewSLAMPCDMap(name, load) {
      this.$nextTick(() => {
        const req = new slamApi.GetMapRequest();
        req.setName(name);
        req.setMimeType('pointcloud/pcd');
        if (load) {
          this.initPCD();
        }
        window.slamService.getMap(req, new grpc.Metadata(), (err, resp) => {
          this.grpcCallback(err, resp, false);
          if (err) {
            return;
          }
          const pcObject = resp.getPointCloud();
          this.fullcloud = pcObject.getPointCloud_asB64();
          this.pcdLoad(`data:pointcloud/pcd;base64,${this.fullcloud}`);
        });
      });
    },
    getReadings(sensorNames) {
      const req = new sensorsApi.GetReadingsRequest();
      const sensorsName = filterResources(this.resources, 'rdk', 'service', 'sensors')[0];
      const names = sensorNames.map((name) => {
        const resourceName = new commonApi.ResourceName();
        resourceName.setNamespace(name.namespace);
        resourceName.setType(name.type);
        resourceName.setSubtype(name.subtype);
        resourceName.setName(name.name);
        return resourceName;
      });
      req.setName(sensorsName.name);
      req.setSensorNamesList(names);
      window.sensorsService.getReadings(req, new grpc.Metadata(), (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }

        for (const r of resp.getReadingsList()) {
          const readings = r.getReadingsMap();
          const rr = {};

          for (const [k, v] of readings.entries()) {
            rr[k] = v.toJavaScript();
          }
          
          this.sensorReadings[resourceNameToString(r.getName().toObject())] = rr;
        }
      });
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
    grabClick(e) {
      const mouse = new THREE.Vector2();
      mouse.x = (e.offsetX / e.srcElement.offsetWidth) * 2 - 1;
      mouse.y = (e.offsetY / e.srcElement.offsetHeight) * -2 + 1;

      pcdGlobal.raycaster.setFromCamera(mouse, pcdGlobal.camera);

      const intersects = pcdGlobal.raycaster.intersectObjects(pcdGlobal.scene.children);
      const p = intersects.length > 0 ? intersects[0] : null;

      if (p !== null) {
        console.log(p.point);
        this.setPoint(p.point);
      } else {
        console.log('no point intersected');
      }
    },
    doPCDMove() {
      const gripperName = filterResources(this.resources, 'rdk', 'component', 'gripper')[0];
      const cameraName = pcdGlobal.cameraName;
      const cameraPointX = this.pcdClick.x;
      const cameraPointY = this.pcdClick.y;
      const cameraPointZ = this.pcdClick.z;

      const req = new motionApi.MoveRequest();
      const cameraPoint = new commonApi.Pose();
      // We are deliberately just getting the first motion service to ensure this will not break.
      // May want to allow for more services in the future
      const motionName = filterResources(this.resources, 'rdk', 'services', 'motion')[0];
      cameraPoint.setX(cameraPointX);
      cameraPoint.setY(cameraPointY);
      cameraPoint.setZ(cameraPointZ);

      const pose = new commonApi.PoseInFrame();
      pose.setReferenceFrame(cameraName);
      pose.setPose(cameraPoint);
      req.setDestination(pose);
      req.setName(motionName);
      const componentName = new commonApi.ResourceName();
      componentName.setNamespace(gripperName.namespace);
      componentName.setType(gripperName.type);
      componentName.setSubtype(gripperName.subtype);
      componentName.setName(gripperName.name);
      req.setComponentName(componentName);
      console.log(`making move attempt using ${gripperName}`);

      window.motionService.move(req, new grpc.Metadata(), (err, resp) => {
        this.grpcCallback(err, resp);
        if (err) {
          return Promise.reject(err);
        }
        return Promise.resolve(resp).then(() => console.log(`move success: ${resp.getSuccess()}`));
      });
    },
    findSegments(segmenterName, segmenterParams) {
      console.log('parameters for segmenter below:');
      console.log(segmenterParams);
      this.pcdClick.calculatingSegments = true;
      this.pcdClick.foundSegments = false;
      const req = new visionApi.GetObjectPointCloudsRequest();
      // We are deliberately just getting the first vision service to ensure this will not break.
      // May want to allow for more services in the future
      const visionName = filterResources(this.resources, 'rdk', 'services', 'vision')[0];
      
      req.setName(visionName);
      req.setCameraName(pcdGlobal.cameraName);
      req.setSegmenterName(segmenterName);
      req.setParameters(proto.google.protobuf.Struct.fromJavaScript(segmenterParams));
      const mimeType = 'pointcloud/pcd';
      req.setMimeType(mimeType);
      console.log('finding object segments...');
      window.visionService.getObjectPointClouds(req, new grpc.Metadata(), (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          console.log('error getting segments');
          console.log(err);
          this.pcdClick.calculatingSegments = false;
          return;
        }
        console.log('got pcd segments');
        this.pcdClick.foundSegments = true;
        this.objects = resp.getObjectsList();
        this.pcdClick.calculatingSegments = false;
      });
    },
    doSegmentLoad (i) {
      const segment = this.objects[i];
      const data = segment.getPointCloud_asB64();
      const center = segment.getGeometries().getGeometriesList()[0].getCenter();
      const box = segment.getGeometries().getGeometriesList()[0].getBox();
      const p = { x: center.getX() / 1000, y: center.getY() / 1000, z: center.getZ() / 1000 };
      console.log(p);
      this.setPoint(p);
      setBoundingBox(box, p);
      this.pcdLoad(`data:pointcloud/pcd;base64,${data}`);
    },
    doPointLoad (i) {
      const segment = this.objects[i];
      const center = segment.getGeometries().getGeometriesList()[0].getCenter();
      this.setPoint({
        x: center.getX() / 1000,
        y: center.getY() / 1000,
        z: center.getZ() / 1000,
      });
    },
    doBoundingBoxLoad (i) {
      const segment = this.objects[i];
      const center = segment.getGeometries().getGeometriesList()[0].getCenter();
      const box = segment.getGeometries().getGeometriesList()[0].getBox();
      const centerP = {
        x: center.getX() / 1000,
        y: center.getY() / 1000,
        z: center.getZ() / 1000,
      };
      setBoundingBox(box, centerP);
    },
    doPCDLoad (data) {
      this.pcdLoad(`data:pointcloud/pcd;base64,${data}`);
    },
    doCenterPCDLoad (data) {
      this.pcdLoad(`data:pointcloud/pcd;base64,${data}`);
      const p = { x: 0 / 1000, y: 0 / 1000, z: 0 / 1000 };
      console.log(p);
      this.setPoint(p);
    },
    doPCDDownload(data) {
      window.open(`data:pointcloud/pcd;base64,${data}`);
    },
    doSelectObject(selection, index) {
      switch (selection) {
        case 'Center Point':
          this.doSegmentLoad(index);
          break;
        case 'Bounding Box':
          this.doBoundingBoxLoad(index);
          break;
        case 'Cropped':
          this.doPointLoad(index);
          break;
        default:
          break;
      }
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
    getGPIO (boardName) {
      const pin = this.getPin;
      BoardControlHelper.getGPIO(boardName, pin, (err, resp) => {
        if (err) {
          toast.error(err);
          return;
        }
        const x = resp.toObject();
        document.querySelector(`#get_pin_value_${boardName}`).innerHTML = `Pin: ${pin} is ${x.high ? 'high' : 'low'}`;
      });
    },
    setGPIO (boardName) {
      const pin = this.setPin;
      const v = document.querySelector(`#set_pin_v_${boardName}`).value;
      BoardControlHelper.setGPIO(boardName, pin, v === 'high', this.grpcCallback);
    },
    getPWM (boardName) {
      const pin = this.getPin;
      BoardControlHelper.getPWM(boardName, pin, (err, resp) => {
        if (err) {
          toast.error(err);
          return;
        }
        const { dutyCyclePct } = resp.toObject();
        document.querySelector(`#get_pin_value_${boardName}`).innerHTML = `Pin ${pin}'s duty cycle is ${dutyCyclePct * 100}%.`;
      });
    },
    setPWM (boardName) {
      const pin = this.setPin;
      const v = this.pwm / 100;
      BoardControlHelper.setPWM(boardName, pin, v, this.grpcCallback);
    },
    getPWMFrequency (boardName) {
      const pin = this.getPin;
      BoardControlHelper.getPWMFrequency(boardName, pin, (err, resp) => {
        if (err) {
          toast.error(err);
          return;
        }
        const { frequencyHz } = resp.toObject();
        document.querySelector(`#get_pin_value_${boardName}`).innerHTML = `Pin ${pin}'s frequency is ${frequencyHz}Hz.`;
      });
    },
    setPWMFrequency (boardName) {
      const pin = this.setPin;
      const v = this.pwmFrequency;
      BoardControlHelper.setPWMFrequency(boardName, pin, v, this.grpcCallback);
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
      // We are deliberately just getting the first sensors service to ensure this will not break.
      // May want to allow for more services in the future
      const sensorsName = filterResources(this.resources, 'rdk', 'service', 'sensors')[0];
      const req = new sensorsApi.GetSensorsRequest();
      req.setName(sensorsName.name);
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
    initPCDIfNeeded() {
      if (pcdGlobal) {
        return;
      }
      
      this.initPCD();
    },
    initPCD() {
      this.pcdClick.enable = true;

      const sphereGeometry = new THREE.SphereGeometry(0.009, 32, 32);
      const sphereMaterial = new THREE.MeshBasicMaterial({ color: 0xFF_00_00 });

      pcdGlobal = {
        scene: new THREE.Scene(),
        camera: new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 2000),
        renderer: new THREE.WebGLRenderer(),
        raycaster: new THREE.Raycaster(),
        sphere: new THREE.Mesh(sphereGeometry, sphereMaterial),
      };

      pcdGlobal.renderer.setSize(window.innerWidth / 2, window.innerHeight / 2);
      document.querySelector('#pcd').append(pcdGlobal.renderer.domElement);

      pcdGlobal.controls = new OrbitControls(pcdGlobal.camera, pcdGlobal.renderer.domElement);
      pcdGlobal.camera.position.set(0, 0, 0);
      pcdGlobal.controls.target.set(0, 0, -1);
      pcdGlobal.controls.update();
      pcdGlobal.camera.updateMatrix();
    },
    pcdLoad(path) {
      const loader = new PCDLoader();
      loader.load(
        path,

        // called when the resource is loaded
        (mesh) => {
          pcdGlobal.scene.clear();
          pcdGlobal.scene.add(mesh);
          pcdGlobal.scene.add(pcdGlobal.sphere);
          if (pcdGlobal.cube) {
            pcdGlobal.scene.add(pcdGlobal.cube);
          }
          pcdAnimate();
        },
        // called when loading is in progresses
        () => { /* noop */ },
        // called when loading has errors
        (_error) => { /* noop */ }
      );
      this.pcdClick.pcdloaded = true;
    },
    setPoint(point) {
      this.pcdClick.x = r(point.x);
      this.pcdClick.y = r(point.y);
      this.pcdClick.z = r(point.z);
      pcdGlobal.sphere.position.copy(point);
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

let pcdGlobal = null;

function resizeRendererToDisplaySize(renderer) {
  const canvas = renderer.domElement;
  const width = canvas.clientWidth;
  const height = canvas.clientHeight;
  const needResize = canvas.width !== width || canvas.height !== height;
  if (needResize) {
    renderer.setSize(width, height, false);
  }
  return needResize;
}

function pcdAnimate() {
  if (resizeRendererToDisplaySize(pcdGlobal.renderer)) {
    const canvas = pcdGlobal.renderer.domElement;
    pcdGlobal.camera.aspect = canvas.clientWidth / canvas.clientHeight;
    pcdGlobal.camera.updateProjectionMatrix();
  }
  pcdGlobal.renderer.render(pcdGlobal.scene, pcdGlobal.camera);
  pcdGlobal.controls.update();
  requestAnimationFrame(pcdAnimate);
}

function r(n) {
  return Math.round(n * 1000);
}

function setBoundingBox(box, centerPoint) {
  const geometry = new THREE.BoxGeometry(box.getWidthMm() / 1000, box.getLengthMm() / 1000, box.getDepthMm() / 1000);
  const edges = new THREE.EdgesGeometry(geometry);
  const material = new THREE.LineBasicMaterial({ color: 0xFF_00_00 });
  const cube = new THREE.LineSegments(edges, material);
  cube.position.copy(centerPoint);
  cube.name = 'bounding-box';
  pcdGlobal.scene.remove(pcdGlobal.scene.getObjectByName('bounding-box'));
  pcdGlobal.cube = cube;
  pcdGlobal.scene.add(cube);
}

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
          class="mb-2 block w-full appearance-none border p-2 text-gray-700 transition-colors duration-150 ease-in-out placeholder:text-gray-400 focus:outline-none dark:text-gray-200 dark:placeholder:text-gray-500"
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
    <template
      v-for="base in filterResources(resources, 'rdk', 'component', 'base')"
      :key="base.name"
    >
      <BaseComponent
        :name="base.name"
        :resources="resources"
        @showcamera="handleSelectCamera($event, filterResources(resources, 'rdk', 'component', 'camera'))"
      />
    </template>

    <!-- ******* GANTRY *******  -->
    <v-collapse
      v-for="gantry in filterRdkComponentsWithStatus(resources, status, 'gantry')"
      :key="gantry.name"
      :title="gantry.name"
      class="gantry"
    >
      <v-breadcrumbs
        slot="title"
        :crumbs="['gantry'].join(',')"
      />
      <div
        slot="header"
        class="flex items-center justify-between gap-2"
      >
        <v-button
          variant="danger"
          icon="stop-circle"
          label="STOP"
          @click.stop="gantryStop(gantry.name)"
        />
      </div>
      <div class="border border-t-0 border-black p-4">
        <table class="border border-t-0 border-black p-4">
          <thead>
            <tr>
              <th class="border border-black p-2">
                axis
              </th>
              <th
                class="border border-black p-2"
                colspan="2"
              >
                position
              </th>
              <th class="border border-black p-2">
                length
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="pp in resourceStatusByName(gantry).parts"
              :key="pp.axis"
            >
              <th class="border border-black p-2">
                {{ pp.axis }}
              </th>
              <td class="flex p-2">
                <v-button
                  label="--"
                  @click="gantryInc( gantry, pp.axis, -10 )"
                />
                <v-button
                  label="-"
                  @click="gantryInc( gantry, pp.axis, -1 )"
                />
                <v-button
                  label="+"
                  @click="gantryInc( gantry, pp.axis, 1 )"
                />
                <v-button
                  label="++"
                  @click="gantryInc( gantry, pp.axis, 10 )"
                />
              </td>
              <td class="border border-black p-2">
                {{ pp.pos.toFixed(2) }}
              </td>
              <td class="border border-black p-2">
                {{ pp.length }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </v-collapse>

    <!-- ******* MovementSensor *******  -->
    <v-collapse
      v-for="movementsensor in filterResources(resources, 'rdk', 'component', 'movement_sensor')"
      :key="movementsensor.name"
      :title="movementsensor.name"
      class="movement"
    >
      <v-breadcrumbs
        slot="title"
        :crumbs="['movement_sensor'].join(',')"
      />
      <div class="flex items-end border border-t-0 border-black p-4">
        <template v-if="movementsensorData[movementsensor.name] && movementsensorData[movementsensor.name].properties">
          <div
            v-if="movementsensorData[movementsensor.name].properties.positionSupported"
            class="mr-4 w-1/4"
          >
            <h3 class="mb-1">
              Position
            </h3>
            <table class="w-full border border-t-0 border-black p-4">
              <tr>
                <th class="border border-black p-2">
                  Latitude
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].coordinate?.latitude.toFixed(6) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Longitude
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].coordinate?.longitude.toFixed(6) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Altitide
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].altitudeMm?.toFixed(2) }}
                </td>
              </tr>
            </table>
            <a :href="'https://www.google.com/maps/search/' + movementsensorData[movementsensor.name].coordinate?.latitude + ',' + movementsensorData[movementsensor.name].coordinate?.longitude">google maps</a>
          </div>

          <div
            v-if="movementsensorData[movementsensor.name].properties.orientationSupported"
            class="mr-4 w-1/4"
          >
            <h3 class="mb-1">
              Orientation (degrees)
            </h3>
            <table class="w-full border border-t-0 border-black p-4">
              <tr>
                <th class="border border-black p-2">
                  OX
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].orientation?.oX.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  OY
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].orientation?.oY.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  OZ
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].orientation?.oZ.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Theta
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].orientation?.theta.toFixed(2) }}
                </td>
              </tr>
            </table>
          </div>
                
          <div
            v-if="movementsensorData[movementsensor.name].properties.angularVelocitySupported"
            class="mr-4 w-1/4"
          >
            <h3 class="mb-1">
              Angular Velocity (degrees/second)
            </h3>
            <table class="w-full border border-t-0 border-black p-4">
              <tr>
                <th class="border border-black p-2">
                  X
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].angularVelocity?.x.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Y
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].angularVelocity?.y.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Z
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].angularVelocity?.z.toFixed(2) }}
                </td>
              </tr>
            </table>
          </div>

          <div
            v-if="movementsensorData[movementsensor.name].properties.linearVelocitySupported"
            class="mr-4 w-1/4"
          >
            <h3 class="mb-1">
              Linear Velocity
            </h3>
            <table class="w-full border border-t-0 border-black p-4">
              <tr>
                <th class="border border-black p-2">
                  X
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].linearVelocity?.x.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Y
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].linearVelocity?.y.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Z
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].linearVelocity?.z.toFixed(2) }}
                </td>
              </tr>
            </table>
          </div>

          <div
            v-if="movementsensorData[movementsensor.name].properties.compassHeadingSupported"
            class="mr-4 w-1/4"
          >
            <h3 class="mb-1">
              Compass Heading
            </h3>
            <table class="w-full border border-t-0 border-black p-4">
              <tr>
                <th class="border border-black p-2">
                  Compass
                </th>
                <td class="border border-black p-2">
                  {{ movementsensorData[movementsensor.name].compassHeading?.toFixed(2) }}
                </td>
              </tr>
            </table>
          </div>
        </template>
      </div>
    </v-collapse>

    <!-- ******* ARM *******  -->
    <v-collapse
      v-for="arm in filterResources(resources, 'rdk', 'component', 'arm')"
      :key="arm.name"
      :title="arm.name"
      class="arm"
    >
      <v-breadcrumbs
        slot="title"
        :crumbs="['arm'].join(',')"
      />
      <div
        slot="header"
        class="flex items-center justify-between gap-2"
      >
        <v-button
          variant="danger"
          icon="stop-circle"
          label="STOP"
          @click.stop="armStop(arm.name)"
        />
      </div>
      <div class="mt-2 flex">
        <div
          v-if="armToggle[arm.name]"
          class="mr-4 w-1/2 border border-black p-4"
        >
          <h3 class="mb-2">
            END POSITION (mms)
          </h3>
          <div class="inline-grid grid-cols-2 gap-1 pb-1">
            <template
              v-for="cc in armToggle[arm.name].pos_pieces"
              :key="cc.endPosition[0]"
            >
              <label class="py-1 pr-2 text-right">{{ cc.endPosition[1] }}</label>
              <input
                v-model="cc.endPositionValue"
                class="border border-black py-1 px-4"
              >
            </template>
          </div>
          <div class="mt-2 flex gap-2">
            <v-button
              class="mr-4 whitespace-nowrap"
              label="Go To End Position"
              @click="armModifyAllDoEndPosition(arm)"
            />
            <div class="flex-auto text-right">
              <v-button
                label="Cancel"
                @click="armModifyAllCancel(arm)"
              />
            </div>
          </div>
        </div>
        <div
          v-if="armToggle[arm.name]"
          class="w-1/2 border border-black p-4"
        >
          <h3 class="mb-2">
            JOINTS (degrees)
          </h3>
          <div class="grid grid-cols-2 gap-1 pb-1">
            <template
              v-for="bb in armToggle[arm.name].joint_pieces"
              :key="bb.joint"
            >
              <label class="py-1 pr-2 text-right">Joint {{ bb.joint }}</label>
              <input
                v-model="bb.jointValue"
                class="border border-black py-1 px-4"
              >
            </template>
          </div>
          <div class="mt-2 flex gap-2">
            <v-button
              label="Go To Joints"
              @click="armModifyAllDoJoint(arm)"
            />
            <div class="flex-auto text-right">
              <v-button
                label="Cancel"
                @click="armModifyAllCancel(arm)"
              />
            </div>
          </div>
        </div>
      </div>

      <div class="mb-2 flex">
        <div
          v-if="resourceStatusByName(arm)"
          class="mr-4 w-1/2 border border-black p-4"
        >
          <h3 class="mb-2">
            END POSITION (mms)
          </h3>
          <div class="inline-grid grid-cols-6 gap-1 pb-1">
            <template
              v-for="aa in resourceStatusByName(arm).pos_pieces"
              :key="aa.endPosition[0]"
            >
              <h4 class="py-1 pr-2 text-right">
                {{ aa.endPosition[1] }}
              </h4>
              <v-button
                label="--"
                @click="armEndPositionInc( arm, aa.endPosition[1], -10 )"
              />
              <v-button
                label="-"
                @click="armEndPositionInc( arm, aa.endPosition[1], -1 )"
              />
              <v-button
                label="+"
                @click="armEndPositionInc( arm, aa.endPosition[1], 1 )"
              />
              <v-button
                label="++"
                @click="armEndPositionInc( arm, aa.endPosition[1], 10 )"
              />
              <h4 class="py-1">
                {{ aa.endPositionValue.toFixed(2) }}
              </h4>
            </template>
          </div>
          <div class="mt-2 flex gap-2">
            <v-button
              label="Home"
              @click="armHome(arm)"
            />
            <div class="flex-auto text-right">
              <v-button
                class="whitespace-nowrap"
                label="Modify All"
                @click="armModifyAll(arm)"
              />
            </div>
          </div>
        </div>
        <div
          v-if="resourceStatusByName(arm)"
          class="w-1/2 border border-black p-4"
        >
          <h3 class="mb-2">
            JOINTS (degrees)
          </h3>
          <div class="inline-grid grid-cols-6 gap-1 pb-1">
            <template
              v-for="aa in resourceStatusByName(arm).joint_pieces"
              :key="aa.joint"
            >
              <h4 class="whitespace-nowrap py-1 pr-2 text-right">
                Joint {{ aa.joint }}
              </h4>
              <v-button
                label="--"
                @click="armJointInc( arm, aa.joint, -10 )"
              />
              <v-button
                label="-"
                @click="armJointInc( arm, aa.joint, -1 )"
              />
              <v-button
                label="+"
                @click="armJointInc( arm, aa.joint, 1 )"
              />
              <v-button
                label="++"
                @click="armJointInc( arm, aa.joint, 10 )"
              />
              <h4 class="py-1 pl-2">
                {{ aa.jointValue.toFixed(2) }}
              </h4>
            </template>
          </div>
          <div class="mt-2 flex gap-2">
            <v-button
              label="Home"
              @click="armHome(arm)"
            />
            <div class="flex-auto text-right">
              <v-button
                class="whitespace-nowrap"
                label="Modify All"
                @click="armModifyAll(arm)"
              />
            </div>
          </div>
        </div>
      </div>
    </v-collapse>

    <!-- ******* GRIPPER *******  -->
    <v-collapse
      v-for="gripper in filterResources(resources, 'rdk', 'component', 'gripper')"
      :key="gripper.name"
      :title="gripper.name"
      class="gripper"
    >
      <v-breadcrumbs
        slot="title"
        :crumbs="['gripper'].join(',')"
      />
      <div
        slot="header"
        class="flex items-center justify-between gap-2"
      >
        <v-button
          variant="danger"
          icon="stop-circle"
          label="STOP"
          @click.stop="gripperStop(gripper.name)"
        />
      </div>
      <div class="flex gap-2 border border-t-0 border-black p-4">
        <v-button
          label="Open"
          @click="gripperAction( gripper.name, 'open')"
        />
        <v-button
          label="Grab"
          @click="gripperAction( gripper.name, 'grab')"
        />
      </div>
    </v-collapse>

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
    <v-collapse
      v-for="board in filterRdkComponentsWithStatus(resources, status, 'board')"
      :key="board.name"
      :title="board.name"
      class="board"
    >
      <v-breadcrumbs
        slot="title"
        :crumbs="['board'].join(',')"
      />
      <div class="border border-t-0 border-black p-4">
        <h3 class="mb-2">
          Analogs
        </h3>
        <table class="mb-4 table-auto border border-black">
          <tr
            v-for="(analog, name) in resourceStatusByName(board).analogsMap"
            :key="name"
          >
            <th class="border border-black p-2">
              {{ name }}
            </th>
            <td class="border border-black p-2">
              {{ analog.value || 0 }}
            </td>
          </tr>
        </table>
        <h3 class="mb-2">
          Digital Interrupts
        </h3>
        <table class="mb-4 w-full table-auto border border-black">
          <tr
            v-for="(di, name) in resourceStatusByName(board).digitalInterruptsMap"
            :key="name"
          >
            <th class="border border-black p-2">
              {{ name }}
            </th>
            <td class="border border-black p-2">
              {{ di.value || 0 }}
            </td>
          </tr>
        </table>
        <h3 class="mb-2">
          GPIO
        </h3>
        <table class="mb-4 w-full table-auto border border-black">
          <tr>
            <th class="border border-black p-2">
              Get
            </th>
            <td class="border border-black p-2">
              <div class="flex items-end gap-2">
                <v-input
                  label="Pin"
                  type="number"
                  :value="getPin"
                  @input="getPin = $event.detail.value"
                />
                <v-button
                  label="Get Pin State"
                  @click="getGPIO(board.name)"
                />
                <v-button
                  label="Get PWM"
                  @click="getPWM(board.name)"
                />
                <v-button
                  label="Get PWM Frequency"
                  @click="getPWMFrequency(board.name)"
                />
                <span
                  :id="'get_pin_value_' + board.name"
                  class="py-2"
                />
              </div>
            </td>
          </tr>
          <tr>
            <th class="border border-black p-2">
              Set
            </th>
            <td class="p-2">
              <div class="flex items-end gap-2">
                <v-input
                  type="number"
                  class="mr-2"
                  label="Pin"
                  :value="setPin"
                  @input="setPin = $event.detail.value"
                />
                <select
                  :id="'set_pin_v_' + board.name"
                  class="mr-2 h-[30px] border border-black bg-white text-sm"
                >
                  <option>low</option>
                  <option>high</option>
                </select>
                <v-button
                  class="mr-2"
                  label="Set Pin State"
                  @click="setGPIO(board.name)"
                />
                <v-input
                  v-model="pwm"
                  label="PWM"
                  type="number"
                  class="mr-2"
                />
                <v-button
                  class="mr-2"
                  label="Set PWM"
                  @click="setPWM(board.name)"
                />
                <v-input
                  v-model="pwmFrequency"
                  label="PWM Frequency"
                  type="number"
                  class="mr-2"
                />
                <v-button
                  class="mr-2"
                  label="Set PWM Frequency"
                  @click="setPWMFrequency(board.name)"
                />
              </div>
            </td>
          </tr>
        </table>
      </div>
    </v-collapse>

    <!-- ******* CAMERAS *******  -->
    <Camera
      v-for="camera in filterResources(resources, 'rdk', 'component', 'camera')"
      :key="camera.name"
      :stream-name="camera.name"
      :crumbs="[camera.name]"
      :x="pcdClick.x"
      :y="pcdClick.y"
      :z="pcdClick.z"
      :pcd-click="pcdClick"
      :segmenter-names="segmenterNames"
      :segmenter-parameters="segmenterParameters"
      :segmenter-parameter-names="segmenterParameterNames"
      :parameter-type="parameterType"
      :segment-algo="segmentAlgo"
      :segment-objects="objects"
      :find-status="pcdClick.calculatingSegments"
      @full-image="doPCDLoad(fullcloud)"
      @center-pcd="doCenterPCDLoad(fullcloud)"
      @find-segments="findSegments(segmentAlgo, segmenterParameters)"
      @change-segmenter="getSegmenterParameters"
      @toggle-camera="isOn => { viewCamera(camera.name, isOn) }"
      @refresh-camera="t => { viewCameraFrame(camera.name, t) }"
      @selected-camera-view="t => { viewCameraFrame(camera.name, t) }"
      @toggle-pcd="renderPCD(camera.name)"
      @pcd-click="grabClick"
      @pcd-move="doPCDMove"
      @point-load="doPointLoad"
      @segment-load="doSegmentLoad"
      @bounding-box-load="doBoundingBoxLoad"
      @download-screenshot="renderFrame(camera.name)"
      @download-raw-data="doPCDDownload(fullcloud)"
      @select-object="doSelectObject"
      @segmenter-parameters-input="(name, value) => segmenterParameters[name] = Number(value)"
    />

    <!-- ******* NAVIGATION ******* -->
    <Navigation
      v-for="nav in filterResources(resources, 'rdk', 'service', 'navigation')"
      :key="nav.name"
      :resources="nav.resources"
      :name="nav.name"
    />

    <!-- ******* SENSORS ******* -->
    <v-collapse
      v-if="nonEmpty(sensorNames)"
      title="Sensors"
      class="sensors"
    >
      <div class="border border-t-0 border-black p-4">
        <table class="w-full table-auto border border-black">
          <tr>
            <th class="border border-black p-2">
              Name
            </th>
            <th class="border border-black p-2">
              Type
            </th>
            <th class="border border-black p-2">
              Readings
            </th>
            <th class="border border-black p-2 text-center">
              <v-button
                group
                label="Get All Readings"
                @click="getReadings(sensorNames)"
              />
            </th>
          </tr>
          <tr
            v-for="name in sensorNames"
            :key="name.name"
          >
            <td class="border border-black p-2">
              {{ name.name }}
            </td>
            <td class="border border-black p-2">
              {{ name.subtype }}
            </td>
            <td class="border border-black p-2">
              <table style="font-size:.7em; text-align: left;">
                <tr
                  v-for="(sensorValue, sensorField) in sensorReadings[resourceNameToString(name)]"
                  :key="sensorField"
                >
                  <th>{{ sensorField }}</th>
                  <td>
                    {{ sensorValue }}
                    <a
                      v-if="sensorValue._type == 'geopoint'"
                      :href="'https://www.google.com/maps/search/' + sensorValue.lat + ',' + sensorValue.lng"
                    >google maps</a>
                  </td>
                </tr>
              </table>
            </td>
            <td class="border border-black p-2 text-center">
              <v-button
                group
                label="Get Readings"
                @click="getReadings([name])"
              />
            </td>
          </tr>
        </table>
      </div>
    </v-collapse>

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
      :image-map="imageMapTemp"
      @update-slam-image-refresh-frequency="updateSLAMImageRefreshFrequency"
      @update-slam-pcd-refresh-frequency="updateSLAMPCDRefreshFrequency"
    />

    <!-- ******* DO ******* -->
    <DoCommand :resources="filterResourcesWithNames(resources)" />

    <!-- ******* CURRENT OPERATIONS ******* -->
    <v-collapse
      title="Current Operations"
      class="operations"
    >
      <div class="border border-t-0 border-black p-4">
        <table class="w-full table-auto border border-black">
          <tr>
            <th class="border border-black p-2">
              id
            </th>
            <th class="border border-black p-2">
              method
            </th>
            <th class="border border-black p-2">
              elapsed time
            </th>
            <th class="border border-black p-2" />
          </tr>
          <tr
            v-for="o in currentOps"
            :key="o.id"
          >
            <td class="border border-black p-2">
              {{ o.id }}
            </td>
            <td class="border border-black p-2">
              {{ o.method }}
            </td>
            <td class="border border-black p-2">
              {{ o.elapsed }}ms
            </td>
            <td class="border border-black p-2 text-center">
              <v-button
                label="Kill"
                @click="killOp(o.id)"
              />
            </td>
          </tr>
        </table>
      </div>
    </v-collapse>
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
