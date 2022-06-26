/*
* This file contains gRPC helper functions for the Remote Control page.
* These helpers will be deprecated by a future node SDK.
* Feel free to add any missing gRPC method wrappers.
*/

// Base control helpers
window.BaseControlHelper = {
  moveStraight: function(name, distance_mm, speed_mm_s, cb) {
    const req = new baseApi.MoveStraightRequest();
    req.setName(name);
    req.setMmPerSec(speed_mm_s);
    req.setDistanceMm(distance_mm);

    rcLogConditionally(req);
    baseService.moveStraight(req, {}, cb);
  },

  spin: function(name, angle_deg, speed_deg_s, cb) {
    const req = new baseApi.SpinRequest();
    req.setName(name);
    req.setAngleDeg(angle_deg);
    req.setDegsPerSec(speed_deg_s);

    rcLogConditionally(req);
    baseService.spin(req, {}, cb);
  },

  setPower: function(name, linearVector, angularVector, cb) {
      const req = new baseApi.SetPowerRequest();
      req.setName(name);
      req.setLinear(linearVector);
      req.setAngular(angularVector);

      rcLogConditionally(req);
      baseService.setPower(req, {}, cb);
  },

};

/*
* Base keyboard control calculations.
* Input: State of keys. e.g. {straight : true, backward : false, right : false, left: false}
* Output: linearY and angularZ throttle
*/
window.computeKeyboardBaseControls = function(keysPressed) {
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
window.MotorControlHelper = {
  setPower: function(name, powerPct, cb) {
    const req = new motorApi.SetPowerRequest();
    req.setName(name);
    req.setPowerPct(powerPct);

    rcLogConditionally(req);
    motorService.setPower(req, {}, cb);
  },
  
  goFor: function(name, rpm, revolutions, cb) {
    const req = new motorApi.GoForRequest();
    req.setName(name);
    req.setRpm(rpm);
    req.setRevolutions(revolutions);

    rcLogConditionally(req);
    motorService.goFor(req, {}, cb);
  },

  goTo: function(name, rpm, positionRevolutions, cb) {
    const req = new motorApi.GoToRequest();
    req.setName(name);
    req.setRpm(rpm);
    req.setPositionRevolutions(positionRevolutions);

    rcLogConditionally(req);
    motorService.goTo(req, {}, cb);
  },

  stop: function(name, cb) {
    const req = new motorApi.StopRequest();
    req.setName(name);

    rcLogConditionally(req);
    motorService.stop(req, {}, cb);
  },
};

// Simple motor control helpers
window.BoardControlHelper = {
  getGPIO: function (name, pin, cb) {
    const req = new boardApi.GetGPIORequest();
    req.setName(name);
    req.setPin(pin);

    rcLogConditionally(req);
    boardService.getGPIO(req, {}, cb);
  },

  setGPIO: function (name, pin, value, cb) {
    const req = new boardApi.SetGPIORequest();
    req.setName(name);
    req.setPin(pin);
    req.setHigh(value);

    rcLogConditionally(req);
    boardService.setGPIO(req, {}, cb);
  },
};

// PWM helpers
window.PWMControlHelper = {
  getPWM: function (name, pin, cb) {
    const req = new boardApi.PWMRequest();
    req.setName(name);
    req.setPin(pin);
  
    rcLogConditionally(req);
    boardService.pWM(req, {}, cb);
  },
  
  setPWM: function (name, pin, value, cb) {
    const req = new boardApi.SetPWMRequest();
    req.setName(name);
    req.setPin(pin);
    req.setDutyCyclePct(value);
  
    rcLogConditionally(req);
    boardService.setPWM(req, {}, cb);
  },
};
