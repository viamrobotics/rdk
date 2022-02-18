// package: proto.api.v1
// file: proto/api/v1/robot.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as google_protobuf_duration_pb from "google-protobuf/google/protobuf/duration_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as google_api_annotations_pb from "../../../google/api/annotations_pb";
import * as proto_api_common_v1_common_pb from "../../../proto/api/common/v1/common_pb";

export class StatusRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): StatusRequest.AsObject;
  static toObject(includeInstance: boolean, msg: StatusRequest): StatusRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: StatusRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): StatusRequest;
  static deserializeBinaryFromReader(message: StatusRequest, reader: jspb.BinaryReader): StatusRequest;
}

export namespace StatusRequest {
  export type AsObject = {
  }
}

export class StatusStreamRequest extends jspb.Message {
  hasEvery(): boolean;
  clearEvery(): void;
  getEvery(): google_protobuf_duration_pb.Duration | undefined;
  setEvery(value?: google_protobuf_duration_pb.Duration): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): StatusStreamRequest.AsObject;
  static toObject(includeInstance: boolean, msg: StatusStreamRequest): StatusStreamRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: StatusStreamRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): StatusStreamRequest;
  static deserializeBinaryFromReader(message: StatusStreamRequest, reader: jspb.BinaryReader): StatusStreamRequest;
}

export namespace StatusStreamRequest {
  export type AsObject = {
    every?: google_protobuf_duration_pb.Duration.AsObject,
  }
}

export class StatusResponse extends jspb.Message {
  hasStatus(): boolean;
  clearStatus(): void;
  getStatus(): Status | undefined;
  setStatus(value?: Status): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): StatusResponse.AsObject;
  static toObject(includeInstance: boolean, msg: StatusResponse): StatusResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: StatusResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): StatusResponse;
  static deserializeBinaryFromReader(message: StatusResponse, reader: jspb.BinaryReader): StatusResponse;
}

export namespace StatusResponse {
  export type AsObject = {
    status?: Status.AsObject,
  }
}

export class StatusStreamResponse extends jspb.Message {
  hasStatus(): boolean;
  clearStatus(): void;
  getStatus(): Status | undefined;
  setStatus(value?: Status): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): StatusStreamResponse.AsObject;
  static toObject(includeInstance: boolean, msg: StatusStreamResponse): StatusStreamResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: StatusStreamResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): StatusStreamResponse;
  static deserializeBinaryFromReader(message: StatusStreamResponse, reader: jspb.BinaryReader): StatusStreamResponse;
}

export namespace StatusStreamResponse {
  export type AsObject = {
    status?: Status.AsObject,
  }
}

export class Status extends jspb.Message {
  getArmsMap(): jspb.Map<string, ArmStatus>;
  clearArmsMap(): void;
  getBasesMap(): jspb.Map<string, boolean>;
  clearBasesMap(): void;
  getGrippersMap(): jspb.Map<string, boolean>;
  clearGrippersMap(): void;
  getBoardsMap(): jspb.Map<string, proto_api_common_v1_common_pb.BoardStatus>;
  clearBoardsMap(): void;
  getCamerasMap(): jspb.Map<string, boolean>;
  clearCamerasMap(): void;
  getSensorsMap(): jspb.Map<string, SensorStatus>;
  clearSensorsMap(): void;
  getFunctionsMap(): jspb.Map<string, boolean>;
  clearFunctionsMap(): void;
  getServosMap(): jspb.Map<string, ServoStatus>;
  clearServosMap(): void;
  getMotorsMap(): jspb.Map<string, MotorStatus>;
  clearMotorsMap(): void;
  getServicesMap(): jspb.Map<string, boolean>;
  clearServicesMap(): void;
  getInputControllersMap(): jspb.Map<string, InputControllerStatus>;
  clearInputControllersMap(): void;
  getGantriesMap(): jspb.Map<string, GantryStatus>;
  clearGantriesMap(): void;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Status.AsObject;
  static toObject(includeInstance: boolean, msg: Status): Status.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Status, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Status;
  static deserializeBinaryFromReader(message: Status, reader: jspb.BinaryReader): Status;
}

