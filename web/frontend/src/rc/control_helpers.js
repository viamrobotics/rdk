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

  moveArc: function(name, distance_mm, speed_mm_s, angle_deg, cb) {
    const req = new baseApi.MoveArcRequest();
    req.setName(name);
    req.setDistanceMm(distance_mm);
    req.setMmPerSec(speed_mm_s);
    req.setAngleDeg(angle_deg);

    rcLogConditionally(req);
    baseService.moveArc(req, {}, cb);
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

// Leaving in window scope for tunning. Should be const or in inputs
window.keyboardBaseDefaults = {
  maxSpeed : 300,
  maxAngle : 425,
  distRatio : 10,
};

/*
  Input: State of keys. e.g. {straight : true, backward : false, right : false, left: false}
  Output: distance, speed, and angle parameters for MoveArc
*/
window.computeKeyboardBaseControls = function(keysPressed) {
  let mmPerSec;
  let angleDeg;

  if (keysPressed.forward && keysPressed.backward) {
    mmPerSec = 0;
  } else if (keysPressed.forward) {
    mmPerSec = 1;
  } else if (keysPressed.backward) {
    mmPerSec = -1;
  } else {
    mmPerSec = 0;
  }

  // Angle
  if (keysPressed.right && keysPressed.left) {
    angleDeg = 0;
  } else if (keysPressed.right) {
    angleDeg = -1;
  } else if (keysPressed.left) {
    angleDeg = 1;
  } else {
    angleDeg = 0;
  }

  let distance;
  let speed;
  let angle;

  let moveType; // for logging only
  if (mmPerSec === 0 && angleDeg === 0) {
    moveType = 'Stop';
    distance = keyboardBaseDefaults.maxSpeed * keyboardBaseDefaults.distRatio;
    speed = 0;
    angle = angleDeg * keyboardBaseDefaults.maxAngle * -1;
  } else if (mmPerSec === 0) {
    moveType = 'Spin';
    distance = 0;
    speed = angleDeg * keyboardBaseDefaults.maxSpeed;
    angle = Math.abs(angleDeg * keyboardBaseDefaults.maxAngle * keyboardBaseDefaults.distRatio / 2);
  } else if (angleDeg === 0) {
    moveType = 'Straight';
    distance = Math.abs(mmPerSec * keyboardBaseDefaults.maxSpeed * keyboardBaseDefaults.distRatio);
    speed = mmPerSec * keyboardBaseDefaults.maxSpeed;
    angle = Math.abs(angleDeg * keyboardBaseDefaults.maxAngle * keyboardBaseDefaults.distRatio);
  } else {
    moveType = 'Arc';
    distance = Math.abs(mmPerSec * keyboardBaseDefaults.maxSpeed * keyboardBaseDefaults.distRatio);
    speed = mmPerSec * keyboardBaseDefaults.maxSpeed;
    angle = angleDeg * keyboardBaseDefaults.maxAngle * keyboardBaseDefaults.distRatio * 2 - 1;
  }

  console.log('%s: s = %f | a = %f | Dist = %f | Speed = %f | Angle = %f', moveType, mmPerSec, angleDeg, distance, speed, angle);
  return {distance, speed, angle};
};
