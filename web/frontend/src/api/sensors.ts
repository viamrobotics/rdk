import { sensorsApi, type ResourceName, Client } from '@viamrobotics/sdk';

export const getSensors = (client: Client, name: string) => {
  const request = new sensorsApi.GetSensorsRequest();
  request.setName(name);

  return new Promise<ResourceName[]>((resolve, reject) => {
    client.sensorsService.getSensors(request, (error, response) => {
      if (error) {
        reject(error);
      }

      resolve((response?.toObject().sensorNamesList) ?? []);
    });
  });
};
