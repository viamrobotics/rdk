<script>

// import ViamBase from './components/Base.vue'
import robotApi from './gen/proto/api/robot/v1/robot_pb.esm';
import commonApi from './gen/proto/api/common/v1/common_pb.esm';
import armApi from './gen/proto/api/component/arm/v1/arm_pb.esm';
import { RobotServiceClient } from './gen/proto/api/robot/v1/robot_pb_service.esm';
import { ArmServiceClient } from './gen/proto/api/component/arm/v1/arm_pb_service.esm';
import baseApi from './gen/proto/api/component/base/v1/base_pb.esm';
import { BaseServiceClient } from './gen/proto/api/component/base/v1/base_pb_service.esm';
import { BoardServiceClient } from './gen/proto/api/component/board/v1/board_pb_service.esm';
import cameraApi from './gen/proto/api/component/camera/v1/camera_pb.esm';
import { CameraServiceClient } from './gen/proto/api/component/camera/v1/camera_pb_service.esm';
import gantryApi from './gen/proto/api/component/gantry/v1/gantry_pb.esm';
import { GantryServiceClient } from './gen/proto/api/component/gantry/v1/gantry_pb_service.esm';
import gripperApi from './gen/proto/api/component/gripper/v1/gripper_pb.esm';
import { GripperServiceClient } from './gen/proto/api/component/gripper/v1/gripper_pb_service.esm';
import imuApi from './gen/proto/api/component/imu/v1/imu_pb.esm';
import { IMUServiceClient } from './gen/proto/api/component/imu/v1/imu_pb_service.esm';
import { InputControllerServiceClient } from './gen/proto/api/component/inputcontroller/v1/input_controller_pb_service.esm';
import { MotorServiceClient } from './gen/proto/api/component/motor/v1/motor_pb_service.esm';
import navigationApi from './gen/proto/api/service/navigation/v1/navigation_pb.esm';
import { NavigationServiceClient } from './gen/proto/api/service/navigation/v1/navigation_pb_service.esm';
import motionApi from './gen/proto/api/service/motion/v1/motion_pb.esm';
import { MotionServiceClient } from './gen/proto/api/service/motion/v1/motion_pb_service.esm';
import visionApi from './gen/proto/api/service/vision/v1/vision_pb.esm';
import { VisionServiceClient } from './gen/proto/api/service/vision/v1/vision_pb_service.esm';
import sensorsApi from './gen/proto/api/service/sensors/v1/sensors_pb.esm';
import { SensorsServiceClient } from './gen/proto/api/service/sensors/v1/sensors_pb_service.esm';
import servoApi from './gen/proto/api/component/servo/v1/servo_pb.esm';
import { ServoServiceClient } from './gen/proto/api/component/servo/v1/servo_pb_service.esm';
import slamApi from './gen/proto/api/service/slam/v1/slam_pb.esm';
import { SLAMServiceClient } from './gen/proto/api/service/slam/v1/slam_pb_service.esm';
import streamApi from './gen/proto/stream/v1/stream_pb.esm';
import { StreamServiceClient } from './gen/proto/stream/v1/stream_pb_service.esm';
import { dialDirect, dialWebRTC } from '@viamrobotics/rpc';
import * as THREE from 'three';
import { PCDLoader } from 'three/examples/jsm/loaders/PCDLoader';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls';
import {
  BaseControlHelper,
  MotorControlHelper,
  BoardControlHelper,
  computeKeyboardBaseControls,
} from './rc/control_helpers';

const { webrtcHost } = window;

const rtcConfig = {
	iceServers: [
		{
			urls: 'stun:global.stun.twilio.com:3478?transport=udp',
		},
	],
};

if (window.webrtcAdditionalICEServers) {
	rtcConfig.iceServers = rtcConfig.iceServers.concat(window.webrtcAdditionalICEServers);
}

