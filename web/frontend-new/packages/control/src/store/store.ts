import Vue from "vue";
import Vuex, { MutationTree } from "vuex";

import { MotorStatus } from "proto/api/v1/robot_pb";
import {
  MotorServiceGoForResponse
} from "proto/api/component/v1/motor_pb";

import {
    CameraServiceGetFrameResponse,
    CameraServiceGetObjectPointCloudsResponse
} from "proto/api/component/v1/camera_pb";

interface systemInfo {
    build_time: string;
    commit: string;
    debug_enabled: boolean;
    frontend_enabled: boolean;
    semver: string;
  }

Vue.use(Vuex);

interface RootState {
    isInitialized: boolean;
    appInfo: systemInfo;
    cameraFrame: CameraServiceGetFrameResponse
    cameraPCD: CameraServiceGetObjectPointCloudsResponse,
    motorData: MotorServiceGoForResponse
  }

const state: RootState = {
    isInitialized: false,
    appInfo: {
      build_time: "",
      commit: "",
      debug_enabled: true,
      frontend_enabled: false,
      semver: "",
    },
  
    cameraFrame: <CameraServiceGetFrameResponse>{},
    cameraPCD: <CameraServiceGetObjectPointCloudsResponse>{},
  
    motorData: <MotorServiceGoForResponse>{},
  };

  const mutations: MutationTree<RootState> = {
    setIsInitialized(state) {
      state.isInitialized = true;
    },
    updateAppInfo(state, systemInfo: systemInfo) {
      state.appInfo = systemInfo;
    },
    updateCameraFrame(state, cameraFrame: CameraServiceGetFrameResponse) {
      state.cameraFrame = cameraFrame;
    },
    updatePCD(state, cameraPCD: CameraServiceGetObjectPointCloudsResponse) {
        state.cameraPCD = cameraPCD;
    },
    updateMotorData(state, motorData: MotorServiceGoForResponse) {
        state.motorData = motorData;
    },
  };
  
  const store = new Vuex.Store<RootState>({
    state,
    mutations,
  });
  
  export default store;