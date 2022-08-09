<!-- eslint-disable require-atomic-updates -->
<script>

import * as THREE from 'three';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls';
import { toast } from './lib/toast';
import robotApi from './gen/proto/api/robot/v1/robot_pb.esm';
import commonApi from './gen/proto/api/common/v1/common_pb.esm';
import armApi from './gen/proto/api/component/arm/v1/arm_pb.esm';
import baseApi from './gen/proto/api/component/base/v1/base_pb.esm';
import cameraApi from './gen/proto/api/component/camera/v1/camera_pb.esm';
import gantryApi from './gen/proto/api/component/gantry/v1/gantry_pb.esm';
import gripperApi from './gen/proto/api/component/gripper/v1/gripper_pb.esm';
import imuApi from './gen/proto/api/component/imu/v1/imu_pb.esm';
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
  BaseControlHelper,
  MotorControlHelper,
  BoardControlHelper,
  ServoControlHelper,
  computeKeyboardBaseControls,
} from './rc/control_helpers';

import BaseComponent from './components/base.vue';
import Camera from './components/camera.vue';
import Do from './components/do.vue';
import Gamepad from './components/gamepad.vue';
import InputController from './components/input-controller.vue';
import MotorDetail from './components/motor-detail.vue';
import Navigation from './components/navigation.vue';
import ServoComponent from './components/servo.vue';
import Slam from './components/slam.vue';

function roundTo2Decimals(num) {
  return Math.round(num * 100) / 100;
}

function fixArmStatus(old) {
  const newStatus = {
    pos_pieces: [],
    joint_pieces: [],
    is_moving: old.is_moving || false,
  };

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
    newStatus.pos_pieces.push(
      { 
        endPosition: fieldSetter,
        endPositionValue: old.end_position[endPositionField] || 0,
      }
    );
  }

  for (let j = 0; j < old.joint_positions.values.length; j++) {
    newStatus.joint_pieces.push(
      { 
        joint: j,
        jointValue: old.joint_positions.values[j] || 0,
      }
    );
  }

  return newStatus;
}

function fixBoardStatus(old) {
  return {
    analogsMap: old.analogs || [],
    digitalInterruptsMap: old.digital_interrupts || [],
  };
}

function fixGantryStatus(old) {
  const newStatus = {
    parts: [],
    is_moving: old.is_moving || false,
  };

  if (old.lengths_mm.length !== old.positions_mm.length) {
    throw 'gantry lists different lengths';
  }

  for (let i = 0; i < old.lengths_mm.length; i++) {
    newStatus.parts.push({
      axis: i,
      pos: old.positions_mm[i],
      length: old.lengths_mm[i],
    });
  }

  return newStatus;
}

function fixInputStatus(old) {
  const events = old.events || [];
  const eventsList = events.map((event) => {
    return {
      time: event.time || {},
      event: event.event || '',
      control: event.control || '',
      value: event.value || 0,
    };
  });
  return { eventsList };
}

function fixMotorStatus(old) {
  return {
    isPowered: old.is_powered || false,
    positionReporting: old.position_reporting || false,
    position: old.position || 0,
    isMoving: old.is_moving || false,
  };
}

function fixServoStatus(old) {
  return { positionDeg: old.position_deg || 0, is_moving: old.is_moving || false };
}