export namespace Status {
  export type AsObject = {
    armsMap: Array<[string, ArmStatus.AsObject]>,
    basesMap: Array<[string, boolean]>,
    grippersMap: Array<[string, boolean]>,
    boardsMap: Array<[string, proto_api_common_v1_common_pb.BoardStatus.AsObject]>,
    camerasMap: Array<[string, boolean]>,
    sensorsMap: Array<[string, SensorStatus.AsObject]>,
    functionsMap: Array<[string, boolean]>,
    servosMap: Array<[string, ServoStatus.AsObject]>,
    motorsMap: Array<[string, MotorStatus.AsObject]>,
    servicesMap: Array<[string, boolean]>,
    inputControllersMap: Array<[string, InputControllerStatus.AsObject]>,
    gantriesMap: Array<[string, GantryStatus.AsObject]>,
  }
}

export class ComponentConfig extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getType(): string;
  setType(value: string): void;

  getParent(): string;
  setParent(value: string): void;

  hasPose(): boolean;
  clearPose(): void;
  getPose(): Pose | undefined;
  setPose(value?: Pose): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ComponentConfig.AsObject;
  static toObject(includeInstance: boolean, msg: ComponentConfig): ComponentConfig.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ComponentConfig, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ComponentConfig;
  static deserializeBinaryFromReader(message: ComponentConfig, reader: jspb.BinaryReader): ComponentConfig;
}

export namespace ComponentConfig {
  export type AsObject = {
    name: string,
    type: string,
    parent: string,
    pose?: Pose.AsObject,
  }
}

export class ConfigRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ConfigRequest): ConfigRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ConfigRequest;
  static deserializeBinaryFromReader(message: ConfigRequest, reader: jspb.BinaryReader): ConfigRequest;
}

export namespace ConfigRequest {
  export type AsObject = {
  }
}

export class ConfigResponse extends jspb.Message {
  clearComponentsList(): void;
  getComponentsList(): Array<ComponentConfig>;
  setComponentsList(value: Array<ComponentConfig>): void;
  addComponents(value?: ComponentConfig, index?: number): ComponentConfig;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ConfigResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ConfigResponse): ConfigResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ConfigResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ConfigResponse;
  static deserializeBinaryFromReader(message: ConfigResponse, reader: jspb.BinaryReader): ConfigResponse;
}

export namespace ConfigResponse {
  export type AsObject = {
    componentsList: Array<ComponentConfig.AsObject>,
  }
}

export class DoActionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DoActionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: DoActionRequest): DoActionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DoActionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DoActionRequest;
  static deserializeBinaryFromReader(message: DoActionRequest, reader: jspb.BinaryReader): DoActionRequest;
}

export namespace DoActionRequest {
  export type AsObject = {
    name: string,
  }
}

export class DoActionResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DoActionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: DoActionResponse): DoActionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DoActionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DoActionResponse;
  static deserializeBinaryFromReader(message: DoActionResponse, reader: jspb.BinaryReader): DoActionResponse;
}

export namespace DoActionResponse {
  export type AsObject = {
  }
}

export class GantryStatus extends jspb.Message {
  clearPositionsList(): void;
  getPositionsList(): Array<number>;
  setPositionsList(value: Array<number>): void;
  addPositions(value: number, index?: number): number;

  clearLengthsList(): void;
  getLengthsList(): Array<number>;
  setLengthsList(value: Array<number>): void;
  addLengths(value: number, index?: number): number;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GantryStatus.AsObject;
  static toObject(includeInstance: boolean, msg: GantryStatus): GantryStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GantryStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GantryStatus;
  static deserializeBinaryFromReader(message: GantryStatus, reader: jspb.BinaryReader): GantryStatus;
}

export namespace GantryStatus {
  export type AsObject = {
    positionsList: Array<number>,
    lengthsList: Array<number>,
  }
}

