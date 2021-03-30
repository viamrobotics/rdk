window.robotApi = require('proto/api/v1/robot_pb.js');
const { RobotServicePromiseClient } = require('proto/api/v1/robot_grpc_web_pb.js');

const url = `${location.protocol}//${location.hostname}${location.port?':'+location.port:''}`;
window.robotService = new RobotServicePromiseClient(url);
