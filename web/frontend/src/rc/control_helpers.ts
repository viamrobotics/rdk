/*
* This file contains gRPC helper functions for the Remote Control page.
* These helpers will be deprecated by a future node SDK.
* Feel free to add any missing gRPC method wrappers.
*/

import { grpc } from '@improbable-eng/grpc-web';
import motorApi from '../gen/proto/api/component/motor/v1/motor_pb.esm';
import baseApi from '../gen/proto/api/component/base/v1/base_pb.esm';
import servoApi from '../gen/proto/api/component/servo/v1/servo_pb.esm';
import type { Vector3 } from '../gen/proto/api/common/v1/common_pb.esm';
import type { ServiceError } from '../gen/proto/stream/v1/stream_pb_service.esm';
import { rcLogConditionally } from '../lib/log';

window.rcDebug = false;

type Callback = (error: ServiceError | null, responseMessage: unknown | null) => void

// Base control helpers
export const BaseControlHelper = {
  moveStraight(name: string, distance_mm: number, speed_mm_s: number, cb: Callback) {
    const req = new baseApi.MoveStraightRequest();
    req.setName(name);
    req.setMmPerSec(speed_mm_s);
    req.setDistanceMm(distance_mm);

    rcLogConditionally(req);
    window.baseService.moveStraight(req, new grpc.Metadata(), cb);
  },

  spin(name: string, angle_deg: number, speed_deg_s: number, cb: Callback) {
    const req = new baseApi.SpinRequest();
    req.setName(name);
    req.setAngleDeg(angle_deg);
    req.setDegsPerSec(speed_deg_s);

    rcLogConditionally(req);
    window.baseService.spin(req, new grpc.Metadata(), cb);
  },

  setPower(name: string, linearVector: Vector3, angularVector: Vector3, cb: Callback) {
    const req = new baseApi.SetPowerRequest();
    req.setName(name);
    req.setLinear(linearVector);
    req.setAngular(angularVector);

    rcLogConditionally(req);
    window.baseService.setPower(req, new grpc.Metadata(), cb);
  },

  setVelocity(name: string, linearVector: Vector3, angularVector: Vector3, cb: Callback) {
    const req = new baseApi.SetVelocityRequest();
    req.setName(name);
    req.setLinear(linearVector);
    req.setAngular(angularVector);

    rcLogConditionally(req);
    window.baseService.setVelocity(req, new grpc.Metadata(), cb);
  },
};

/*
* Base keyboard control calculations.
* Input: State of keys. e.g. {straight : true, backward : false, right : false, left: false}
* Output: linearY and angularZ throttle
*/
export const computeKeyboardBaseControls = (keysPressed: Record<string, boolean>) => {
  let linear = 0;
  let angular = 0;

  if (keysPressed.forward) {
    linear = 1;
  } else if (keysPressed.backward) {
    linear = -1;
  } 
    
  if (keysPressed.right) {
    angular = -1;
  } else if (keysPressed.left) {
    angular = 1;
  } 
    
  return { linear, angular };
};

// Simple motor control helpers
export const MotorControlHelper = {
  setPower(name: string, powerPct: number, cb: Callback) {
    const req = new motorApi.SetPowerRequest();
    req.setName(name);
    req.setPowerPct(powerPct);

    rcLogConditionally(req);
    window.motorService.setPower(req, new grpc.Metadata(), cb);
  },
  
  goFor(name: string, rpm: number, revolutions: number, cb: Callback) {
    const req = new motorApi.GoForRequest();
    req.setName(name);
    req.setRpm(rpm);
    req.setRevolutions(revolutions);

    rcLogConditionally(req);
    window.motorService.goFor(req, new grpc.Metadata(), cb);
  },

  goTo(name: string, rpm: number, positionRevolutions: number, cb: Callback) {
    const req = new motorApi.GoToRequest();
    req.setName(name);
    req.setRpm(rpm);
    req.setPositionRevolutions(positionRevolutions);

    rcLogConditionally(req);
    window.motorService.goTo(req, new grpc.Metadata(), cb);
  },

  stop(name: string, cb: Callback) {
    const req = new motorApi.StopRequest();
    req.setName(name);

    rcLogConditionally(req);
    window.motorService.stop(req, new grpc.Metadata(), cb);
  },
};

// Servo control helpers
// todo: add the rest
export const ServoControlHelper = {
  stop(name: string, cb: Callback) {
    const req = new servoApi.StopRequest();
    req.setName(name);

    rcLogConditionally(req);
    window.servoService.stop(req, new grpc.Metadata(), cb);
  },
};
