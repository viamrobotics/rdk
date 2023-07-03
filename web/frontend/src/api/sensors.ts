import { sensorsApi, Client } from '@viamrobotics/sdk';

export const getSensors = async (client: Client, name: string) => {
  const request = new sensorsApi.GetSensorsRequest();
  request.setName(name);

  const response = await new Promise<sensorsApi.GetSensorsResponse | null>((resolve, reject) => {
    client.sensorsService.getSensors(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().sensorNamesList ?? [];
};
