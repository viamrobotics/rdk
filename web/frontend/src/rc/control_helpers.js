/*
Simple base control helpers. Should be replaced by a proper SDK once available.
*/
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
  Input: State of keys. e.g. {straight : true, backward : false, right : false, left: false}
  Output: linearY and angularZ throttle
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

/*
Simple base control helpers. Should be replaced by a proper SDK once available.
*/
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
