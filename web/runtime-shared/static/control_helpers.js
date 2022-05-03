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

/******/ })()
;
//# sourceMappingURL=data:application/json;charset=utf-8;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly9yZGstd2ViLy4vc3JjL3JjL2NvbnRyb2xfaGVscGVycy5qcyJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7OztBQUFBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLG9DQUFvQztBQUNwQyxHQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBLCtCQUErQjtBQUMvQixHQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQSw0QkFBNEI7QUFDNUIsR0FBRztBQUNIIiwiZmlsZSI6ImNvbnRyb2xfaGVscGVycy5qcyIsInNvdXJjZXNDb250ZW50IjpbIi8qXG5TaW1wbGUgYmFzZSBjb250cm9sIGhlbHBlcnMuIFNob3VsZCBiZSByZXBsYWNlZCBieSBhIHByb3BlciBTREsgb25jZSBhdmFpbGFibGUuXG4qL1xud2luZG93LkJhc2VDb250cm9sSGVscGVyID0ge1xuICBtb3ZlU3RyYWlnaHQ6IGZ1bmN0aW9uKG5hbWUsIGRpc3RhbmNlX21tLCBzcGVlZF9tbV9zLCBjYikge1xuICAgIGNvbnN0IHJlcSA9IG5ldyBiYXNlQXBpLk1vdmVTdHJhaWdodFJlcXVlc3QoKTtcbiAgICByZXEuc2V0TmFtZShuYW1lKTtcbiAgICByZXEuc2V0TW1QZXJTZWMoc3BlZWRfbW1fcyk7XG4gICAgcmVxLnNldERpc3RhbmNlTW0oZGlzdGFuY2VfbW0pO1xuXG4gICAgcmNMb2dDb25kaXRpb25hbGx5KHJlcSk7XG4gICAgYmFzZVNlcnZpY2UubW92ZVN0cmFpZ2h0KHJlcSwge30sIGNiKTtcbiAgfSxcblxuICBtb3ZlQXJjOiBmdW5jdGlvbihuYW1lLCBkaXN0YW5jZV9tbSwgc3BlZWRfbW1fcywgYW5nbGVfZGVnLCBjYikge1xuICAgIGNvbnN0IHJlcSA9IG5ldyBiYXNlQXBpLk1vdmVBcmNSZXF1ZXN0KCk7XG4gICAgcmVxLnNldE5hbWUobmFtZSk7XG4gICAgcmVxLnNldERpc3RhbmNlTW0oZGlzdGFuY2VfbW0pO1xuICAgIHJlcS5zZXRNbVBlclNlYyhzcGVlZF9tbV9zKTtcbiAgICByZXEuc2V0QW5nbGVEZWcoYW5nbGVfZGVnKTtcblxuICAgIHJjTG9nQ29uZGl0aW9uYWxseShyZXEpO1xuICAgIGJhc2VTZXJ2aWNlLm1vdmVBcmMocmVxLCB7fSwgY2IpO1xuICB9LFxuXG4gIHNwaW46IGZ1bmN0aW9uKG5hbWUsIGFuZ2xlX2RlZywgc3BlZWRfZGVnX3MsIGNiKSB7XG4gICAgY29uc3QgcmVxID0gbmV3IGJhc2VBcGkuU3BpblJlcXVlc3QoKTtcbiAgICByZXEuc2V0TmFtZShuYW1lKTtcbiAgICByZXEuc2V0QW5nbGVEZWcoYW5nbGVfZGVnKTtcbiAgICByZXEuc2V0RGVnc1BlclNlYyhzcGVlZF9kZWdfcyk7XG5cbiAgICByY0xvZ0NvbmRpdGlvbmFsbHkocmVxKTtcbiAgICBiYXNlU2VydmljZS5zcGluKHJlcSwge30sIGNiKTtcbiAgfSxcbn1cbiJdLCJzb3VyY2VSb290IjoiIn0=