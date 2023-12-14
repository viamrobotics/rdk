import { type Client, powerSensorApi } from '@viamrobotics/sdk';
import { rcLogConditionally } from '@/lib/log';

export const getVoltage = async (robotClient: Client, name: string) => {
  const req = new powerSensorApi.GetVoltageRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<powerSensorApi.GetVoltageResponse | null>((resolve, reject) => {
    robotClient.powerSensorService.getVoltage(req, (error, res) => {
      if(error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().volts;
};

export const getCurrent = async (robotClient: Client, name: string) => {
  const req = new powerSensorApi.GetCurrentRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<powerSensorApi.GetCurrentResponse | null>((resolve, reject) => {
    robotClient.powerSensorService.getCurrent(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().amperes;
};

export const getPower = async (robotClient: Client, name: string) => {
  const req = new powerSensorApi.GetPowerRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<powerSensorApi.GetPowerResponse | null>((resolve, reject) => {
    robotClient.powerSensorService.getPower(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().watts;
};