export class ArmStatus extends jspb.Message {
  hasGridPosition(): boolean;
  clearGridPosition(): void;
  getGridPosition(): Pose | undefined;
  setGridPosition(value?: Pose): void;

  hasJointPositions(): boolean;
  clearJointPositions(): void;
  getJointPositions(): JointPositions | undefined;
  setJointPositions(value?: JointPositions): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmStatus.AsObject;
  static toObject(includeInstance: boolean, msg: ArmStatus): ArmStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmStatus;
  static deserializeBinaryFromReader(message: ArmStatus, reader: jspb.BinaryReader): ArmStatus;
}

export namespace ArmStatus {
  export type AsObject = {
    gridPosition?: Pose.AsObject,
    jointPositions?: JointPositions.AsObject,
  }
}

export class Pose extends jspb.Message {
  getX(): number;
  setX(value: number): void;

  getY(): number;
  setY(value: number): void;

  getZ(): number;
  setZ(value: number): void;

  getOX(): number;
  setOX(value: number): void;

  getOY(): number;
  setOY(value: number): void;

  getOZ(): number;
  setOZ(value: number): void;

  getTheta(): number;
  setTheta(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Pose.AsObject;
  static toObject(includeInstance: boolean, msg: Pose): Pose.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Pose, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Pose;
  static deserializeBinaryFromReader(message: Pose, reader: jspb.BinaryReader): Pose;
}

export namespace Pose {
  export type AsObject = {
    x: number,
    y: number,
    z: number,
    oX: number,
    oY: number,
    oZ: number,
    theta: number,
  }
}

export class JointPositions extends jspb.Message {
  clearDegreesList(): void;
  getDegreesList(): Array<number>;
  setDegreesList(value: Array<number>): void;
  addDegrees(value: number, index?: number): number;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JointPositions.AsObject;
  static toObject(includeInstance: boolean, msg: JointPositions): JointPositions.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JointPositions, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JointPositions;
  static deserializeBinaryFromReader(message: JointPositions, reader: jspb.BinaryReader): JointPositions;
}

export namespace JointPositions {
  export type AsObject = {
    degreesList: Array<number>,
  }
}

export class FrameConfig extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  hasPose(): boolean;
  clearPose(): void;
  getPose(): Pose | undefined;
  setPose(value?: Pose): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): FrameConfig.AsObject;
  static toObject(includeInstance: boolean, msg: FrameConfig): FrameConfig.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: FrameConfig, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): FrameConfig;
  static deserializeBinaryFromReader(message: FrameConfig, reader: jspb.BinaryReader): FrameConfig;
}

export namespace FrameConfig {
  export type AsObject = {
    parent: string,
    pose?: Pose.AsObject,
  }
}

export class FrameSystemConfig extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasFrameConfig(): boolean;
  clearFrameConfig(): void;
  getFrameConfig(): FrameConfig | undefined;
  setFrameConfig(value?: FrameConfig): void;

  getModelJson(): Uint8Array | string;
  getModelJson_asU8(): Uint8Array;
  getModelJson_asB64(): string;
  setModelJson(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): FrameSystemConfig.AsObject;
  static toObject(includeInstance: boolean, msg: FrameSystemConfig): FrameSystemConfig.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: FrameSystemConfig, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): FrameSystemConfig;
  static deserializeBinaryFromReader(message: FrameSystemConfig, reader: jspb.BinaryReader): FrameSystemConfig;
}

export namespace FrameSystemConfig {
  export type AsObject = {
    name: string,
    frameConfig?: FrameConfig.AsObject,
    modelJson: Uint8Array | string,
  }
}

export class FrameServiceConfigRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): FrameServiceConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: FrameServiceConfigRequest): FrameServiceConfigRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: FrameServiceConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): FrameServiceConfigRequest;
  static deserializeBinaryFromReader(message: FrameServiceConfigRequest, reader: jspb.BinaryReader): FrameServiceConfigRequest;
}

export namespace FrameServiceConfigRequest {
  export type AsObject = {
  }
}

