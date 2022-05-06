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

// Leaving in window scope for tunning. Should be const or in inputs
window.keyboardBaseDefaults = {
  maxSpeed : 300.0,
  maxAngle : 425.0,
  distRatio : 10
}

/*
  Input: State of keys. e.g. {straight : true, backward : false, right : false, left: false}
  Output: distance, speed, and angle parameters for MoveArc
*/
window.computeKeyboardBaseControls = function(keysPressed) {
  let mmPerSec;
  let angleDeg;

  if (keysPressed.forward && keysPressed.backward) {
    mmPerSec = 0.0;
  } else if (keysPressed.forward) {
    mmPerSec = 1.0;
  } else if (keysPressed.backward) {
    mmPerSec = -1.0;
  } else {
    mmPerSec = 0.0;
  }

  // Angle
  if (keysPressed.right && keysPressed.left) {
    angleDeg = 0.0;
  } else if (keysPressed.right) {
    angleDeg = -1.0;
  } else if (keysPressed.left) {
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

  console.log("%s: s = %f | a = %f | Dist = %f | Speed = %f | Angle = %f", moveType, mmPerSec, angleDeg, distance, speed, angle);
  return {distance, speed, angle};
}

/******/ })()
;
//# sourceMappingURL=data:application/json;charset=utf-8;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly9yZGstd2ViLy4vc3JjL3JjL2NvbnRyb2xfaGVscGVycy5qcyJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7OztBQUFBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLG9DQUFvQztBQUNwQyxHQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLCtCQUErQjtBQUMvQixHQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQSw0QkFBNEI7QUFDNUIsR0FBRztBQUNIOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLDhCQUE4QjtBQUM5QjtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQSxHQUFHO0FBQ0g7QUFDQSxHQUFHO0FBQ0g7QUFDQSxHQUFHO0FBQ0g7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQSxHQUFHO0FBQ0g7QUFDQSxHQUFHO0FBQ0g7QUFDQSxHQUFHO0FBQ0g7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUEsZUFBZTtBQUNmO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxHQUFHO0FBQ0g7QUFDQTtBQUNBO0FBQ0E7QUFDQSxHQUFHO0FBQ0g7QUFDQTtBQUNBO0FBQ0E7QUFDQSxHQUFHO0FBQ0g7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLFVBQVU7QUFDViIsImZpbGUiOiJjb250cm9sX2hlbHBlcnMuanMiLCJzb3VyY2VzQ29udGVudCI6WyIvKlxuU2ltcGxlIGJhc2UgY29udHJvbCBoZWxwZXJzLiBTaG91bGQgYmUgcmVwbGFjZWQgYnkgYSBwcm9wZXIgU0RLIG9uY2UgYXZhaWxhYmxlLlxuKi9cbndpbmRvdy5CYXNlQ29udHJvbEhlbHBlciA9IHtcbiAgbW92ZVN0cmFpZ2h0OiBmdW5jdGlvbihuYW1lLCBkaXN0YW5jZV9tbSwgc3BlZWRfbW1fcywgY2IpIHtcbiAgICBjb25zdCByZXEgPSBuZXcgYmFzZUFwaS5Nb3ZlU3RyYWlnaHRSZXF1ZXN0KCk7XG4gICAgcmVxLnNldE5hbWUobmFtZSk7XG4gICAgcmVxLnNldE1tUGVyU2VjKHNwZWVkX21tX3MpO1xuICAgIHJlcS5zZXREaXN0YW5jZU1tKGRpc3RhbmNlX21tKTtcblxuICAgIHJjTG9nQ29uZGl0aW9uYWxseShyZXEpO1xuICAgIGJhc2VTZXJ2aWNlLm1vdmVTdHJhaWdodChyZXEsIHt9LCBjYik7XG4gIH0sXG5cbiAgbW92ZUFyYzogZnVuY3Rpb24obmFtZSwgZGlzdGFuY2VfbW0sIHNwZWVkX21tX3MsIGFuZ2xlX2RlZywgY2IpIHtcbiAgICBjb25zdCByZXEgPSBuZXcgYmFzZUFwaS5Nb3ZlQXJjUmVxdWVzdCgpO1xuICAgIHJlcS5zZXROYW1lKG5hbWUpO1xuICAgIHJlcS5zZXREaXN0YW5jZU1tKGRpc3RhbmNlX21tKTtcbiAgICByZXEuc2V0TW1QZXJTZWMoc3BlZWRfbW1fcyk7XG4gICAgcmVxLnNldEFuZ2xlRGVnKGFuZ2xlX2RlZyk7XG5cbiAgICByY0xvZ0NvbmRpdGlvbmFsbHkocmVxKTtcbiAgICBiYXNlU2VydmljZS5tb3ZlQXJjKHJlcSwge30sIGNiKTtcbiAgfSxcblxuICBzcGluOiBmdW5jdGlvbihuYW1lLCBhbmdsZV9kZWcsIHNwZWVkX2RlZ19zLCBjYikge1xuICAgIGNvbnN0IHJlcSA9IG5ldyBiYXNlQXBpLlNwaW5SZXF1ZXN0KCk7XG4gICAgcmVxLnNldE5hbWUobmFtZSk7XG4gICAgcmVxLnNldEFuZ2xlRGVnKGFuZ2xlX2RlZyk7XG4gICAgcmVxLnNldERlZ3NQZXJTZWMoc3BlZWRfZGVnX3MpO1xuXG4gICAgcmNMb2dDb25kaXRpb25hbGx5KHJlcSk7XG4gICAgYmFzZVNlcnZpY2Uuc3BpbihyZXEsIHt9LCBjYik7XG4gIH0sXG59XG5cbi8vIExlYXZpbmcgaW4gd2luZG93IHNjb3BlIGZvciB0dW5uaW5nLiBTaG91bGQgYmUgY29uc3Qgb3IgaW4gaW5wdXRzXG53aW5kb3cua2V5Ym9hcmRCYXNlRGVmYXVsdHMgPSB7XG4gIG1heFNwZWVkIDogMzAwLjAsXG4gIG1heEFuZ2xlIDogNDI1LjAsXG4gIGRpc3RSYXRpbyA6IDEwXG59XG5cbi8qXG4gIElucHV0OiBTdGF0ZSBvZiBrZXlzLiBlLmcuIHtzdHJhaWdodCA6IHRydWUsIGJhY2t3YXJkIDogZmFsc2UsIHJpZ2h0IDogZmFsc2UsIGxlZnQ6IGZhbHNlfVxuICBPdXRwdXQ6IGRpc3RhbmNlLCBzcGVlZCwgYW5kIGFuZ2xlIHBhcmFtZXRlcnMgZm9yIE1vdmVBcmNcbiovXG53aW5kb3cuY29tcHV0ZUtleWJvYXJkQmFzZUNvbnRyb2xzID0gZnVuY3Rpb24oa2V5c1ByZXNzZWQpIHtcbiAgbGV0IG1tUGVyU2VjO1xuICBsZXQgYW5nbGVEZWc7XG5cbiAgaWYgKGtleXNQcmVzc2VkLmZvcndhcmQgJiYga2V5c1ByZXNzZWQuYmFja3dhcmQpIHtcbiAgICBtbVBlclNlYyA9IDAuMDtcbiAgfSBlbHNlIGlmIChrZXlzUHJlc3NlZC5mb3J3YXJkKSB7XG4gICAgbW1QZXJTZWMgPSAxLjA7XG4gIH0gZWxzZSBpZiAoa2V5c1ByZXNzZWQuYmFja3dhcmQpIHtcbiAgICBtbVBlclNlYyA9IC0xLjA7XG4gIH0gZWxzZSB7XG4gICAgbW1QZXJTZWMgPSAwLjA7XG4gIH1cblxuICAvLyBBbmdsZVxuICBpZiAoa2V5c1ByZXNzZWQucmlnaHQgJiYga2V5c1ByZXNzZWQubGVmdCkge1xuICAgIGFuZ2xlRGVnID0gMC4wO1xuICB9IGVsc2UgaWYgKGtleXNQcmVzc2VkLnJpZ2h0KSB7XG4gICAgYW5nbGVEZWcgPSAtMS4wO1xuICB9IGVsc2UgaWYgKGtleXNQcmVzc2VkLmxlZnQpIHtcbiAgICBhbmdsZURlZyA9IDEuMDtcbiAgfSBlbHNlIHtcbiAgICBhbmdsZURlZyA9IDAuMDtcbiAgfVxuXG4gIGxldCBkaXN0YW5jZTtcbiAgbGV0IHNwZWVkO1xuICBsZXQgYW5nbGU7XG5cbiAgbGV0IG1vdmVUeXBlOyAvLyBmb3IgbG9nZ2luZyBvbmx5XG4gIGlmIChtbVBlclNlYyA9PSAwICYmIGFuZ2xlRGVnID09IDApIHtcbiAgICBtb3ZlVHlwZSA9ICdTdG9wJztcbiAgICBkaXN0YW5jZSA9IGtleWJvYXJkQmFzZURlZmF1bHRzLm1heFNwZWVkICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMuZGlzdFJhdGlvO1xuICAgIHNwZWVkID0gMC4wO1xuICAgIGFuZ2xlID0gYW5nbGVEZWcgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5tYXhBbmdsZSAqIC0xO1xuICB9IGVsc2UgaWYgKG1tUGVyU2VjID09IDApIHtcbiAgICBtb3ZlVHlwZSA9ICdTcGluJztcbiAgICBkaXN0YW5jZSA9IDA7XG4gICAgc3BlZWQgPSBhbmdsZURlZyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heFNwZWVkO1xuICAgIGFuZ2xlID0gTWF0aC5hYnMoYW5nbGVEZWcgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5tYXhBbmdsZSAqIGtleWJvYXJkQmFzZURlZmF1bHRzLmRpc3RSYXRpbyAvIDIpO1xuICB9IGVsc2UgaWYgKGFuZ2xlRGVnID09IDApIHtcbiAgICBtb3ZlVHlwZSA9ICdTdHJhaWdodCc7XG4gICAgZGlzdGFuY2UgPSBNYXRoLmFicyhtbVBlclNlYyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heFNwZWVkICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMuZGlzdFJhdGlvKTtcbiAgICBzcGVlZCA9IG1tUGVyU2VjICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMubWF4U3BlZWQ7XG4gICAgYW5nbGUgPSBNYXRoLmFicyhhbmdsZURlZyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heEFuZ2xlICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMuZGlzdFJhdGlvKTtcbiAgfSBlbHNlIHtcbiAgICBtb3ZlVHlwZSA9ICdBcmMnO1xuICAgIGRpc3RhbmNlID0gTWF0aC5hYnMobW1QZXJTZWMgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5tYXhTcGVlZCAqIGtleWJvYXJkQmFzZURlZmF1bHRzLmRpc3RSYXRpbyk7XG4gICAgc3BlZWQgPSBtbVBlclNlYyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heFNwZWVkO1xuICAgIGFuZ2xlID0gYW5nbGVEZWcgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5tYXhBbmdsZSAqIGtleWJvYXJkQmFzZURlZmF1bHRzLmRpc3RSYXRpbyAqIDIgLSAxO1xuICB9XG5cbiAgY29uc29sZS5sb2coXCIlczogcyA9ICVmIHwgYSA9ICVmIHwgRGlzdCA9ICVmIHwgU3BlZWQgPSAlZiB8IEFuZ2xlID0gJWZcIiwgbW92ZVR5cGUsIG1tUGVyU2VjLCBhbmdsZURlZywgZGlzdGFuY2UsIHNwZWVkLCBhbmdsZSk7XG4gIHJldHVybiB7ZGlzdGFuY2UsIHNwZWVkLCBhbmdsZX07XG59XG4iXSwic291cmNlUm9vdCI6IiJ9