export default {
  components: {
    BaseComponent,
    Camera,
    Do,
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
      streamNames: [],
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
      imuData: {},
      currentOps: [],
      setPin: '',
      getPin: '',
      pwm: '',
      pwmFrequency: '',
      imageMapTemp: '',
    };
  },
  async mounted() {
    this.grpcCallback = this.grpcCallback.bind(this);
    await this.waitForClientAndStart();

    if (window.streamService) {
      this.queryStreams();
    }

    this.imuRefresh();
    await this.queryMetadata();
  },
  methods: {
    filterResources,
    filterRdkComponentsWithStatus,
    resourceNameToString,
    filterResourcesWithNames,
    
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
      const visionName = filterResources(this.resources, 'rdk', 'services', 'vision')[0];
      
      req.setName(visionName)

      visionService.getSegmenterNames(req, {}, (err, resp) => {
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
      const visionName = filterResources(this.resources, 'rdk', 'services', 'vision')[0];

      req.setName(visionName)
      req.setSegmenterName(name);
      
      visionService.getSegmenterParameters(req, {}, (err, resp) => {
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
      gantryService.moveToPosition(req, {}, this.grpcCallback);
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
      armService.moveToPosition(req, {}, this.grpcCallback);
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
      armService.moveToJointPositions(req, {}, this.grpcCallback);
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
      armService.moveToJointPositions(req, {}, this.grpcCallback);
    },
    armModifyAll(name) {
      const arm = this.resourceStatusByName(name);
      const n = { pos_pieces: [], joint_pieces: [] };
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
      armService.moveToJointPositions(req, {}, this.grpcCallback);
      delete this.armToggle[name.name];
    },

    gripperAction(name, action) {
      let req;
      switch (action) {
      case 'open':
        req = new gripperApi.OpenRequest();
        req.setName(name);
        gripperService.open(req, {}, this.grpcCallback);
        break;
      case 'grab':
        req = new gripperApi.GrabRequest();
        req.setName(name);
        gripperService.grab(req, {}, this.grpcCallback);
        break;
      }
    },
    servoMove(name, amount) {
      const servo = this.rawResourceStatusByName(name);
      const oldAngle = servo.position_deg || 0;
      const angle = oldAngle + amount;
      const req = new servoApi.MoveRequest();
      req.setName(name.name);
      req.setAngleDeg(angle);
      servoService.move(req, {}, this.grpcCallback);
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
      inputControllerService.triggerEvent(req, {}, this.grpcCallback);
    },
    killOp(id) {
      const req = new robotApi.KillOperationRequest();
      req.setId(id);
      window.robotService.killOperation(req, {}, this.grpcCallback);
    },
    baseKeyboardCtl(name, controls) {
      if (Object.values(controls).every((item) => item === false)) {
        console.log('All keyboard inputs false, stopping base.');
        this.handleBaseActionStop(name);
        return;
      } 

      const inputs = computeKeyboardBaseControls(controls);
      const linear = new commonApi.Vector3();
      const angular = new commonApi.Vector3();
      linear.setY(inputs.linear);
      angular.setZ(inputs.angular);
      BaseControlHelper.setPower(name, linear, angular, this.grpcCallback);
    },
    handleBaseActionStop(name) {
      const req = new baseApi.StopRequest();
      req.setName(name);
      baseService.stop(req, {}, this.grpcCallback);
    },
    handleBaseSpin(name, event) {
      BaseControlHelper.spin(name, 
        event.angle * event.direction,
        event.speed,
        this.grpcCallback
      );
    },
    handleBaseStraight(name, event) {
      if (event.movementType === 'Continuous') {
        const linear = new commonApi.Vector3();
        linear.setY(event.speed * event.direction);

        BaseControlHelper.setVelocity(
          name, 
          linear, // linear
          new commonApi.Vector3(), // angular
          this.grpcCallback
        );
      } else {
        BaseControlHelper.moveStraight(name,
          event.distance, 
          event.speed * event.direction, 
          this.grpcCallback
        );
      }
    },
    renderFrame(cameraName) {
      const req = new cameraApi.RenderFrameRequest();
      req.setName(cameraName);
      const mimeType = 'image/jpeg';
      req.setMimeType(mimeType);
      cameraService.renderFrame(req, {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        const blob = new Blob([resp.getData_asU8()], { type: mimeType });
        window.open(URL.createObjectURL(blob), '_blank');
      });
    },
    viewCameraFrame(time) {
      clearInterval(this.cameraFrameIntervalId);
      const cameraName = this.streamNames[0];
      if (time === 'manual') {
        this.viewManualFrame(cameraName);
      } else if (time === 'live') {
        this.viewCamera(cameraName);
      } else {
        this.viewIntervalFrame(cameraName, time);
      }
    },
    viewManualFrame(cameraName) {
      const req = new cameraApi.RenderFrameRequest();
      req.setName(cameraName);
      const mimeType = 'image/jpeg';
      req.setMimeType(mimeType);
      cameraService.renderFrame(req, {}, (err, resp) => {
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
        cameraService.renderFrame(req, {}, (err, resp) => {
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
        cameraService.getPointCloud(req, {}, (err, resp) => {
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
    updateSLAMImageRefreshFrequency(time) {
      clearInterval(this.slamImageIntervalId);
      if (time === 'manual') {
        this.viewSLAMImageMap();
      } else if (time === 'off') {
        // do nothing
      } else {
        this.viewSLAMImageMap();
        this.slamImageIntervalId = window.setInterval(() => {
          this.viewSLAMImageMap();
        }, Number(time) * 1000);
      }
    },
    viewSLAMImageMap() {
      
      const req = new slamApi.GetMapRequest();
      const slamName = filterResources(this.resources, 'rdk', 'services', 'slam')[0];
      req.setName(slamName);
      req.setMimeType('image/jpeg');
      req.setIncludeRobotMarker(true);
      slamService.getMap(req, {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        const blob = new Blob([resp.getImage_asU8()], { type: 'image/jpeg' });
        this.imageMapTemp = URL.createObjectURL(blob);
      });
    },
    updateSLAMPCDRefreshFrequency(time, load) {
      clearInterval(this.slamPCDIntervalId);
      if (time === 'manual') {
        this.viewSLAMPCDMap(load);
      } else if (time === 'off') {
        // do nothing
      } else {
        this.viewSLAMPCDMap(load);
        this.slamPCDIntervalId = window.setInterval(() => {
          this.viewSLAMPCDMap();
        }, Number(time) * 1000);
      }
    },
    viewSLAMPCDMap(load) {
      this.$nextTick(() => {
        const req = new slamApi.GetMapRequest();
        const slamName = filterResources(this.resources, 'rdk', 'services', 'slam')[0];
        req.setName(slamName);
        req.setMimeType('pointcloud/pcd');
        if (load) {
          this.initPCD();
        }
        slamService.getMap(req, {}, (err, resp) => {
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
      const sensorsName = filterResources(this.resources, 'rdk', 'service','sensors')[0];
      const names = sensorNames.map((name) => {
        const resourceName = new commonApi.ResourceName();
        resourceName.setNamespace(name.namespace);
        resourceName.setType(name.type);
        resourceName.setSubtype(name.subtype);
        resourceName.setName(name.name);
        return resourceName;
      });
      req.setName(sensorsName.name)
      req.setSensorNamesList(names);
      sensorsService.getReadings(req, {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        for (const r of resp.getReadingsList()) {
          const readings = r.getReadingsList().map((v) => v.toJavaScript());
          this.sensorReadings[resourceNameToString(r.getName().toObject())] = readings;
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

      motionService.move(req, {}, (err, resp) => {
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
      const visionName = filterResources(this.resources, 'rdk', 'services', 'vision')[0];
      
      req.setName(visionName);
      req.setCameraName(pcdGlobal.cameraName);
      req.setSegmenterName(segmenterName);
      req.setParameters(proto.google.protobuf.Struct.fromJavaScript(segmenterParams));
      const mimeType = 'pointcloud/pcd';
      req.setMimeType(mimeType);
      console.log('finding object segments...');
      visionService.getObjectPointClouds(req, {}, (err, resp) => {
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
    viewCamera(name) {
      const streamName = normalizeRemoteName(name);
      const streamContainer = document.querySelector(`#stream-${streamName}`);
      const req = new streamApi.AddStreamRequest();
      req.setName(name);
      streamService.addStream(req, {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (streamContainer && streamContainer.querySelectorAll('img').length > 0) {
          streamContainer.querySelectorAll('img')[0].remove();
        }
        if (err) {
          this.error = 'no live camera device found';
          
        }
      });
    },
    viewPreviewCamera(name) {
      const req = new streamApi.AddStreamRequest();
      req.setName(name);
      streamService.addStream(req, {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          this.error = 'no live camera device found';
          
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
    async queryMetadata () {
      let pResolve;
      let pReject;
      const p = new Promise((resolve, reject) => {
        pResolve = resolve;
        pReject = reject;
      });
      let resourcesChanged = false;
      let shouldRestartStatusStream = false;

      window.robotService.resourceNames(new robotApi.ResourceNamesRequest(), {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          pReject(err);
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
        pResolve(null);
      });
      await p;

      if (resourcesChanged === true) {
        this.querySensors();

        if (shouldRestartStatusStream === true) {
          this.restartStatusStream();
        }
      }
      setTimeout(() => this.queryMetadata(), 500);
    },
    querySensors() {
      const sensorsName = filterResources(this.resources, 'rdk', 'service','sensors')[0];
      const req = new sensorsApi.GetSensorsRequest();
      req.setName(sensorsName.name)
      sensorsService.getSensors(req, {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        this.sensorNames = resp.toObject().sensorNamesList;
      });
    },
    loadCurrentOps () {
      window.clearTimeout(this.currentOpsTimerId);
      const req = new robotApi.GetOperationsRequest();
      window.robotService.getOperations(req, {}, (err, resp) => {
        const lst = resp.toObject().operationsList;
        this.currentOps = lst;

        const now = Date.now();
        for (const op of this.currentOps) {
          op.elapsed = now - (op.started.seconds * 1000);
        }

        this.currentOpsTimerId = window.setTimeout(this.loadCurrentOps, 1000);
      });
    },
    async doConnect(authEntity, creds, onError) {
      console.debug('connecting');
      document.querySelector('#connecting').classList.remove('hidden');
      try {
        await window.connect(authEntity, creds);
        this.loadCurrentOps();
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
        for (elem of authElems) {
          elem.disabled = true;
        }
      };
      const enableAll = () => {
        for (elem of authElems) {
          elem.disabled = false;
        }
      };
      for (authType of window.supportedAuthTypes) {
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
    queryStreams () {
      streamService.listStreams(new streamApi.ListStreamsRequest(), {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (!err) {
          const streamNames = resp.toObject().namesList;
          this.streamNames = streamNames;
        }
        setTimeout(() => this.queryStreams(), 500);
      });
    },
    initPCDIfNeeded() {
      if (pcdGlobal) {
        return;
      }
      initPCD();
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
    imuRefresh() {
      for (const x of filterResources(this.resources, 'rdk', 'component', 'imu')) {
        const name = x.name;

        if (!this.imuData[name]) {
          this.imuData[name] = {};
        }

        {
          const req = new imuApi.ReadOrientationRequest();
          req.setName(name);

          imuService.readOrientation(req, {}, (err, resp) => {
            if (err) {
              console.log(err);
              return;
            }
            this.imuData[name].orientation = resp.toObject().orientation;
          });
        }

        {
          const req = new imuApi.ReadAngularVelocityRequest();
          req.setName(name);

          imuService.readAngularVelocity(req, {}, (err, resp) => {
            if (err) {
              console.log(err);
              return;
            }
            this.imuData[name].angularVelocity = resp.toObject().angularVelocity;
          });
        }

        {
          const req = new imuApi.ReadAccelerationRequest();
          req.setName(name);

          imuService.readAcceleration(req, {}, (err, resp) => {
            if (err) {
              console.log(err);
              return;
            }
            this.imuData[name].acceleration = resp.toObject().acceleration;
          });
        }

        {
          const req = new imuApi.ReadMagnetometerRequest();
          req.setName(name);

          imuService.readMagnetometer(req, {}, (err, resp) => {
            if (err) {
              console.log(err);
              return;
            }
            this.imuData[name].magnetometer = resp.toObject().magnetometer;
          });
        }
      }

      setTimeout(this.imuRefresh, 500);
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
      style="color: red;"
    >
      {{ error }}
    </div>

    <!-- ******* BASE *******  -->
    <div
      v-for="base in filterResources(resources, 'rdk', 'component', 'base')"
      :key="base.name"
      class="base"
    >
      <div v-if="streamNames.length === 0">
        <div class="camera">
          <BaseComponent
            :base-name="base.name"
            :connected-camera="false"
            :crumbs="['base', base.name]"
            @keyboard-ctl="baseKeyboardCtl(base.name, $event)"
            @base-spin="handleBaseSpin(base.name, $event)"
            @base-straight="handleBaseStraight(base.name, $event)"
            @base-stop="handleBaseActionStop(base.name)"
          />
        </div>
      </div>
      <div v-else>
        <div
          v-for="streamName in streamNames"
          :key="streamName"
          class="camera"
        >
          <BaseComponent
            :base-name="base.name"
            :stream-name="streamName"
            :crumbs="['base', base.name]"
            :connected-camera="true"
            @base-change-tab="viewPreviewCamera(streamName)"
            @keyboard-ctl="baseKeyboardCtl(base.name, $event)"
            @base-spin="handleBaseSpin(base.name, $event)"
            @base-straight="handleBaseStraight(base.name, $event)"
            @base-stop="handleBaseActionStop(base.name)"
            @show-base-camera="viewPreviewCamera(streamName)"
          />
        </div>
      </div>
    </div>

    <!-- ******* GANTRY *******  -->
    <v-collapse
      v-for="gantry in filterRdkComponentsWithStatus(resources, status, 'gantry')"
      :key="gantry.name"
      :title="`Gantry ${gantry.name}`"
    >
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

    <!-- ******* IMU *******  -->
    <v-collapse
      v-for="imu in filterResources(resources, 'rdk', 'component', 'imu')"
      :key="imu.name"
      :title="`IMU: ${imu.name}`"
    >
      <div class="flex items-end border border-t-0 border-black p-4">
        <template v-if="imuData[imu.name] && imuData[imu.name].angularVelocity">
          <div class="mr-4 w-1/4">
            <h3 class="mb-1">
              Orientation (degrees)
            </h3>
            <table class="w-full border border-t-0 border-black p-4">
              <tr>
                <th class="border border-black p-2">
                  Roll
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].orientation?.rollDeg.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Pitch
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].orientation?.pitchDeg.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Yaw
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].orientation?.yawDeg.toFixed(2) }}
                </td>
              </tr>
            </table>
          </div>
                
          <div class="mr-4 w-1/4">
            <h3 class="mb-1">
              Angular Velocity (degrees/second)
            </h3>
            <table class="w-full border border-t-0 border-black p-4">
              <tr>
                <th class="border border-black p-2">
                  X
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].angularVelocity?.xDegsPerSec.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Y
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].angularVelocity?.yDegsPerSec.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Z
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].angularVelocity?.zDegsPerSec.toFixed(2) }}
                </td>
              </tr>
            </table>
          </div>
                
          <div class="mr-4 w-1/4">
            <h3 class="mb-1">
              Acceleration (mm/second/second)
            </h3>
            <table class="w-full border border-t-0 border-black p-4">
              <tr>
                <th class="border border-black p-2">
                  X
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].acceleration?.xMmPerSecPerSec.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Y
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].acceleration?.yMmPerSecPerSec.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Z
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].acceleration?.zMmPerSecPerSec.toFixed(2) }}
                </td>
              </tr>
            </table>
          </div>
                
          <div class="w-1/4">
            <h3 class="mb-1">
              Magnetometer (gauss)
            </h3>
            <table class="w-full border border-t-0 border-black p-4">
              <tr>
                <th class="border border-black p-2">
                  X
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].magnetometer?.xGauss.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Y
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].magnetometer?.yGauss.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Z
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu.name].magnetometer?.zGauss.toFixed(2) }}
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
      :title="`Arm ${arm.name}`"
    >
      <div class="mb-2 flex">
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
      :title="`Gripper ${gripper.name}`"
    >
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
      @servo-move="(amount) => servoMove(servo, amount)"
      @servo-stop="servoStop(servo.name)"
    />

    <!-- ******* MOTOR *******  -->
    <MotorDetail
      v-for="motor in filterRdkComponentsWithStatus(resources, status, 'motor')"
      :key="'new-' + motor.name" 
      :motor-name="motor.name" 
      :crumbs="['motor', motor.name]" 
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
      :title="`Board ${board.name}`"
    >
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

    <!-- sensors -->
    <v-collapse
      v-if="nonEmpty(sensorNames)"
      title="Sensors"
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
              {{ sensorReadings[resourceNameToString(name)] }}
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

    <!-- get segments -->
    <Navigation
      v-for = "nav in filterResources(resources, 'rdk', 'service', 'navigation')"
      :resources="nav.resources"
      :name = "nav.name"
    />

    <!-- current operations -->
    <v-collapse title="Current Operations">
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

    <!-- ******* CAMERAS *******  -->
    <Camera
      v-for="streamName in streamNames"
      :key="streamName"
      :stream-name="streamName"
      :crumbs="[streamName]"
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
      @toggle-camera="viewCamera(streamName)"
      @refresh-camera="viewCameraFrame"
      @selected-camera-view="viewCameraFrame"
      @toggle-pcd="renderPCD(streamName)"
      @pcd-click="grabClick"
      @pcd-move="doPCDMove"
      @point-load="doPointLoad"
      @segment-load="doSegmentLoad"
      @bounding-box-load="doBoundingBoxLoad"
      @download-screenshot="renderFrame(streamName)"
      @download-raw-data="doPCDDownload(fullcloud)"
      @select-object="doSelectObject"
      @segmenter-parameters-input="(name, value) => segmenterParameters[name] = Number(value)"
    />

    <!-- ******* SLAM *******  -->
    <Slam
      v-if="filterResources(resources, 'rdk', 'service', 'slam').length > 0"
      :image-map="imageMapTemp"
      @update-slam-image-refresh-frequency="updateSLAMImageRefreshFrequency"
      @update-slam-pcd-refresh-frequency="updateSLAMPCDRefreshFrequency"
    />

    <!-- ******* DO ******* -->
    <Do :resources="filterResourcesWithNames(resources)" />
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
