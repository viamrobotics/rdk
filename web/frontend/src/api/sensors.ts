import { Client, sensorsApi } from '@viamrobotics/sdk';

export const getSensors = async (robotClient: Client, name: string) => {
  const request = new sensorsApi.GetSensorsRequest({ name });
  const resp = await robotClient.sensorsService.getSensors(request);
  return resp.sensorNames;
};
