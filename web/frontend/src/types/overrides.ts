import type { Pose } from '@viamrobotics/sdk';
import type { Timestamp } from 'google-protobuf/google/protobuf/timestamp_pb';

export interface MappingMetadata {
  id: string;
  orgId: string;
  locationId: string;
  robotId: string;
  timeStartSubmitted?: Timestamp.AsObject;
  timeCloudRunJobStarted?: Timestamp.AsObject;
  timeEndSubmitted?: Timestamp.AsObject;
  timeCloudRunJobEnded?: Timestamp.AsObject;
  endStatus: string;
  cloudRunJobId: string;
  viamServerVersion: string;
  mapName: string;
  slamVersion: string;
  config: string;
}

type MappingDetails = {
  mode: 'localize' | 'create' | 'update';
  name?: string;
  version?: string;
};

export interface SLAMOverrides {
  getMappingSessionPCD: (
    sessionId: string
  ) => Promise<{ map: Uint8Array; pose: Pose }>;
  startMappingSession: () => Promise<string>;
  getActiveMappingSession: () => Promise<MappingMetadata | undefined>;
  endMappingSession: (
    sessionId: string
  ) => Promise<{ packageId: string; version: string }>;
  viewMap: (sessionId: string) => void;
  mappingDetails: MappingDetails;
  isMapping: boolean;
}
export interface RCOverrides {
  slam?: SLAMOverrides;
}
