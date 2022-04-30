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
//# sourceMappingURL=data:application/json;charset=utf-8;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly9yZGstd2ViLy4vc3JjL3JjL2NvbnRyb2xfaGVscGVycy5qcyJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7OztBQUFBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLG9DQUFvQztBQUNwQyxHQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLCtCQUErQjtBQUMvQixHQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQSw0QkFBNEI7QUFDNUIsR0FBRztBQUNIOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLDhCQUE4QjtBQUM5QjtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQSxHQUFHO0FBQ0g7QUFDQSxHQUFHO0FBQ0g7QUFDQSxHQUFHO0FBQ0g7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQSxHQUFHO0FBQ0g7QUFDQSxHQUFHO0FBQ0g7QUFDQSxHQUFHO0FBQ0g7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUEsZUFBZTtBQUNmO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxHQUFHO0FBQ0g7QUFDQTtBQUNBO0FBQ0E7QUFDQSxHQUFHO0FBQ0g7QUFDQTtBQUNBO0FBQ0E7QUFDQSxHQUFHO0FBQ0g7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLFVBQVU7QUFDVixDIiwiZmlsZSI6ImNvbnRyb2xfaGVscGVycy5qcyIsInNvdXJjZXNDb250ZW50IjpbIi8qXG5TaW1wbGUgYmFzZSBjb250cm9sIGhlbHBlcnMuIFNob3VsZCBiZSByZXBsYWNlZCBieSBhIHByb3BlciBTREsgb25jZSBhdmFpbGFibGUuXG4qL1xud2luZG93LkJhc2VDb250cm9sSGVscGVyID0ge1xuICBtb3ZlU3RyYWlnaHQ6IGZ1bmN0aW9uKG5hbWUsIGRpc3RhbmNlX21tLCBzcGVlZF9tbV9zLCBjYikge1xuICAgIGNvbnN0IHJlcSA9IG5ldyBiYXNlQXBpLk1vdmVTdHJhaWdodFJlcXVlc3QoKTtcbiAgICByZXEuc2V0TmFtZShuYW1lKTtcbiAgICByZXEuc2V0TW1QZXJTZWMoc3BlZWRfbW1fcyk7XG4gICAgcmVxLnNldERpc3RhbmNlTW0oZGlzdGFuY2VfbW0pO1xuXG4gICAgcmNMb2dDb25kaXRpb25hbGx5KHJlcSk7XG4gICAgYmFzZVNlcnZpY2UubW92ZVN0cmFpZ2h0KHJlcSwge30sIGNiKTtcbiAgfSxcblxuICBtb3ZlQXJjOiBmdW5jdGlvbihuYW1lLCBkaXN0YW5jZV9tbSwgc3BlZWRfbW1fcywgYW5nbGVfZGVnLCBjYikge1xuICAgIGNvbnN0IHJlcSA9IG5ldyBiYXNlQXBpLk1vdmVBcmNSZXF1ZXN0KCk7XG4gICAgcmVxLnNldE5hbWUobmFtZSk7XG4gICAgcmVxLnNldERpc3RhbmNlTW0oZGlzdGFuY2VfbW0pO1xuICAgIHJlcS5zZXRNbVBlclNlYyhzcGVlZF9tbV9zKTtcbiAgICByZXEuc2V0QW5nbGVEZWcoYW5nbGVfZGVnKTtcblxuICAgIHJjTG9nQ29uZGl0aW9uYWxseShyZXEpO1xuICAgIGJhc2VTZXJ2aWNlLm1vdmVBcmMocmVxLCB7fSwgY2IpO1xuICB9LFxuXG4gIHNwaW46IGZ1bmN0aW9uKG5hbWUsIGFuZ2xlX2RlZywgc3BlZWRfZGVnX3MsIGNiKSB7XG4gICAgY29uc3QgcmVxID0gbmV3IGJhc2VBcGkuU3BpblJlcXVlc3QoKTtcbiAgICByZXEuc2V0TmFtZShuYW1lKTtcbiAgICByZXEuc2V0QW5nbGVEZWcoYW5nbGVfZGVnKTtcbiAgICByZXEuc2V0RGVnc1BlclNlYyhzcGVlZF9kZWdfcyk7XG5cbiAgICByY0xvZ0NvbmRpdGlvbmFsbHkocmVxKTtcbiAgICBiYXNlU2VydmljZS5zcGluKHJlcSwge30sIGNiKTtcbiAgfSxcbn1cblxuLy8gTGVhdmluZyBpbiB3aW5kb3cgc2NvcGUgZm9yIHR1bm5pbmcuIFNob3VsZCBiZSBjb25zdCBvciBpbiBpbnB1dHNcbndpbmRvdy5rZXlib2FyZEJhc2VEZWZhdWx0cyA9IHtcbiAgbWF4U3BlZWQgOiAzMDAuMCxcblx0bWF4QW5nbGUgOiA0MjUuMCxcblx0ZGlzdFJhdGlvIDogMTBcbn1cblxuLypcbiAgSW5wdXQ6IFN0YXRlIG9mIGtleXMuIGUuZy4ge3N0cmFpZ2h0IDogdHJ1ZSwgYmFja3dhcmQgOiBmYWxzZSwgcmlnaHQgOiBmYWxzZSwgbGVmdDogZmFsc2V9XG4gIE91dHB1dDogZGlzdGFuY2UsIHNwZWVkLCBhbmQgYW5nbGUgcGFyYW1ldGVycyBmb3IgTW92ZUFyY1xuKi9cbndpbmRvdy5jb21wdXRlS2V5Ym9hcmRCYXNlQ29udHJvbHMgPSBmdW5jdGlvbihrZXlzUHJlc3NlZCkge1xuICBsZXQgbW1QZXJTZWM7XG4gIGxldCBhbmdsZURlZztcblxuICBpZiAoa2V5c1ByZXNzZWQuZm9yd2FyZCAmJiBrZXlzUHJlc3NlZC5iYWNrd2FyZCkge1xuICAgIG1tUGVyU2VjID0gMC4wO1xuICB9IGVsc2UgaWYgKGtleXNQcmVzc2VkLmZvcndhcmQpIHtcbiAgICBtbVBlclNlYyA9IDEuMDtcbiAgfSBlbHNlIGlmIChrZXlzUHJlc3NlZC5iYWNrd2FyZCkge1xuICAgIG1tUGVyU2VjID0gLTEuMDtcbiAgfSBlbHNlIHtcbiAgICBtbVBlclNlYyA9IDAuMDtcbiAgfVxuXG4gIC8vIEFuZ2xlXG4gIGlmIChrZXlzUHJlc3NlZC5yaWdodCAmJiBrZXlzUHJlc3NlZC5sZWZ0KSB7XG4gICAgYW5nbGVEZWcgPSAwLjA7XG4gIH0gZWxzZSBpZiAoa2V5c1ByZXNzZWQucmlnaHQpIHtcbiAgICBhbmdsZURlZyA9IC0xLjA7XG4gIH0gZWxzZSBpZiAoa2V5c1ByZXNzZWQubGVmdCkge1xuICAgIGFuZ2xlRGVnID0gMS4wO1xuICB9IGVsc2Uge1xuICAgIGFuZ2xlRGVnID0gMC4wO1xuICB9XG5cbiAgbGV0IGRpc3RhbmNlO1xuICBsZXQgc3BlZWQ7XG4gIGxldCBhbmdsZTtcblxuICBsZXQgbW92ZVR5cGU7IC8vIGZvciBsb2dnaW5nIG9ubHlcbiAgaWYgKG1tUGVyU2VjID09IDAgJiYgYW5nbGVEZWcgPT0gMCkge1xuICAgIG1vdmVUeXBlID0gJ1N0b3AnO1xuICAgIGRpc3RhbmNlID0ga2V5Ym9hcmRCYXNlRGVmYXVsdHMubWF4U3BlZWQgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5kaXN0UmF0aW87XG4gICAgc3BlZWQgPSAwLjA7XG4gICAgYW5nbGUgPSBhbmdsZURlZyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heEFuZ2xlICogLTE7XG4gIH0gZWxzZSBpZiAobW1QZXJTZWMgPT0gMCkge1xuICAgIG1vdmVUeXBlID0gJ1NwaW4nO1xuICAgIGRpc3RhbmNlID0gMDtcbiAgICBzcGVlZCA9IGFuZ2xlRGVnICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMubWF4U3BlZWQ7XG4gICAgYW5nbGUgPSBNYXRoLmFicyhhbmdsZURlZyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heEFuZ2xlICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMuZGlzdFJhdGlvIC8gMik7XG4gIH0gZWxzZSBpZiAoYW5nbGVEZWcgPT0gMCkge1xuICAgIG1vdmVUeXBlID0gJ1N0cmFpZ2h0JztcbiAgICBkaXN0YW5jZSA9IE1hdGguYWJzKG1tUGVyU2VjICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMubWF4U3BlZWQgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5kaXN0UmF0aW8pO1xuICAgIHNwZWVkID0gbW1QZXJTZWMgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5tYXhTcGVlZDtcbiAgICBhbmdsZSA9IE1hdGguYWJzKGFuZ2xlRGVnICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMubWF4QW5nbGUgKiBrZXlib2FyZEJhc2VEZWZhdWx0cy5kaXN0UmF0aW8pO1xuICB9IGVsc2Uge1xuICAgIG1vdmVUeXBlID0gJ0FyYyc7XG4gICAgZGlzdGFuY2UgPSBNYXRoLmFicyhtbVBlclNlYyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heFNwZWVkICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMuZGlzdFJhdGlvKTtcbiAgICBzcGVlZCA9IG1tUGVyU2VjICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMubWF4U3BlZWQ7XG4gICAgYW5nbGUgPSBhbmdsZURlZyAqIGtleWJvYXJkQmFzZURlZmF1bHRzLm1heEFuZ2xlICoga2V5Ym9hcmRCYXNlRGVmYXVsdHMuZGlzdFJhdGlvICogMiAtIDE7XG4gIH1cblxuICBjb25zb2xlLmxvZyhcIiVzOiBzID0gJWYgfCBhID0gJWYgfCBEaXN0ID0gJWYgfCBTcGVlZCA9ICVmIHwgQW5nbGUgPSAlZlwiLCBtb3ZlVHlwZSwgbW1QZXJTZWMsIGFuZ2xlRGVnLCBkaXN0YW5jZSwgc3BlZWQsIGFuZ2xlKTtcbiAgcmV0dXJuIHtkaXN0YW5jZSwgc3BlZWQsIGFuZ2xlfTtcbn0iXSwic291cmNlUm9vdCI6IiJ9