const connect = async (authEntity, creds) => {
	let transportFactory;
	const opts = { 
		authEntity,
		credentials: creds,
		webrtcOptions: { rtcConfig },
	};
	const impliedURL = `${location.protocol}//${location.hostname}${location.port ? `:${ location.port}` : ''}`;
	if (window.webrtcEnabled) {
		if (!window.webrtcSignalingAddress) {
			window.webrtcSignalingAddress = impliedURL;
		}
		opts.webrtcOptions.signalingAuthEntity = opts.authEntity;
		opts.webrtcOptions.signalingCredentials = opts.credentials;

		const webRTCConn = await dialWebRTC(window.webrtcSignalingAddress, window.webrtcHost, opts);
		transportFactory = webRTCConn.transportFactory;
		window.streamService = new StreamServiceClient(window.webrtcHost, { transport: transportFactory });
		
		// eslint-disable-next-line require-await
		webRTCConn.peerConnection.ontrack = async event => {
			const video = document.createElement('video');
			video.srcObject = event.streams[0];
			video.autoplay = true;
			video.controls = false;
			video.playsInline = true;
			const streamName = event.streams[0].id;
			const streamContainer = document.getElementById(`stream-${streamName}`);
			if (streamContainer && streamContainer.querySelectorAll('video').length > 0) {
				streamContainer.querySelectorAll('video')[0].remove();
			}
			if (streamContainer) {
				streamContainer.append(video);
			}
			const videoPreview = document.createElement('video');
			videoPreview.srcObject = event.streams[0];
			videoPreview.autoplay = true;
			videoPreview.controls = false;
			videoPreview.playsInline = true;
			const streamPreviewContainer = document.getElementById(`stream-preview-${streamName}`);
			if (streamPreviewContainer && streamPreviewContainer.querySelectorAll('video').length > 0) {
				streamPreviewContainer.querySelectorAll('video')[0].remove();
			}
			if (streamPreviewContainer) {
				streamPreviewContainer.append(videoPreview);
			}
		};
	} else {
		transportFactory = await dialDirect(impliedURL, opts);
	}

  window.robotService = new RobotServiceClient(webrtcHost, { transport: transportFactory });
  // TODO(RSDK-144): these should be created as needed
  window.armService = new ArmServiceClient(webrtcHost, { transport: transportFactory });
  window.baseService = new BaseServiceClient(webrtcHost, { transport: transportFactory });
  window.boardService = new BoardServiceClient(webrtcHost, { transport: transportFactory });
  window.cameraService = new CameraServiceClient(webrtcHost, { transport: transportFactory });
  window.gantryService = new GantryServiceClient(webrtcHost, { transport: transportFactory });
  window.gripperService = new GripperServiceClient(webrtcHost, { transport: transportFactory });
  window.imuService = new IMUServiceClient(webrtcHost, { transport: transportFactory });
  window.inputControllerService = new InputControllerServiceClient(webrtcHost, { transport: transportFactory });
  window.motorService = new MotorServiceClient(webrtcHost, { transport: transportFactory });
  window.navigationService = new NavigationServiceClient(webrtcHost, { transport: transportFactory });
  window.motionService = new MotionServiceClient(webrtcHost, { transport: transportFactory });
  window.visionService = new VisionServiceClient(webrtcHost, { transport: transportFactory });
  window.sensorsService = new SensorsServiceClient(webrtcHost, { transport: transportFactory });
  window.servoService = new ServoServiceClient(webrtcHost, { transport: transportFactory });
  window.slamService = new SLAMServiceClient(webrtcHost, { transport: transportFactory });

	// save authEntity, creds
	window.connect = () => connect(authEntity, creds);
};
window.connect = connect;

window.rcDebug = false;

window.rcLogConditionally = function (req) {
	if (rcDebug) {
		console.log('gRPC call:', req);
	}
};

function roundTo2Decimals(num) {
  return Math.round(num * 100) / 100;
}

function fixArmStatus(old) {
  const newStatus = { pos_pieces : [], joint_pieces : [], is_moving: old['is_moving'] || false };
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
        endPosition : fieldSetter,
        endPositionValue : old['end_position'][endPositionField] || 0,
      }
    );
  }

  for (let j = 0; j < old['joint_positions']['degrees'].length; j++ ){
    newStatus.joint_pieces.push(
      { 
        joint : j,
        jointValue : old['joint_positions']['degrees'][j] || 0,
      }
    );
  }

  return newStatus;
}

function fixBoardStatus(old) {
  return {
    analogsMap: old['analogs'] || [],
    digitalInterruptsMap: old['digital_interrupts'] || [],
  };
}

function fixGantryStatus(old) {
  const newStatus = {
    parts: [],
    is_moving: old['is_moving'] || false,
  };

  if (old['lengths_mm'].length !== old['positions_mm'].length) {
    throw 'gantry lists different lengths';
  }

  for (let i = 0; i < old['lengths_mm'].length; i++) {
    newStatus.parts.push({ axis: i, pos: old['positions_mm'][i], length: old['lengths_mm'][i] });
  }

  return newStatus;
}

function fixInputStatus(old) {
  const events = old['events'] || [];
  const eventsList = events.map((e) => {
    return {
      time: e['time'] || {},
      event: e['event'] || '',
      control: e['control'] || '',
      value: e['value'] || 0,
    };
  });
  return { eventsList };
}

function fixMotorStatus(old) {
  return {
    isPowered: old['is_powered'] || false,
    positionReporting: old['position_reporting'] || false,
    position: old['position'] || 0,
    isMoving: old['is_moving'] || false,
  };
}

function fixServoStatus(old) {
  return { positionDeg: old['position_deg'] || 0, is_moving: old['is_moving'] || false };
}

