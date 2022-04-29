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
}

const keyboardBaseDefaults = {
  maxSpeed : 500.0,
	maxAngle : 425.0,
	distRatio : 10
}

/*
  Input: State of keys. e.g. {ButtonNorth : true, ButtonSouth : false, ButtonEast : false, ButtonWest: false}
  Output: distance, speed, and angle parameters for MoveArc
*/
function computeKeyboardBaseControls(keysPressed) {
  let mmPerSec;
  let angleDeg;

  if (keysPressed.ButtonNorth && keysPressed.ButtonSouth) {
    mmPerSec = 0.0;
  } else if (keysPressed.ButtonNorth) {
    mmPerSec = 1.0;
  } else if (keysPressed.ButtonSouth) {
    mmPerSec = -1.0;
  } else {
    mmPerSec = 0.0;
  }

  // Angle
  if (keysPressed.ButtonEast && keysPressed.ButtonWest) {
    angleDeg = 0.0;
  } else if (keysPressed.ButtonEast) {
    angleDeg = -1.0;
  } else if (keysPressed.ButtonWest) {
    angleDeg = 1.0;
  } else {
    angleDeg = 0.0;
  }

  let distance;
  let speed;
  let angle;

  let moveType; // for logging only
  if (mmPerSec == 0 && angleDeg == 0) {
    moveType = 'Stop';
    distance = keyboardBaseDefaults.maxSpeed * keyboardBaseDefaults.distRatio;
    speed = 0.0;
    angle = angleDeg * keyboardBaseDefaults.maxAngle * -1;
  } else if (mmPerSec == 0) {
    moveType = 'Spin';
    distance = 0;
    speed = angleDeg * keyboardBaseDefaults.maxSpeed;
    angle = Math.abs(angleDeg * keyboardBaseDefaults.maxAngle * keyboardBaseDefaults.distRatio / 2);
  } else if (angleDeg == 0) {
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

  console.log("%s: s = %f | a = %f | Dist = %f | Speed = %f | Angle = %f", mmPerSec, angleDeg, distance, speed, angle);
  return {'distance' : distance, 'speed' : speed, 'angle' : angle};
}