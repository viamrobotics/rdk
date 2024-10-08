import type { Pose, Timestamp } from '@viamrobotics/sdk';

export interface MappingMetadata {
  id: string;
  orgId: string;
  locationId: string;
  robotId: string;
  timeStartSubmitted?: Timestamp;
  timeCloudRunJobStarted?: Timestamp;
  timeEndSubmitted?: Timestamp;
  timeCloudRunJobEnded?: Timestamp;
  endStatus: string;
  cloudRunJobId: string;
  viamServerVersion: string;
  mapName: string;
  slamVersion: string;
  config: string;
}

interface MappingDetails {
  name?: string;
  version?: string;
}

interface SensorInfo {
  name: string;
  type: SensorType;
}
enum SensorType {
  UNSPECIFIED = 0,
  CAMERA = 1,
  MOVEMENT_SENSOR = 2,
}

export interface SLAMOverrides {
  getMappingSessionPCD?: (
    sessionId: string
  ) => Promise<{ map: Uint8Array; pose: Pose }>;
  startMappingSession: (
    mapName: string,
    sensorInfoList: SensorInfo[]
  ) => Promise<string>;
  getActiveMappingSession: () => Promise<MappingMetadata | undefined>;
  endMappingSession: (
    sessionId: string
  ) => Promise<{ packageId: string; version: string }>;
  viewMap: (sessionId: string) => void;
  validateMapName: (mapName: string) => string;
  mappingDetails: MappingDetails;
  isCloudSlam: boolean;
}
export interface RCOverrides {
  slam?: SLAMOverrides;
}