export default {
  directives: {
    // TODO(APP-82): replace with vue component after naveed work done
    mapMounted() {
      if (this.mapOnce) {
        return;
      }
      this.mapOnce = true;
      this.initNavigation();
    },
  },
  components: {
    // ViamBase,
  },
  data() {
    return {
      error: '',
      res: {},
      rawStatus: {},
      status: {},
      pcdClick: {},
      sensorReadings: {},
      resources: [],
      sensorNames: [],
      streamNames: [],
      intervalId: null,
      segmenterNames: [],
      segmenterParameterNames: [],
      segmenterParameters: {},
      segmentAlgo: '',
      fullcloud: null,
      objects: null,
      minPtsPlane: 10_000,
      minPtsSegment: 100,
      clusterRad: 5,
      armToggle: {},
      mapOnce: false,
      value: 0,
      imuData: {},
      currentOps: [],
      setPin: '',
      getPin: '',
      locationValue: '40.745297,-74.010916',
      imageMapTemp: '',
      pcdMapTemp: null,
    };
  },
  async mounted() {
    this.grpcCallback = this.grpcCallback.bind(this);
    await this.waitForClientAndStart();

    if (window.streamService) {
      this.queryStreams();
    }

    this.imuRefresh();
    this.queryMetadata();
  },
  methods: {
    fixRawStatus(name, status) {
      switch (this.resourceNameToSubtypeString(name)) {
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
      req.setSegmenterName(name);
      visionService.getSegmenterParameters(req, {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          console.log(`error getting segmenter parameters for ${ name}`);
          console.log(err);
          return;
        }
        this.segmenterParameterNames = resp.getSegmenterParametersList();
        this.segmenterParameters = {};
      });
    },
    filterResources(namespace, type, subtype) {
      return this.resources.filter((elem) => {
        return elem.namespace === namespace && elem.type === type && elem.subtype === subtype;
      }).sort((a, b) => {
        if (a.name < b.name) {
          return -1;
        }
        if (a.name > b.name) {
          return 1;
        }
        return 0;
      });
    },
    resourceNameToSubtypeString(name) {
      if (!name) {
        return '';
      }

      return `${name.namespace}:${name.type}:${name.subtype}`;
    },
    resourceNameToString(name) {
      if (!name) {
        return '';
      }

      let strName = this.resourceNameToSubtypeString(name);
      if (name.name !== '') {
        strName += `/${name.name}`;
      }
      return strName;
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
      return this.status[this.resourceNameToString(name)];
    },
    rawResourceStatusByName(name) {
      return this.rawStatus[this.resourceNameToString(name)];
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
      const old = arm['end_position'];
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
      const newList = arm['joint_positions']['degrees'];
      newList[field] += amount;
      newPositionDegs.setDegreesList(newList);
      const req = new armApi.MoveToJointPositionsRequest();
      req.setName(name.name);
      req.setPositionDegs(newPositionDegs);
      armService.moveToJointPositions(req, {}, this.grpcCallback);
    },
    armHome(name) {
      const arm = this.rawResourceStatusByName(name);
      const newPositionDegs = new armApi.JointPositions();
      const newList = arm['joint_positions']['degrees'];
      for (let i = 0; i < newList.length; i++) {
        newList[i] = 0;
      }
      newPositionDegs.setDegreesList(newList);
      const req = new armApi.MoveToJointPositionsRequest();
      req.setName(name.name);
      req.setPositionDegs(newPositionDegs);
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
      const newList = arm['joint_positions']['degrees'];
      const newPieces = this.armToggle[name.name].joint_pieces;
      for (let i = 0; i < newPieces.length && i < newList.length; i++) {
        newList[newPieces[i].joint] = newPieces[i].jointValue;
      }

      newPositionDegs.setDegreesList(newList);
      const req = new armApi.MoveToJointPositionsRequest();
      req.setName(name.name);
      req.setPositionDegs(newPositionDegs);
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
      const oldAngle = servo['position_deg'] || 0;
      const angle = oldAngle + amount;
      const req = new servoApi.MoveRequest();
      req.setName(name.name);
      req.setAngleDeg(angle);
      servoService.move(req, {}, this.grpcCallback);
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
        elem.name !== 'WebGamepad'
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
    baseKeyboardCtl: function(name, controls) {
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
      req = new cameraApi.RenderFrameRequest();
      req.setName(cameraName);
      const mimeType = 'image/jpeg';
      req.setMimeType(mimeType);
      const { grpcCallback } = this;
      cameraService.renderFrame(req, {}, (err, resp) => {
        grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        const blob = new Blob([resp.getData_asU8()], { type: mimeType });
        window.open(URL.createObjectURL(blob), '_blank');
      });
    },
    viewCameraFrame(time) {
        clearInterval(this.intervalId);
        const cameraName = this.streamNames[0];
        if (time === 'manual' ) {
            this.viewManualFrame(cameraName);
        } else if (time === 'live') {
            this.viewCamera(cameraName);
        } else {
            this.viewIntervalFrame(cameraName, time);
        }
    },
    viewManualFrame(cameraName) {
      req = new cameraApi.RenderFrameRequest();
      req.setName(cameraName);
      const mimeType = 'image/jpeg';
      req.setMimeType(mimeType);
      const { grpcCallback } = this;
      cameraService.renderFrame(req, {}, (err, resp) => {
        grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        const streamContainer = document.getElementById(`stream-${cameraName}`);
        if (streamContainer && streamContainer.getElementsByTagName('video').length > 0) {
            streamContainer.getElementsByTagName('video')[0].remove();
        }
        if (streamContainer && streamContainer.getElementsByTagName('img').length > 0) {
            streamContainer.getElementsByTagName('img')[0].remove();
        }
        const image = new Image();
        const blob = new Blob([resp.getData_asU8()], { type: mimeType });
        image.src = URL.createObjectURL(blob);
        streamContainer.append(image);
      });
    },
    viewIntervalFrame(cameraName, time) {
      const { grpcCallback } = this;
        this.intervalId = setInterval(() => {
          req = new cameraApi.RenderFrameRequest();
          req.setName(cameraName);
          const mimeType = 'image/jpeg';
          req.setMimeType(mimeType);
          cameraService.renderFrame(req, {}, (err, resp) => {
            grpcCallback(err, resp, false);
            if (err) {
              return;
            }
            const streamContainer = document.getElementById(`stream-${cameraName}`);
            if (streamContainer && streamContainer.getElementsByTagName('video').length > 0) {
                streamContainer.getElementsByTagName('video')[0].remove();
            }
            if (streamContainer && streamContainer.getElementsByTagName('img').length > 0) {
                streamContainer.getElementsByTagName('img')[0].remove();
            }
            const image = new Image();
            const blob = new Blob([resp.getData_asU8()], { type: mimeType });
            image.src = URL.createObjectURL(blob);
            streamContainer.append(image);
          });
        }, +time * 1000);
    },
    renderPCD(cameraName) {
      this.$nextTick(() => {
        this.pcdClick.pcdloaded = false;
        this.pcdClick.foundSegments = false;
        this.initPCDIfNeeded();
        pcdGlobal.cameraName = cameraName;

        req = new cameraApi.GetPointCloudRequest();
        req.setName(cameraName);
        const mimeType = 'pointcloud/pcd';
        req.setMimeType(mimeType);
        cameraService.getPointCloud(req, {}, (err, resp) => {
          this.grpcCallback(err, resp, false);
          if (err) {
            return;
          }
          console.log('loading pcd');
          this.fullcloud = resp.getPointCloud_asB64();
          pcdLoad(`data:${mimeType};base64,${this.fullcloud}`);
        });
      });
    },
    viewSLAMImageMap() {
      req = new slamApi.GetMapRequest();
      req.setName('UI');
      const mimeType = 'image/jpeg';
      req.setMimeType(mimeType);
      slamService.getMap(req, {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        console.log('loading image map');
        const blob = new Blob([resp.getImage_asU8()], { type: mimeType });
        this.imageMapTemp = URL.createObjectURL(blob);
      });
    },
    viewSLAMPCDMap() {
      req = new slamApi.GetMapRequest();
      req.setName('UI');
      const mimeType = 'pointcloud/pcd';
      req.setMimeType(mimeType);
      this.initPCDIfNeeded();
      slamService.getMap(req, {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        console.log('loading pcd map');
        pcObject = resp.getPointCloud();
        this.fullcloud = pcObject.getPointCloud_asB64();
        pcdLoad(`data:${mimeType};base64,${this.fullcloud}`);
      });
    },
    getReadings(sensorNames) {
      const { grpcCallback } = this;
      const req = new sensorsApi.GetReadingsRequest();
      const names = sensorNames.map((name) => {
        const resourceName = new commonApi.ResourceName();
        resourceName.setNamespace(name.namespace);
        resourceName.setType(name.type);
        resourceName.setSubtype(name.subtype);
        resourceName.setName(name.name);
        return resourceName;
      });
      req.setSensorNamesList(names);
      sensorsService.getReadings(req, {}, (err, resp) => {
        grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        for (const r of resp.getReadingsList()) {
          const readings = r.getReadingsList().map((v) => v.toJavaScript());
          this.sensorReadings[this.resourceNameToString(r.getName().toObject())] = readings;
        }
      });
    },
    processFunctionResults: function (err, resp) {
      this.grpcCallback(err, resp, false);
      if (err) {
        document.getElementById('function_results').value = `${err}`;
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
      document.getElementById('function_results').value = resultStr;
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
        setPoint(p.point);
      } else {
        console.log('no point intersected');
      }
    },
    doPCDMove() {
      const gripperName = this.filterResources('rdk', 'component', 'gripper')[0];
      const cameraName = pcdGlobal.cameraName;
      const cameraPointX = this.pcdClick.x;
      const cameraPointY = this.pcdClick.y;
      const cameraPointZ = this.pcdClick.z;

      const req = new motionApi.MoveRequest();
      const cameraPoint = new commonApi.Pose();
      cameraPoint.setX(cameraPointX);
      cameraPoint.setY(cameraPointY);
      cameraPoint.setZ(cameraPointZ);

      const pose = new commonApi.PoseInFrame();
      pose.setReferenceFrame(cameraName);
      pose.setPose(cameraPoint);
      req.setDestination(pose);
      const componentName = new commonApi.ResourceName();
      componentName.setNamespace(gripperName.namespace);
      componentName.setType(gripperName.type);
      componentName.setSubtype(gripperName.subtype);
      componentName.setName(gripperName.name);
      req.setComponentName(componentName);
      console.log(`making move attempt using ${ gripperName}`);

      motionService.move(req, {}, (err, resp) => {
        this.grpcCallback(err, resp);
        if (err) {
          return Promise.reject(err);
        }
        return Promise.resolve(resp).then(console.log(`move success: ${ resp.getSuccess()}`));
      });
    },
    findSegments(segmenterName, segmenterParams) {
      console.log('parameters for segmenter below:');
      console.log(segmenterParams);
      this.pcdClick.calculatingSegments = true;
      this.pcdClick.foundSegments = false;
      const req = new visionApi.GetObjectPointCloudsRequest();
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
      setPoint(p);
      setBoundingBox(box, p);
      const mimeType = 'pointcloud/pcd';
      pcdLoad(`data:${mimeType};base64,${data}`);
    },
    doPointLoad (i) {
      const segment = this.objects[i];
      const center = segment.getGeometries().getGeometriesList()[0].getCenter();
      const p = { x: center.getX() / 1000, y: center.getY() / 1000, z: center.getZ() / 1000 };
      console.log(p);
      setPoint(p);
    },
    doBoundingBoxLoad (i) {
      const segment = this.objects[i];
      const center = segment.getGeometries().getGeometriesList()[0].getCenter();
      const box = segment.getGeometries().getGeometriesList()[0].getBox();
      const centerP = { x: center.getX() / 1000, y: center.getY() / 1000, z: center.getZ() / 1000 };
      setBoundingBox(box, centerP);
    },
    doPCDLoad (data) {
      const mimeType = 'pointcloud/pcd';
      pcdLoad(`data:${mimeType};base64,${data}`);
    },
    doCenterPCDLoad (data) {
      const mimeType = 'pointcloud/pcd';
      pcdLoad(`data:${mimeType};base64,${data}`);
      const p = { x: 0 / 1000, y: 0 / 1000, z: 0 / 1000 };
      console.log(p);
      setPoint(p);
    },
    doPCDDownload: function(data) {
      const mimeType = 'pointcloud/pcd';
      window.open(`data:${mimeType};base64,${data}`);
    },
    doSelectObject: function (selection, index) {
        console.log(selection);
        console.log(index);
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
    setNavigationMode: function (mode) {
      let pbMode = navigationApi.Mode.MODE_UNSPECIFIED;
      switch (mode) {
        case 'manual':
          pbMode = navigationApi.Mode.MODE_MANUAL;
          break;
        case 'waypoint':
          pbMode = navigationApi.Mode.MODE_WAYPOINT;
          break;
      }
      const req = new navigationApi.SetModeRequest();
      req.setMode(pbMode);
      navigationService.setMode(req, {}, this.grpcCallback);
    },
    setNavigationLocation (elId) {
      const posSplit = document.getElementById(elId).value.split(',');
      if (posSplit.length !== 2) {
        return;
      }
      const lat = Number.parseFloat(posSplit[0]);
      const lng = Number.parseFloat(posSplit[1]);
      const req = new robotApi.ResourceRunCommandRequest();
      let gpsName = '';
      gpses = this.filterResources('rdk', 'component', 'gps');
      if (gpses.length > 0) {
        gpsName = gpses[0].name;
      } else {
        this.error = 'no gps device found';
        return;
      }
      req.setResourceName(gpsName);
      req.setCommandName('set_location');
      req.setArgs(
        proto.google.protobuf.Struct.fromJavaScript({
          latitude: lat,
          longitude: lng,
        })
      );
      window.robotService.resourceRunCommand(req, {}, this.grpcCallback);
    },
    viewCamera(name) {
      const streamContainer = document.getElementById(`stream-${name}`);
      const req = new streamApi.AddStreamRequest();
      req.setName(name);
      const { grpcCallback } = this;
      streamService.addStream(req, {}, (err, resp) => {
        grpcCallback(err, resp, false);
        if (streamContainer && streamContainer.getElementsByTagName('img').length > 0) {
            streamContainer.getElementsByTagName('img')[0].remove();
        }
        if (err) {
          this.error = 'no live camera device found';
          return;
        }
      });
    },
    viewPreviewCamera(name) {
      const req = new streamApi.AddStreamRequest();
      req.setName(name);
      const { grpcCallback } = this;
      streamService.addStream(req, {}, (err, resp) => {
        grpcCallback(err, resp, false);
        if (err) {
          this.error = 'no live camera device found';
          return;
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
    getGPIO: function (boardName) {
      const pin = document.getElementById(`get_pin_${ boardName}`).value;
      BoardControlHelper.getGPIO(boardName, pin, (err, resp) => {
        if (err) {
          console.log(err);
          return;
        }
        const x = resp.toObject();
        document.getElementById(`get_pin_value_${ boardName}`).innerHTML = `Pin: ${ pin } is ${ x.high ? 'high' : 'low'}`;
      });
    },
    setGPIO (boardName) {
      const pin = document.getElementById(`set_pin_${ boardName}`).value;
      const v = document.getElementById(`set_pin_v_${ boardName}`).value;
      BoardControlHelper.setGPIO(boardName, pin, v === 'high', this.grpcCallback);
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
        const resourceNameToString = this.resourceNameToString.bind(this);

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
    async initNavigation() {
      const { grpcCallback } = this;

      await mapReady;
      window.map = new google.maps.Map(document.getElementById('map'), { zoom: 18 });
      window.map.addListener('click', (e) => {
        const req = new navigationApi.AddWaypointRequest();
        const point = new commonApi.GeoPoint();
        point.setLatitude(e.latLng.lat());
        point.setLongitude(e.latLng.lng());
        req.setLocation(point);
        navigationService.addWaypoint(req, {}, grpcCallback);
      });

      let centered = false;
      const knownWaypoints = {};
      let localLabelCounter = 0;
      
      const updateWaypoints = function () {
        const req = new navigationApi.GetWaypointsRequest();
        navigationService.getWaypoints(req, {}, (err, resp) => {
          grpcCallback(err, resp, false);
          if (err) {
            setTimeout(updateWaypoints, 1000);
            return;
          }
          let waypoints = [];
          if (resp) {
            waypoints = resp.getWaypointsList();
          }
          const currentWaypoints = {};
          for (const waypoint of waypoints) {
            const pos = { lat: waypoint.getLocation().getLatitude(), lng: waypoint.getLocation().getLongitude() };
            const posStr = JSON.stringify(pos);
            if (knownWaypoints[posStr]) {
              currentWaypoints[posStr] = knownWaypoints[posStr];
              continue;
            }
            const marker = new google.maps.Marker({
              position: pos,
              map: window.map,
              label: `${localLabelCounter++}`,
            });
            currentWaypoints[posStr] = marker;
            knownWaypoints[posStr] = marker;
            marker.addListener('click', () => {
              console.log('clicked on marker', pos);
            });
            marker.addListener('dblclick', () => {
              const req = new navigationApi.RemoveWaypointRequest();
              req.setId(waypoint.getId());
              navigationService.removeWaypoint(req, {}, grpcCallback);
            });
          }
          const waypointsToDelete = Object.keys(knownWaypoints).filter((elem) => {
            return !(elem in currentWaypoints);
          });
          for (key of waypointsToDelete) {
            const marker = knownWaypoints[key];
            marker.setMap(null);
            delete knownWaypoints[key];
          }
          setTimeout(updateWaypoints, 1000);
        });
      };
      updateWaypoints();

      const locationMarker = new google.maps.Marker({ label: 'robot' });
      const updateLocation = () => {
        const req = new navigationApi.GetLocationRequest();
        navigationService.getLocation(req, {}, (err, resp) => {
          grpcCallback(err, resp, false);
          if (err) {
            setTimeout(updateLocation, 1000);
            return;
          }
          const pos = { lat: resp.getLocation().getLatitude(), lng: resp.getLocation().getLongitude() };
          if (!centered) {
            centered = true;
            window.map.setCenter(pos);
          }
          locationMarker.setPosition(pos);
          locationMarker.setMap(window.map);
          setTimeout(updateLocation, 1000);
        });
      };
      updateLocation();
    },
    querySensors() {
      sensorsService.getSensors(new sensorsApi.GetSensorsRequest(), {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          return;
        }
        this.sensorNames = resp.toObject().sensorNamesList;
      });
    },
    loadCurrentOps () {
      const req = new robotApi.GetOperationsRequest();
      window.robotService.getOperations(req, {}, (err, resp) => {
        const lst = resp.toObject().operationsList;
        this.currentOps = lst;
      });
      setTimeout(this.loadCurrentOps, 500);
    },
    async doConnect(authEntity, creds, onError) {
      console.debug('connecting');
      const alertError = document.getElementById('connecting-error');
      alertError.innerHTML = '';
      document.getElementById('connecting').classList.remove('hidden');
      try {
        await window.connect(authEntity, creds);
        this.loadCurrentOps();
      } catch (error) {
        const msg = `failed to connect: ${error}`;
        console.error(msg);
        alertError.classList.remove('hidden').innerHTML = msg;
        if (onError) {
          setTimeout(onError, 1000);
        }
        return;
      }
      console.debug('connected');
      document.getElementById('pre-app').classList.add('hidden');
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
        const authDiv = document.getElementById(`auth-${authType}`);
        const input = authDiv.getElementsByTagName('input')[0];
        const button = authDiv.getElementsByTagName('button')[0];
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
    async queryStreams () {
      let pResolve;
      let pReject;
      const p = new Promise((resolve, reject) => {
        pResolve = resolve;
        pReject = reject;
      });
      streamService.listStreams(new streamApi.ListStreamsRequest(), {}, (err, resp) => {
        this.grpcCallback(err, resp, false);
        if (err) {
          pReject(err);
          return;
        }
        const streamNames = resp.toObject().namesList;
        this.streamNames = streamNames;
        pResolve(null);
      });
      await p;
      setTimeout(() => this.queryStreams(), 500);
    },
    initPCDIfNeeded() {
      if (pcdGlobal) {
        return;
      }
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
      document.getElementById('pcd').append(pcdGlobal.renderer.domElement);

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
      const all = this.filterResources('rdk', 'component', 'imu');
      for (const x of all) {
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
    updateStatus(grpcStatus) {
      const rawStatus = {};
      const status = {};

      for (const s of grpcStatus) {
        const nameObj = s.getName().toObject();
        const statusJs = s.getStatus().toJavaScript();
        const fixed = this.fixRawStatus(nameObj, statusJs);

        const nameStr = this.resourceNameToString(nameObj);
        rawStatus[nameStr] = statusJs;
        status[nameStr] = fixed;
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
        resourceNames = resourceNames.concat(this.filterResources('rdk', 'component', subtype));
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

window.initMap = () => mapReadyResolve();

let mapReadyResolve;
const mapReady = new Promise((resolve) => {
  mapReadyResolve = resolve;
});

</script>

<template>
  <div id="pre-app">
    <div
      id="connecting-error"
      class="hidden border-l-4 border-danger-500 bg-gray-100 px-4 py-3"
      role="alert"
    />

    <div
      id="connecting"
      class="hidden border-l-4 border-greendark bg-gray-100 px-4 py-3"
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
          class="mb-2 appearance-none block w-full border text-gray-700 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none transition-colors duration-150 ease-in-out p-2"
          type="password"
        >
        <button
          class="relative leading-tight font-button transition-colors duration-150 focus:outline-none shadow-sm text-black border border-black bg-primary hover:border-black hover:bg-gray-200 focus:bg-gray-400 active:bg-gray-400 cursor-pointer px-5 py-2"
        >
          Login
        </button>
      </div>
    </template>
  </div>
  
  <div class="flex flex-col gap-2 p-2">
    <div style="color: red;">
      {{ error }}
    </div>

    <!-- ******* BASE *******  -->
    <div
      v-for="base in filterResources('rdk', 'component', 'base')"
      :key="base.name"
      class="base pb-8"
    >
      <div v-if="streamNames.length === 0">
        <div class="camera">
          <viam-base
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
          <viam-base
            :base-name="base.name"
            :stream-name="streamName"
            :crumbs="['base', base.name]"
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
      v-for="gantry in filterResources('rdk', 'component', 'gantry')"
      v-if="resourceStatusByName(gantry)"
      :key="gantry.name"
      :title="`Gantry ${gantry.name}`"
      class="pb-8"
    >
      <div class="border border-black border-t-0 p-4">
        <table class="border border-black border-t-0 p-4">
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
              <td class="border border-black p-2">
                <v-button
                  group
                  label="--"
                  @click="gantryInc( gantry, pp.axis, -10 )"
                />
                <v-button
                  group
                  label="-"
                  @click="gantryInc( gantry, pp.axis, -1 )"
                />
                <v-button
                  group
                  label="+"
                  @click="gantryInc( gantry, pp.axis, 1 )"
                />
                <v-button
                  group
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
      v-for="(imu, x) in filterResources('rdk', 'component', 'imu').entries()"
      :key="imu[1].name"
      :title="`IMU: ${imu[1].name}`"
    >
      <div class="flex border border-black border-t-0 p-4">
        <template v-if="imuData[imu[1].name] && imuData[imu[1].name].angularVelocity">
          <div class="w-1/4 mr-4">
            <h3 class="mb-1">
              Orientation (degrees)
            </h3>
            <table class="w-full border border-black border-t-0 p-4">
              <tr>
                <th class="border border-black p-2">
                  Roll
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].orientation.rollDeg.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Pitch
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].orientation.pitchDeg.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Yaw
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].orientation.yawDeg.toFixed(2) }}
                </td>
              </tr>
            </table>
          </div>
                
          <div class="w-1/4 mr-4">
            <h3 class="mb-1">
              Angular Velocity (degrees/second)
            </h3>
            <table class="w-full border border-black border-t-0 p-4">
              <tr>
                <th class="border border-black p-2">
                  X
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].angularVelocity.xDegsPerSec.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Y
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].angularVelocity.yDegsPerSec.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Z
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].angularVelocity.zDegsPerSec.toFixed(2) }}
                </td>
              </tr>
            </table>
          </div>
                
          <div class="w-1/4 mr-4">
            <h3 class="mb-1">
              Acceleration (mm/second/second)
            </h3>
            <table class="w-full border border-black border-t-0 p-4">
              <tr>
                <th class="border border-black p-2">
                  X
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].acceleration.xMmPerSecPerSec.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Y
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].acceleration.yMmPerSecPerSec.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Z
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].acceleration.zMmPerSecPerSec.toFixed(2) }}
                </td>
              </tr>
            </table>
          </div>
                
          <div class="w-1/4">
            <h3 class="mb-1">
              Magnetometer (gauss)
            </h3>
            <table class="w-full border border-black border-t-0 p-4">
              <tr>
                <th class="border border-black p-2">
                  X
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].magnetometer.xGauss.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Y
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].magnetometer.yGauss.toFixed(2) }}
                </td>
              </tr>
              <tr>
                <th class="border border-black p-2">
                  Z
                </th>
                <td class="border border-black p-2">
                  {{ imuData[imu[1].name].magnetometer.zGauss.toFixed(2) }}
                </td>
              </tr>
            </table>
          </div>
        </template>
      </div>
    </v-collapse>

    <!-- ******* ARM *******  -->
    <v-collapse
      v-for="arm in filterResources('rdk', 'component', 'arm')"
      :key="arm.name"
      :title="`Arm ${arm.name}`"
      class="pb-8"
    >
      <div class="flex mb-2">
        <div
          v-if="armToggle[arm.name]"
          class="border border-black p-4 w-1/2 mr-4"
        >
          <h3 class="mb-2">
            END POSITION (mms)
          </h3>
          <div class="grid grid-cols-2 gap-1 pb-1">
            <template
              v-for="cc in armToggle[arm.name].pos_pieces"
              :key="cc.endPosition[0]"
            >
              <label class="pr-2 py-1 text-right">{{ cc.endPosition[1] }}</label>
              <input
                v-model="cc.endPositionValue"
                class="border border-black py-1 px-4"
              >
            </template>
          </div>
          <div class="flex mt-2">
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
          class="border border-black p-4 w-1/2"
        >
          <h3 class="mb-2">
            JOINTS (degrees)
          </h3>
          <div class="grid grid-cols-2 gap-1 pb-1">
            <template
              v-for="bb in armToggle[arm.name].joint_pieces"
              :key="bb.joint"
            >
              <label class="pr-2 py-1 text-right">Joint {{ bb.joint }}</label>
              <input
                v-model="bb.jointValue"
                class="border border-black py-1 px-4"
              >
            </template>
          </div>
          <div class="flex mt-2">
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

      <div class="flex mb-2">
        <div
          v-if="resourceStatusByName(arm)"
          class="border border-black p-4 w-1/2 mr-4"
        >
          <h3 class="mb-2">
            END POSITION (mms)
          </h3>
          <div class="grid grid-cols-6 gap-1 pb-1">
            <template
              v-for="aa in resourceStatusByName(arm).pos_pieces"
              :key="aa.endPosition[0]"
            >
              <h4 class="pr-2 py-1 text-right">
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
          <div class="flex mt-2">
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
          class="border border-black p-4 w-1/2"
        >
          <h3 class="mb-2">
            JOINTS (degrees)
          </h3>
          <div class="grid grid-cols-6 gap-1 pb-1">
            <template
              v-for="aa in resourceStatusByName(arm).joint_pieces"
              :key="aa.joint"
            >
              <h4 class="pr-2 py-1 text-right whitespace-nowrap">
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
              <h4 class="pl-2 py-1">
                {{ aa.jointValue.toFixed(2) }}
              </h4>
            </template>
          </div>
          <div class="flex mt-2">
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
      v-for="gripper in filterResources('rdk', 'component', 'gripper')"
      :key="gripper.name"
      :title="`Gripper ${gripper.name}`"
      class="pb-8"
    >
      <div class="flex gap-2 border border-black border-t-0 p-4">
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
    <v-collapse
      v-for="servo in filterResources('rdk', 'component', 'servo')"
      v-if="resourceStatusByName(servo)"
      :key="servo.name"
      :title="`Servo ${servo.name}`"
    >
      <div class="flex border border-black border-t-0 p-4">
        <table class="table-auto border-collapse border border-black">
          <tr>
            <td class="border border-black p-2">
              Angle
            </td>
            <td class="border border-black p-2">
              {{ resourceStatusByName(servo).positionDeg }}
            </td>
          </tr>
          <tr>
            <td class="border border-black p-2" />
            <td class="border border-black p-2">
              <v-button
                group
                label="-10"
                @click="servoMove(servo, -10)"
              />
              <v-button
                group
                label="-1"
                @click="servoMove(servo, -1)"
              />
              <v-button
                group
                label="1"
                @click="servoMove(servo, 1)"
              />
              <v-button
                group
                label="10"
                @click="servoMove(servo, 10)"
              />
            </td>
          </tr>
        </table>
      </div>
    </v-collapse>

    <!-- ******* MOTOR *******  -->
    <motor-detail 
      v-for="motor in filterResources('rdk', 'component', 'motor')"
      v-if="resourceStatusByName(motor)"
      :key="'new-' + motor.name"
      class="pb-8"  
      :motor-name="motor.name" 
      :crumbs="['motor', motor.name]" 
      :motor-status="resourceStatusByName(motor)"
      @motor-run="motorCommand(motor.name, $event)"
      @motor-stop="motorStop(motor.name)"
    />

    <!-- ******* INPUT VIEW *******  -->
    <input-controller
      v-for="controller in filteredInputControllerList()"
      v-if="resourceStatusByName(controller)"
      :key="'new-' + controller.name"
      class="pb-8"
      :controller-name="controller.name"
      :controller-status="resourceStatusByName(controller)"
    />

    <!-- ******* WEB CONTROLS *******  -->
    <web-gamepad
      v-if="hasWebGamepad()"
      class="pb-8"
      style="max-width: 1080px;"
      @execute="inputInject($event)"
    />

    <!-- ******* BOARD *******  -->
    <v-collapse
      v-for="board in filterResources('rdk', 'component', 'board')"
      v-if="resourceStatusByName(board)"
      :key="board.name"
    >
      <div class="flex">
        <h2 class="p-4 text-xl">
          Board {{ board.name }}
        </h2>
      </div>
      <template #content>
        <div class="border border-black border-t-0 p-4">
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
            GPIO
          </h3>
          <table class="mb-4 table-auto border border-black">
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
            DigiGPIOtalInterrupts
          </h3>
          <table class="mb-4 table-auto border border-black">
            <tr>
              <th class="border border-black p-2">
                Get
              </th>
              <td class="border border-black p-2">
                <div class="flex">
                  <label class="pr-2 py-2 text-right">Pin:</label>
                  <number-input
                    v-model="getPin"
                    class="mr-2"
                    :input-id="'get_pin_' + board.name"
                  />
                  <v-button
                    class="mr-2"
                    group
                    label="Get"
                    @click="getGPIO(board.name)"
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
                <div class="flex">
                  <label class="pr-2 py-2  text-right">Pin:</label>
                  <number-input
                    v-model="setPin"
                    class="mr-2"
                    :input-id="'set_pin_' + board.name"
                  />
                  <select
                    :id="'set_pin_v_' + board.name"
                    class="bg-white border border-black mr-2"
                    style="height: 38px"
                  >
                    <option>low</option>
                    <option>high</option>
                  </select>
                  <v-button
                    group
                    label="Set"
                    @click="setGPIO(board.name)"
                  />
                </div>
              </td>
            </tr>
          </table>
        </div>
      </template>
    </v-collapse>

    <!-- sensors -->
    <v-collapse
      v-if="nonEmpty(sensorNames)"
      class="pb-8"
      title="Sensors"
    >
      <div class="border border-black border-t-0 p-4">
        <table class="table-auto border border-black">
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
    <v-collapse
      v-if="filterResources('rdk', 'service', 'navigation').length > 0"
      title="Get Segments"
    >
      <div class="border border-black border-t-0 p-4">
        <div class="mb-2">
          <v-button
            group
            label="Manual"
            @click="setNavigationMode('manual')"
          />
          <v-button
            group
            label="Waypoint"
            @click="setNavigationMode('waypoint')"
          />
        </div>
        <div class="mb-2">
          <v-button
            group
            label="Try Set Location"
            @click="setNavigationLocation('nav-set-location')"
          />
        </div>
        <div
          id="map"
          v-map-mounted
          class="mb-2"
        />
        <viam-input
          v-model="locationValue"
          input-id="nav-set-location"
        />
      </div>
    </v-collapse>

    <!-- current operations -->
    <v-collapse
      title="Current Operations"
      class="pb-8"
    >
      <div class="border border-black border-t-0 p-4">
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
              {{ Date.now() - (o.started.seconds * 1000) }}ms
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
      @toggle-pcd="renderPCD(streamName); getSegmenterNames()"
      @pcd-click="grabClick"
      @pcd-move="doPCDMove"
      @point-load="doPointLoad"
      @segment-load="doSegmentLoad"
      @bounding-box-load="doBoundingBoxLoad"
      @download-screenshot="renderFrame(streamName)"
      @download-raw-data="doPCDDownload(fullcloud)"
      @select-object="doSelectObject"
    />

    <!-- ******* SLAM *******  -->
    <div
      v-if="filterResources('rdk', 'service', 'slam').length > 0"
      class="slam pb-8"
    >
      <Slam
        :image-map="imageMapTemp"
        :pcd-map="pcdMapTemp"
        @refresh-image-map="viewSLAMImageMap"
        @refresh-pcd-map="viewSLAMPCDMap"
      />
    </div>
  </div>
</template>

<style>
  #source {
    position: relative;
    width: 50%;
    height: 50%;
  }
  #map {
    height: 400px;
    width: 100%;
  }
  h3 {
    margin: 0.1em;
    margin-block-end: 0.1em;
  }
</style>
