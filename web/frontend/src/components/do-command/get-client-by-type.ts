import type { DoCommandClient } from '@/api/do-command';
import type { RobotClient } from '@viamrobotics/sdk';

export const getClientByType = (robotClient: RobotClient, type: string) => {
  let client: DoCommandClient;
  switch (type) {
    case 'arm': {
      client = robotClient.armService;
      break;
    }
    case 'base': {
      client = robotClient.baseService;
      break;
    }
    case 'board': {
      client = robotClient.boardService;
      break;
    }
    case 'encoder': {
      client = robotClient.encoderService;
      break;
    }
    case 'gantry': {
      client = robotClient.gantryService;
      break;
    }
    case 'generic': {
      client = robotClient.genericService;
      break;
    }
    case 'gripper': {
      client = robotClient.gripperService;
      break;
    }
    case 'input_controller': {
      client = robotClient.inputControllerService;
      break;
    }
    case 'motion': {
      client = robotClient.motionService;
      break;
    }
    case 'motor': {
      client = robotClient.motorService;
      break;
    }
    case 'movement_sensor': {
      client = robotClient.movementSensorService;
      break;
    }
    case 'navigation': {
      client = robotClient.navigationService;
      break;
    }
    case 'power_sensor': {
      client = robotClient.powerSensorService;
      break;
    }
    case 'sensors': {
      client = robotClient.sensorsService;
      break;
    }
    case 'servo': {
      client = robotClient.servoService;
      break;
    }
    case 'slam': {
      client = robotClient.slamService;
      break;
    }
    case 'vision': {
      client = robotClient.visionService;
      break;
    }
    default: {
      return null;
    }
  }

  return client;
};