export class FrameServiceConfigResponse extends jspb.Message {
  clearFrameSystemConfigsList(): void;
  getFrameSystemConfigsList(): Array<FrameSystemConfig>;
  setFrameSystemConfigsList(value: Array<FrameSystemConfig>): void;
  addFrameSystemConfigs(value?: FrameSystemConfig, index?: number): FrameSystemConfig;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): FrameServiceConfigResponse.AsObject;
  static toObject(includeInstance: boolean, msg: FrameServiceConfigResponse): FrameServiceConfigResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: FrameServiceConfigResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): FrameServiceConfigResponse;
  static deserializeBinaryFromReader(message: FrameServiceConfigResponse, reader: jspb.BinaryReader): FrameServiceConfigResponse;
}

export namespace FrameServiceConfigResponse {
  export type AsObject = {
    frameSystemConfigsList: Array<FrameSystemConfig.AsObject>,
  }
}

export class Vector3 extends jspb.Message {
  getX(): number;
  setX(value: number): void;

  getY(): number;
  setY(value: number): void;

  getZ(): number;
  setZ(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Vector3.AsObject;
  static toObject(includeInstance: boolean, msg: Vector3): Vector3.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Vector3, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Vector3;
  static deserializeBinaryFromReader(message: Vector3, reader: jspb.BinaryReader): Vector3;
}

export namespace Vector3 {
  export type AsObject = {
    x: number,
    y: number,
    z: number,
  }
}

export class SensorStatus extends jspb.Message {
  getType(): string;
  setType(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SensorStatus.AsObject;
  static toObject(includeInstance: boolean, msg: SensorStatus): SensorStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SensorStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SensorStatus;
  static deserializeBinaryFromReader(message: SensorStatus, reader: jspb.BinaryReader): SensorStatus;
}

export namespace SensorStatus {
  export type AsObject = {
    type: string,
  }
}

export class SensorReadingsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SensorReadingsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SensorReadingsRequest): SensorReadingsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SensorReadingsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SensorReadingsRequest;
  static deserializeBinaryFromReader(message: SensorReadingsRequest, reader: jspb.BinaryReader): SensorReadingsRequest;
}

export namespace SensorReadingsRequest {
  export type AsObject = {
    name: string,
  }
}

export class SensorReadingsResponse extends jspb.Message {
  clearReadingsList(): void;
  getReadingsList(): Array<google_protobuf_struct_pb.Value>;
  setReadingsList(value: Array<google_protobuf_struct_pb.Value>): void;
  addReadings(value?: google_protobuf_struct_pb.Value, index?: number): google_protobuf_struct_pb.Value;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SensorReadingsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: SensorReadingsResponse): SensorReadingsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SensorReadingsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SensorReadingsResponse;
  static deserializeBinaryFromReader(message: SensorReadingsResponse, reader: jspb.BinaryReader): SensorReadingsResponse;
}

export namespace SensorReadingsResponse {
  export type AsObject = {
    readingsList: Array<google_protobuf_struct_pb.Value.AsObject>,
  }
}

export class ExecuteFunctionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ExecuteFunctionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ExecuteFunctionRequest): ExecuteFunctionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ExecuteFunctionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ExecuteFunctionRequest;
  static deserializeBinaryFromReader(message: ExecuteFunctionRequest, reader: jspb.BinaryReader): ExecuteFunctionRequest;
}

export namespace ExecuteFunctionRequest {
  export type AsObject = {
    name: string,
  }
}

export class ExecuteFunctionResponse extends jspb.Message {
  clearResultsList(): void;
  getResultsList(): Array<google_protobuf_struct_pb.Value>;
  setResultsList(value: Array<google_protobuf_struct_pb.Value>): void;
  addResults(value?: google_protobuf_struct_pb.Value, index?: number): google_protobuf_struct_pb.Value;

  getStdOut(): string;
  setStdOut(value: string): void;

  getStdErr(): string;
  setStdErr(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ExecuteFunctionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ExecuteFunctionResponse): ExecuteFunctionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ExecuteFunctionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ExecuteFunctionResponse;
  static deserializeBinaryFromReader(message: ExecuteFunctionResponse, reader: jspb.BinaryReader): ExecuteFunctionResponse;
}

