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
};

/*
  Input: State of keys. e.g. {straight : true, backward : false, right : false, left: false}
  Output: linearY and angularZ throttle
*/
window.computeKeyboardBaseControls = function(keysPressed) {
  let linear;
  let angular;

  if (keysPressed.forward && keysPressed.backward) {
    linear = 0;
  } else if (keysPressed.forward) {
    linear = 1;
  } else if (keysPressed.backward) {
    lienar = -1;
  } else {
    linear = 0;
  }

  // Angle
  if (keysPressed.right && keysPressed.left) {
    angular = 0;
  } else if (keysPressed.right) {
    angular = -1;
  } else if (keysPressed.left) {
    angular = 1;
  } else {
    angleDeg = 0;
  }

  return {linear, angular};
};

