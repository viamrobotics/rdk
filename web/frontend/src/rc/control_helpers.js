/*
* This file contains gRPC helper functions for the Remote Control page.
* These helpers will be deprecated by a future node SDK.
* Feel free to add any missing gRPC method wrappers.
*/

import boardApi from '../gen/proto/api/component/board/v1/board_pb.esm';
import motorApi from '../gen/proto/api/component/motor/v1/motor_pb.esm';

// Base control helpers
export const BaseControlHelper = {
  moveStraight(name, distance_mm, speed_mm_s, cb) {
    const req = new baseApi.MoveStraightRequest();
    req.setName(name);
    req.setMmPerSec(speed_mm_s);
    req.setDistanceMm(distance_mm);

    rcLogConditionally(req);
    window.baseService.moveStraight(req, {}, cb);
  },

  spin(name, angle_deg, speed_deg_s, cb) {
    const req = new baseApi.SpinRequest();
    req.setName(name);
    req.setAngleDeg(angle_deg);
    req.setDegsPerSec(speed_deg_s);

    rcLogConditionally(req);
    window.baseService.spin(req, {}, cb);
  },

  setPower(name, linearVector, angularVector, cb) {
      const req = new baseApi.SetPowerRequest();
      req.setName(name);
      req.setLinear(linearVector);
      req.setAngular(angularVector);

      rcLogConditionally(req);
      window.baseService.setPower(req, {}, cb);
  },

  setVelocity(name, linearVector, angularVector, cb) {
    const req = new baseApi.SetVelocityRequest();
    req.setName(name);
    req.setLinear(linearVector);
    req.setAngular(angularVector);

    rcLogConditionally(req);
    window.baseService.setVelocity(req, {}, cb);
  },
};

/*
* Base keyboard control calculations.
* Input: State of keys. e.g. {straight : true, backward : false, right : false, left: false}
* Output: linearY and angularZ throttle
*/
export const computeKeyboardBaseControls = (keysPressed) => {
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
    
    return {linear, angular};
};

// Simple motor control helpers
export const MotorControlHelper = {
  setPower(name, powerPct, cb) {
    const req = new motorApi.SetPowerRequest();
    req.setName(name);
    req.setPowerPct(powerPct);

    rcLogConditionally(req);
    window.motorService.setPower(req, {}, cb);
  },
  
  goFor(name, rpm, revolutions, cb) {
    const req = new motorApi.GoForRequest();
    req.setName(name);
    req.setRpm(rpm);
    req.setRevolutions(revolutions);

    rcLogConditionally(req);
    window.motorService.goFor(req, {}, cb);
  },

  goTo(name, rpm, positionRevolutions, cb) {
    const req = new motorApi.GoToRequest();
    req.setName(name);
    req.setRpm(rpm);
    req.setPositionRevolutions(positionRevolutions);

    rcLogConditionally(req);
    window.motorService.goTo(req, {}, cb);
  },

  stop(name, cb) {
    const req = new motorApi.StopRequest();
    req.setName(name);

    rcLogConditionally(req);
    window.motorService.stop(req, {}, cb);
  },
};

// Simple motor control helpers
export const BoardControlHelper = {
  getGPIO (name, pin, cb) {
    const req = new boardApi.GetGPIORequest();
    req.setName(name);
    req.setPin(pin);

    rcLogConditionally(req);
    window.boardService.getGPIO(req, {}, cb);
  },

  setGPIO (name, pin, value, cb) {
    const req = new boardApi.SetGPIORequest();
    req.setName(name);
    req.setPin(pin);
    req.setHigh(value);

    rcLogConditionally(req);
    widnow.boardService.setGPIO(req, {}, cb);
  },

  // TODO: Add PWM
};