export namespace ExecuteFunctionResponse {
  export type AsObject = {
    resultsList: Array<google_protobuf_struct_pb.Value.AsObject>,
    stdOut: string,
    stdErr: string,
  }
}

export class ExecuteSourceRequest extends jspb.Message {
  getSource(): string;
  setSource(value: string): void;

  getEngine(): string;
  setEngine(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ExecuteSourceRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ExecuteSourceRequest): ExecuteSourceRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ExecuteSourceRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ExecuteSourceRequest;
  static deserializeBinaryFromReader(message: ExecuteSourceRequest, reader: jspb.BinaryReader): ExecuteSourceRequest;
}

export namespace ExecuteSourceRequest {
  export type AsObject = {
    source: string,
    engine: string,
  }
}

export class ExecuteSourceResponse extends jspb.Message {
  clearResultsList(): void;
  getResultsList(): Array<google_protobuf_struct_pb.Value>;
  setResultsList(value: Array<google_protobuf_struct_pb.Value>): void;
  addResults(value?: google_protobuf_struct_pb.Value, index?: number): google_protobuf_struct_pb.Value;

  getStdOut(): string;
  setStdOut(value: string): void;

  getStdErr(): string;
  setStdErr(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ExecuteSourceResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ExecuteSourceResponse): ExecuteSourceResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ExecuteSourceResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ExecuteSourceResponse;
  static deserializeBinaryFromReader(message: ExecuteSourceResponse, reader: jspb.BinaryReader): ExecuteSourceResponse;
}

export namespace ExecuteSourceResponse {
  export type AsObject = {
    resultsList: Array<google_protobuf_struct_pb.Value.AsObject>,
    stdOut: string,
    stdErr: string,
  }
}

export class MotorStatus extends jspb.Message {
  getOn(): boolean;
  setOn(value: boolean): void;

  getPositionSupported(): boolean;
  setPositionSupported(value: boolean): void;

  getPosition(): number;
  setPosition(value: number): void;

  hasPidConfig(): boolean;
  clearPidConfig(): void;
  getPidConfig(): google_protobuf_struct_pb.Struct | undefined;
  setPidConfig(value?: google_protobuf_struct_pb.Struct): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorStatus.AsObject;
  static toObject(includeInstance: boolean, msg: MotorStatus): MotorStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorStatus;
  static deserializeBinaryFromReader(message: MotorStatus, reader: jspb.BinaryReader): MotorStatus;
}

export namespace MotorStatus {
  export type AsObject = {
    on: boolean,
    positionSupported: boolean,
    position: number,
    pidConfig?: google_protobuf_struct_pb.Struct.AsObject,
  }
}

export class ServoStatus extends jspb.Message {
  getAngle(): number;
  setAngle(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServoStatus.AsObject;
  static toObject(includeInstance: boolean, msg: ServoStatus): ServoStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServoStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServoStatus;
  static deserializeBinaryFromReader(message: ServoStatus, reader: jspb.BinaryReader): ServoStatus;
}

export namespace ServoStatus {
  export type AsObject = {
    angle: number,
  }
}

export class ResourceRunCommandRequest extends jspb.Message {
  getResourceName(): string;
  setResourceName(value: string): void;

  getCommandName(): string;
  setCommandName(value: string): void;

  hasArgs(): boolean;
  clearArgs(): void;
  getArgs(): google_protobuf_struct_pb.Struct | undefined;
  setArgs(value?: google_protobuf_struct_pb.Struct): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ResourceRunCommandRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ResourceRunCommandRequest): ResourceRunCommandRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ResourceRunCommandRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ResourceRunCommandRequest;
  static deserializeBinaryFromReader(message: ResourceRunCommandRequest, reader: jspb.BinaryReader): ResourceRunCommandRequest;
}

