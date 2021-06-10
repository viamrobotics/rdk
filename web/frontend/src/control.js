const { grpc } = require("@improbable-eng/grpc-web");
window.robotApi = require('proto/api/v1/robot_pb.js');
const { RobotServiceClient } = require('proto/api/v1/robot_pb_service.js');

const url = `${location.protocol}//${location.hostname}${location.port ? ':' + location.port : ''}`;
window.robotService = new RobotServiceClient(url);

window.THREE = require("three/build/three.module.js")
window.pcdLib = require("three/examples/jsm/loaders/PCDLoader.js")
window.orbitLib = require("three/examples/jsm/controls/OrbitControls.js")

