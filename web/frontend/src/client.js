window.robotApi = require('proto/api/v1/robot_pb.js');
const { RobotServicePromiseClient } = require('proto/api/v1/robot_grpc_web_pb.js');

const url = `${location.protocol}//${location.hostname}${location.port?':'+location.port:''}`;
window.robotService = new RobotServicePromiseClient(url);

window.THREE = require("three/build/three.module.js")
window.pcdLib = require("three/examples/jsm/loaders/PCDLoader.js")
window.orbitLib = require("three/examples/jsm/controls/OrbitControls.js")