export namespace ResourceRunCommandRequest {
  export type AsObject = {
    resourceName: string,
    commandName: string,
    args?: google_protobuf_struct_pb.Struct.AsObject,
  }
}

export class ResourceRunCommandResponse extends jspb.Message {
  hasResult(): boolean;
  clearResult(): void;
  getResult(): google_protobuf_struct_pb.Struct | undefined;
  setResult(value?: google_protobuf_struct_pb.Struct): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ResourceRunCommandResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ResourceRunCommandResponse): ResourceRunCommandResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ResourceRunCommandResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ResourceRunCommandResponse;
  static deserializeBinaryFromReader(message: ResourceRunCommandResponse, reader: jspb.BinaryReader): ResourceRunCommandResponse;
}

export namespace ResourceRunCommandResponse {
  export type AsObject = {
    result?: google_protobuf_struct_pb.Struct.AsObject,
  }
}

export class NavigationServiceModeRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceModeRequest.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceModeRequest): NavigationServiceModeRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceModeRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceModeRequest;
  static deserializeBinaryFromReader(message: NavigationServiceModeRequest, reader: jspb.BinaryReader): NavigationServiceModeRequest;
}

export namespace NavigationServiceModeRequest {
  export type AsObject = {
  }
}

export class NavigationServiceModeResponse extends jspb.Message {
  getMode(): NavigationServiceModeMap[keyof NavigationServiceModeMap];
  setMode(value: NavigationServiceModeMap[keyof NavigationServiceModeMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceModeResponse.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceModeResponse): NavigationServiceModeResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceModeResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceModeResponse;
  static deserializeBinaryFromReader(message: NavigationServiceModeResponse, reader: jspb.BinaryReader): NavigationServiceModeResponse;
}

export namespace NavigationServiceModeResponse {
  export type AsObject = {
    mode: NavigationServiceModeMap[keyof NavigationServiceModeMap],
  }
}

export class NavigationServiceSetModeRequest extends jspb.Message {
  getMode(): NavigationServiceModeMap[keyof NavigationServiceModeMap];
  setMode(value: NavigationServiceModeMap[keyof NavigationServiceModeMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceSetModeRequest.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceSetModeRequest): NavigationServiceSetModeRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceSetModeRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceSetModeRequest;
  static deserializeBinaryFromReader(message: NavigationServiceSetModeRequest, reader: jspb.BinaryReader): NavigationServiceSetModeRequest;
}

export namespace NavigationServiceSetModeRequest {
  export type AsObject = {
    mode: NavigationServiceModeMap[keyof NavigationServiceModeMap],
  }
}

export class NavigationServiceSetModeResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceSetModeResponse.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceSetModeResponse): NavigationServiceSetModeResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceSetModeResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceSetModeResponse;
  static deserializeBinaryFromReader(message: NavigationServiceSetModeResponse, reader: jspb.BinaryReader): NavigationServiceSetModeResponse;
}

export namespace NavigationServiceSetModeResponse {
  export type AsObject = {
  }
}

export class NavigationServiceWaypoint extends jspb.Message {
  getId(): string;
  setId(value: string): void;

  hasLocation(): boolean;
  clearLocation(): void;
  getLocation(): proto_api_common_v1_common_pb.GeoPoint | undefined;
  setLocation(value?: proto_api_common_v1_common_pb.GeoPoint): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceWaypoint.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceWaypoint): NavigationServiceWaypoint.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceWaypoint, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceWaypoint;
  static deserializeBinaryFromReader(message: NavigationServiceWaypoint, reader: jspb.BinaryReader): NavigationServiceWaypoint;
}

export namespace NavigationServiceWaypoint {
  export type AsObject = {
    id: string,
    location?: proto_api_common_v1_common_pb.GeoPoint.AsObject,
  }
}

export class NavigationServiceLocationRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceLocationRequest.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceLocationRequest): NavigationServiceLocationRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceLocationRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceLocationRequest;
  static deserializeBinaryFromReader(message: NavigationServiceLocationRequest, reader: jspb.BinaryReader): NavigationServiceLocationRequest;
}

