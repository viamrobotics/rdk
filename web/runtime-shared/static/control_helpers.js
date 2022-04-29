/******/ (() => { // webpackBootstrap
var __webpack_exports__ = {};
/*!***********************************!*\
  !*** ./src/rc/control_helpers.js ***!
  \***********************************/
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
/******/ })()
;
//# sourceMappingURL=data:application/json;charset=utf-8;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly9yZGstd2ViLy4vc3JjL3JjL2NvbnRyb2xfaGVscGVycy5qcyJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7OztBQUFBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLG9DQUFvQztBQUNwQyxHQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLCtCQUErQjtBQUMvQixHQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQSw0QkFBNEI7QUFDNUIsR0FBRztBQUNIOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQSw4QkFBOEI7QUFDOUI7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0EsR0FBRztBQUNIO0FBQ0EsR0FBRztBQUNIO0FBQ0EsR0FBRztBQUNIO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0EsR0FBRztBQUNIO0FBQ0EsR0FBRztBQUNIO0FBQ0EsR0FBRztBQUNIO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBLGVBQWU7QUFDZjtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0EsR0FBRztBQUNIO0FBQ0E7QUFDQTtBQUNBO0FBQ0EsR0FBRztBQUNIO0FBQ0E7QUFDQTtBQUNBO0FBQ0EsR0FBRztBQUNIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQSxVQUFVO0FBQ1YsQyIsImZpbGUiOiJjb250cm9sX2hlbHBlcnMuanMiLCJzb3VyY2VzQ29udGVudCI6WyIvKlxuU2ltcGxlIGJhc2UgY29udHJvbCBoZWxwZXJzLiBTaG91bGQgYmUgcmVwbGFjZWQgYnkgYSBwcm9wZXIgU0RLIG9uY2UgYXZhaWxhYmxlLlxuKi9cbndpbmRvdy5CYXNlQ29udHJvbEhlbHBlciA9IHtcbiAgbW92ZVN0cmFpZ2h0OiBmdW5jdGlvbihuYW1lLCBkaXN0YW5jZV9tbSwgc3BlZWRfbW1fcywgY2IpIHtcbiAgICBjb25zdCByZXEgPSBuZXcgYmFzZUFwaS5Nb3ZlU3RyYWlnaHRSZXF1ZXN0KCk7XG4gICAgcmVxLnNldE5hbWUobmFtZSk7XG4gICAgcmVxLnNldE1tUGVyU2VjKHNwZWVkX21tX3MpO1xuICAgIHJlcS5zZXREaXN0YW5jZU1tKGRpc3RhbmNlX21tKTtcblxuICAgIHJjTG9nQ29uZGl0aW9uYWxseShyZXEpO1xuICAgIGJhc2VTZXJ2aWNlLm1vdmVTdHJhaWdodChyZXEsIHt9LCBjYik7XG4gIH0sXG5cbiAgbW92ZUFyYzogZnVuY3Rpb24obmFtZSwgZGlzdGFuY2VfbW0sIHNwZWVkX21tX3MsIGFuZ2xlX2RlZywgY2IpIHtcbiAgICBjb25zdCByZXEgPSBuZXcgYmFzZUFwaS5Nb3ZlQXJjUmVxdWVzdCgpO1xuICAgIHJlcS5zZXROYW1lKG5hbWUpO1xuICAgIHJlcS5zZXREaXN0YW5jZU1tKGRpc3RhbmNlX21tKTtcbiAgICByZXEuc2V0TW1QZXJTZWMoc3BlZWRfbW1fcyk7XG4gICAgcmVxLnNldEFuZ2xlRGVnKGFuZ2xlX2RlZyk7XG5cbiAgICByY0xvZ0NvbmRpdGlvbmFsbHkocmVxKTtcbiAgICBiYXNlU2VydmljZS5tb3ZlQXJjKHJlcSwge30sIGNiKTtcbiAgfSxcblxuICBzcGluOiBmdW5jdGlvbihuYW1lLCBhbmdsZV9kZWcsIHNwZWVkX2RlZ19zLCBjYikge1xuICAgIGNvbnN0IHJlcSA9IG5ldyBiYXNlQXBpLlNwaW5SZXF1ZXN0KCk7XG4gICAgcmVxLnNldE5hbWUobmFtZSk7XG4gICAgcmVxLnNldEFuZ2xlRGVnKGFuZ2xlX2RlZyk7XG4gICAgcmVxLnNldERlZ3NQZXJTZWMoc3BlZWRfZGVnX3MpO1xuXG4gICAgcmNMb2dDb25kaXRpb25hbGx5KHJlcSk7XG4gICAgYmFzZVNlcnZpY2Uuc3BpbihyZXEsIHt9LCBjYik7XG4gIH0sXG59XG5cbmNvbnN0IGtleWJvYXJkQmFzZURlZmF1bHRzID0ge1xuICBtYXhTcGVlZCA6IDUwMC4wLFxuXHRtYXhBbmdsZSA6IDQyNS4wLFxuXHRkaXN0UmF0aW8gOiAxMFxufVxuXG4vKlxuICBJbnB1dDogU3RhdGUgb2Yga2V5cy4gZS5nLiB7QnV0dG9uTm9ydGggOiB0cnVlLCBCdXR0b25Tb3V0aCA6IGZhbHNlLCBCdXR0b25FYXN0IDogZmFsc2UsIEJ1dHRvbldlc3Q6IGZhbHNlfVxuICBPdXRwdXQ6IGRpc3RhbmNlLCBzcGVlZCwgYW5kIGFuZ2xlIHBhcmFtZXRlcnMgZm9yIE1vdmVBcmNcbiovXG5mdW5jdGlvbiBjb21wdXRlS2V5Ym9hcmRCYXNlQ29udHJvbHMoa2V5c1ByZXNzZWQpIHtcbiAgbGV0IG1tUGVyU2VjO1xuICBsZXQgYW5nbGVEZWc7XG5cbiAgaWYgKGtleXNQcmVzc2VkLkJ1dHRvbk5vcnRoICYmIGtleXNQcmVzc2VkLkJ1dHRvblNvdXRoKSB7XG4gICAgbW1QZXJTZWMgPSAwLjA7XG4gIH0gZWxzZSBpZiAoa2V5c1ByZXNzZWQuQnV0dG9uTm9ydGgpIHtcbiAgICBtbVBlclNlYyA9IDEuMDtcbiAgfSBlbHNlIGlmIChrZXlzUHJlc3NlZC5CdXR0b25Tb3V0aCkge1xuICAgIG1tUGVyU2VjID0gLTEuMDtcbiAgfSBlbHNlIHtcbiAgICBtbVBlclNlYyA9IDAuMDtcbiAgfVxuXG4gIC8vIEFuZ2xlXG4gIGlmIChrZXlzUHJlc3NlZC5CdXR0b25FYXN0ICYmIGtleXNQcmVzc2VkLkJ1dHRvbldlc3QpIHtcbiAgICBhbmdsZURlZyA9IDAuMDtcbiAgfSBlbHNlIGlmIChrZXlzUHJlc3NlZC5CdXR0b25FYXN0KSB7XG4gICAgYW5nbGVEZWcgPSAtMS4wO1xuICB9IGVsc2UgaWYgKGtleXNQcmVzc2VkLkJ1dHRvbldlc3QpIHtcbiAgICBhbmdsZURlZyA9IDEuMDtcbiAgfSBlbHNlIHtcbiAgICBhbmdsZURlZyA9IDAuMDtcbiAgfVxuXG4gIGxldCBkaXN0YW5jZTtcbiAgbGV0IHNwZWVkO1xuICBsZXQgYW5nbGU7XG5cbiAgbGV0IG1vdmVUeXBlOyAvLyBmb3IgbG9nZ2luZyBvbmx5XG4gIGlmIChtbVBlclNlYyA9PSAwICYmIGFuZ2xlRGVnID09IDApIHtcbiAgICBtb3ZlVHlwZSA9ICdTdG9wJztcbiAgICBkaXN0YW5jZSA9IGtleWJvYXJkQmFzZURlZmF1bHRzLm1heFNwZWVkICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMuZGlzdFJhdGlvO1xuICAgIHNwZWVkID0gMC4wO1xuICAgIGFuZ2xlID0gYW5nbGVEZWcgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5tYXhBbmdsZSAqIC0xO1xuICB9IGVsc2UgaWYgKG1tUGVyU2VjID09IDApIHtcbiAgICBtb3ZlVHlwZSA9ICdTcGluJztcbiAgICBkaXN0YW5jZSA9IDA7XG4gICAgc3BlZWQgPSBhbmdsZURlZyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heFNwZWVkO1xuICAgIGFuZ2xlID0gTWF0aC5hYnMoYW5nbGVEZWcgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5tYXhBbmdsZSAqIGtleWJvYXJkQmFzZURlZmF1bHRzLmRpc3RSYXRpbyAvIDIpO1xuICB9IGVsc2UgaWYgKGFuZ2xlRGVnID09IDApIHtcbiAgICBtb3ZlVHlwZSA9ICdTdHJhaWdodCc7XG4gICAgZGlzdGFuY2UgPSBNYXRoLmFicyhtbVBlclNlYyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heFNwZWVkICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMuZGlzdFJhdGlvKTtcbiAgICBzcGVlZCA9IG1tUGVyU2VjICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMubWF4U3BlZWQ7XG4gICAgYW5nbGUgPSBNYXRoLmFicyhhbmdsZURlZyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heEFuZ2xlICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMuZGlzdFJhdGlvKTtcbiAgfSBlbHNlIHtcbiAgICBtb3ZlVHlwZSA9ICdBcmMnO1xuICAgIGRpc3RhbmNlID0gTWF0aC5hYnMobW1QZXJTZWMgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5tYXhTcGVlZCAqIGtleWJvYXJkQmFzZURlZmF1bHRzLmRpc3RSYXRpbyk7XG4gICAgc3BlZWQgPSBtbVBlclNlYyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heFNwZWVkO1xuICAgIGFuZ2xlID0gYW5nbGVEZWcgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5tYXhBbmdsZSAqIGtleWJvYXJkQmFzZURlZmF1bHRzLmRpc3RSYXRpbyAqIDIgLSAxO1xuICB9XG5cbiAgY29uc29sZS5sb2coXCIlczogcyA9ICVmIHwgYSA9ICVmIHwgRGlzdCA9ICVmIHwgU3BlZWQgPSAlZiB8IEFuZ2xlID0gJWZcIiwgbW1QZXJTZWMsIGFuZ2xlRGVnLCBkaXN0YW5jZSwgc3BlZWQsIGFuZ2xlKTtcbiAgcmV0dXJuIHsnZGlzdGFuY2UnIDogZGlzdGFuY2UsICdzcGVlZCcgOiBzcGVlZCwgJ2FuZ2xlJyA6IGFuZ2xlfTtcbn0iXSwic291cmNlUm9vdCI6IiJ9