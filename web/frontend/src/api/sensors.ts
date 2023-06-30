import { grpc } from '@improbable-eng/grpc-web';
import { sensorsApi, type ResourceName } from '@viamrobotics/sdk';
import { filterSubtype } from '@/lib/resource';
import { useClient } from '@/hooks/use-client';

const { client, resources } = useClient();

export const getSensors = () => {
  const sensorsName = filterSubtype(resources.current, 'sensors', { remote: false })[0]?.name;

  if (sensorsName === undefined) {
    return;
  }

  const request = new sensorsApi.GetSensorsRequest();
  request.setName(sensorsName);

  return new Promise<ResourceName[]>((resolve, reject) => {
    client.current.sensorsService.getSensors(request, new grpc.Metadata(), (error, response) => {
      if (error) {
        reject(error);
      }
      resolve(response?.toObject().sensorNamesList ?? []);
    });
  });
};