export namespace NavigationServiceLocationRequest {
  export type AsObject = {
  }
}

export class NavigationServiceLocationResponse extends jspb.Message {
  hasLocation(): boolean;
  clearLocation(): void;
  getLocation(): proto_api_common_v1_common_pb.GeoPoint | undefined;
  setLocation(value?: proto_api_common_v1_common_pb.GeoPoint): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceLocationResponse.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceLocationResponse): NavigationServiceLocationResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceLocationResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceLocationResponse;
  static deserializeBinaryFromReader(message: NavigationServiceLocationResponse, reader: jspb.BinaryReader): NavigationServiceLocationResponse;
}

export namespace NavigationServiceLocationResponse {
  export type AsObject = {
    location?: proto_api_common_v1_common_pb.GeoPoint.AsObject,
  }
}

export class NavigationServiceWaypointsRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceWaypointsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceWaypointsRequest): NavigationServiceWaypointsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceWaypointsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceWaypointsRequest;
  static deserializeBinaryFromReader(message: NavigationServiceWaypointsRequest, reader: jspb.BinaryReader): NavigationServiceWaypointsRequest;
}

export namespace NavigationServiceWaypointsRequest {
  export type AsObject = {
  }
}

export class NavigationServiceWaypointsResponse extends jspb.Message {
  clearWaypointsList(): void;
  getWaypointsList(): Array<NavigationServiceWaypoint>;
  setWaypointsList(value: Array<NavigationServiceWaypoint>): void;
  addWaypoints(value?: NavigationServiceWaypoint, index?: number): NavigationServiceWaypoint;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceWaypointsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceWaypointsResponse): NavigationServiceWaypointsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceWaypointsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceWaypointsResponse;
  static deserializeBinaryFromReader(message: NavigationServiceWaypointsResponse, reader: jspb.BinaryReader): NavigationServiceWaypointsResponse;
}

export namespace NavigationServiceWaypointsResponse {
  export type AsObject = {
    waypointsList: Array<NavigationServiceWaypoint.AsObject>,
  }
}

export class NavigationServiceAddWaypointRequest extends jspb.Message {
  hasLocation(): boolean;
  clearLocation(): void;
  getLocation(): proto_api_common_v1_common_pb.GeoPoint | undefined;
  setLocation(value?: proto_api_common_v1_common_pb.GeoPoint): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceAddWaypointRequest.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceAddWaypointRequest): NavigationServiceAddWaypointRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceAddWaypointRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceAddWaypointRequest;
  static deserializeBinaryFromReader(message: NavigationServiceAddWaypointRequest, reader: jspb.BinaryReader): NavigationServiceAddWaypointRequest;
}

export namespace NavigationServiceAddWaypointRequest {
  export type AsObject = {
    location?: proto_api_common_v1_common_pb.GeoPoint.AsObject,
  }
}

export class NavigationServiceAddWaypointResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceAddWaypointResponse.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceAddWaypointResponse): NavigationServiceAddWaypointResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceAddWaypointResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceAddWaypointResponse;
  static deserializeBinaryFromReader(message: NavigationServiceAddWaypointResponse, reader: jspb.BinaryReader): NavigationServiceAddWaypointResponse;
}

export namespace NavigationServiceAddWaypointResponse {
  export type AsObject = {
  }
}

export class NavigationServiceRemoveWaypointRequest extends jspb.Message {
  getId(): string;
  setId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceRemoveWaypointRequest.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceRemoveWaypointRequest): NavigationServiceRemoveWaypointRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceRemoveWaypointRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceRemoveWaypointRequest;
  static deserializeBinaryFromReader(message: NavigationServiceRemoveWaypointRequest, reader: jspb.BinaryReader): NavigationServiceRemoveWaypointRequest;
}

export namespace NavigationServiceRemoveWaypointRequest {
  export type AsObject = {
    id: string,
  }
}

export class NavigationServiceRemoveWaypointResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NavigationServiceRemoveWaypointResponse.AsObject;
  static toObject(includeInstance: boolean, msg: NavigationServiceRemoveWaypointResponse): NavigationServiceRemoveWaypointResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NavigationServiceRemoveWaypointResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NavigationServiceRemoveWaypointResponse;
  static deserializeBinaryFromReader(message: NavigationServiceRemoveWaypointResponse, reader: jspb.BinaryReader): NavigationServiceRemoveWaypointResponse;
}

export namespace NavigationServiceRemoveWaypointResponse {
  export type AsObject = {
  }
}

export class ObjectManipulationServiceDoGrabRequest extends jspb.Message {
  getCameraName(): string;
  setCameraName(value: string): void;

  hasCameraPoint(): boolean;
  clearCameraPoint(): void;
  getCameraPoint(): Vector3 | undefined;
  setCameraPoint(value?: Vector3): void;

  getGripperName(): string;
  setGripperName(value: string): void;

  getArmName(): string;
  setArmName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ObjectManipulationServiceDoGrabRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ObjectManipulationServiceDoGrabRequest): ObjectManipulationServiceDoGrabRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ObjectManipulationServiceDoGrabRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ObjectManipulationServiceDoGrabRequest;
  static deserializeBinaryFromReader(message: ObjectManipulationServiceDoGrabRequest, reader: jspb.BinaryReader): ObjectManipulationServiceDoGrabRequest;
}

export namespace ObjectManipulationServiceDoGrabRequest {
  export type AsObject = {
    cameraName: string,
    cameraPoint?: Vector3.AsObject,
    gripperName: string,
    armName: string,
  }
}

export class ObjectManipulationServiceDoGrabResponse extends jspb.Message {
  getHasGrabbed(): boolean;
  setHasGrabbed(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ObjectManipulationServiceDoGrabResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ObjectManipulationServiceDoGrabResponse): ObjectManipulationServiceDoGrabResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ObjectManipulationServiceDoGrabResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ObjectManipulationServiceDoGrabResponse;
  static deserializeBinaryFromReader(message: ObjectManipulationServiceDoGrabResponse, reader: jspb.BinaryReader): ObjectManipulationServiceDoGrabResponse;
}

export namespace ObjectManipulationServiceDoGrabResponse {
  export type AsObject = {
    hasGrabbed: boolean,
  }
}

export class InputControllerEvent extends jspb.Message {
  hasTime(): boolean;
  clearTime(): void;
  getTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setTime(value?: google_protobuf_timestamp_pb.Timestamp): void;

  getEvent(): string;
  setEvent(value: string): void;

  getControl(): string;
  setControl(value: string): void;

  getValue(): number;
  setValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerEvent.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerEvent): InputControllerEvent.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerEvent, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerEvent;
  static deserializeBinaryFromReader(message: InputControllerEvent, reader: jspb.BinaryReader): InputControllerEvent;
}

export namespace InputControllerEvent {
  export type AsObject = {
    time?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    event: string,
    control: string,
    value: number,
  }
}

export class InputControllerStatus extends jspb.Message {
  clearEventsList(): void;
  getEventsList(): Array<InputControllerEvent>;
  setEventsList(value: Array<InputControllerEvent>): void;
  addEvents(value?: InputControllerEvent, index?: number): InputControllerEvent;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerStatus.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerStatus): InputControllerStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerStatus;
  static deserializeBinaryFromReader(message: InputControllerStatus, reader: jspb.BinaryReader): InputControllerStatus;
}

export namespace InputControllerStatus {
  export type AsObject = {
    eventsList: Array<InputControllerEvent.AsObject>,
  }
}

export interface NavigationServiceModeMap {
  NAVIGATION_SERVICE_MODE_UNSPECIFIED: 0;
  NAVIGATION_SERVICE_MODE_MANUAL: 1;
  NAVIGATION_SERVICE_MODE_WAYPOINT: 2;
}

export const NavigationServiceMode: NavigationServiceModeMap;

