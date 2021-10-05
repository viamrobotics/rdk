// package: proto.api.v1
// file: proto/api/v1/robot.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as google_protobuf_duration_pb from "google-protobuf/google/protobuf/duration_pb";
import * as google_api_annotations_pb from "../../../google/api/annotations_pb";
import * as google_api_httpbody_pb from "../../../google/api/httpbody_pb";

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
  getBoardsMap(): jspb.Map<string, BoardStatus>;
  clearBoardsMap(): void;
  getCamerasMap(): jspb.Map<string, boolean>;
  clearCamerasMap(): void;
  getLidarsMap(): jspb.Map<string, boolean>;
  clearLidarsMap(): void;
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
    boardsMap: Array<[string, BoardStatus.AsObject]>,
    camerasMap: Array<[string, boolean]>,
    lidarsMap: Array<[string, boolean]>,
    sensorsMap: Array<[string, SensorStatus.AsObject]>,
    functionsMap: Array<[string, boolean]>,
    servosMap: Array<[string, ServoStatus.AsObject]>,
    motorsMap: Array<[string, MotorStatus.AsObject]>,
    servicesMap: Array<[string, boolean]>,
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
  getPose(): ArmPosition | undefined;
  setPose(value?: ArmPosition): void;

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
    pose?: ArmPosition.AsObject,
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

export class ArmStatus extends jspb.Message {
  hasGridPosition(): boolean;
  clearGridPosition(): void;
  getGridPosition(): ArmPosition | undefined;
  setGridPosition(value?: ArmPosition): void;

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
    gridPosition?: ArmPosition.AsObject,
    jointPositions?: JointPositions.AsObject,
  }
}

export class ArmPosition extends jspb.Message {
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
  toObject(includeInstance?: boolean): ArmPosition.AsObject;
  static toObject(includeInstance: boolean, msg: ArmPosition): ArmPosition.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmPosition, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmPosition;
  static deserializeBinaryFromReader(message: ArmPosition, reader: jspb.BinaryReader): ArmPosition;
}

export namespace ArmPosition {
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

export class ArmCurrentPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmCurrentPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmCurrentPositionRequest): ArmCurrentPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmCurrentPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmCurrentPositionRequest;
  static deserializeBinaryFromReader(message: ArmCurrentPositionRequest, reader: jspb.BinaryReader): ArmCurrentPositionRequest;
}

export namespace ArmCurrentPositionRequest {
  export type AsObject = {
    name: string,
  }
}

export class ArmCurrentPositionResponse extends jspb.Message {
  hasPosition(): boolean;
  clearPosition(): void;
  getPosition(): ArmPosition | undefined;
  setPosition(value?: ArmPosition): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmCurrentPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmCurrentPositionResponse): ArmCurrentPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmCurrentPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmCurrentPositionResponse;
  static deserializeBinaryFromReader(message: ArmCurrentPositionResponse, reader: jspb.BinaryReader): ArmCurrentPositionResponse;
}

export namespace ArmCurrentPositionResponse {
  export type AsObject = {
    position?: ArmPosition.AsObject,
  }
}

export class ArmCurrentJointPositionsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmCurrentJointPositionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmCurrentJointPositionsRequest): ArmCurrentJointPositionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmCurrentJointPositionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmCurrentJointPositionsRequest;
  static deserializeBinaryFromReader(message: ArmCurrentJointPositionsRequest, reader: jspb.BinaryReader): ArmCurrentJointPositionsRequest;
}

export namespace ArmCurrentJointPositionsRequest {
  export type AsObject = {
    name: string,
  }
}

export class ArmCurrentJointPositionsResponse extends jspb.Message {
  hasPositions(): boolean;
  clearPositions(): void;
  getPositions(): JointPositions | undefined;
  setPositions(value?: JointPositions): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmCurrentJointPositionsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmCurrentJointPositionsResponse): ArmCurrentJointPositionsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmCurrentJointPositionsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmCurrentJointPositionsResponse;
  static deserializeBinaryFromReader(message: ArmCurrentJointPositionsResponse, reader: jspb.BinaryReader): ArmCurrentJointPositionsResponse;
}

export namespace ArmCurrentJointPositionsResponse {
  export type AsObject = {
    positions?: JointPositions.AsObject,
  }
}

export class ArmMoveToPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasTo(): boolean;
  clearTo(): void;
  getTo(): ArmPosition | undefined;
  setTo(value?: ArmPosition): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmMoveToPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmMoveToPositionRequest): ArmMoveToPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmMoveToPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmMoveToPositionRequest;
  static deserializeBinaryFromReader(message: ArmMoveToPositionRequest, reader: jspb.BinaryReader): ArmMoveToPositionRequest;
}

export namespace ArmMoveToPositionRequest {
  export type AsObject = {
    name: string,
    to?: ArmPosition.AsObject,
  }
}

export class ArmMoveToPositionResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmMoveToPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmMoveToPositionResponse): ArmMoveToPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmMoveToPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmMoveToPositionResponse;
  static deserializeBinaryFromReader(message: ArmMoveToPositionResponse, reader: jspb.BinaryReader): ArmMoveToPositionResponse;
}

export namespace ArmMoveToPositionResponse {
  export type AsObject = {
  }
}

export class ArmMoveToJointPositionsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasTo(): boolean;
  clearTo(): void;
  getTo(): JointPositions | undefined;
  setTo(value?: JointPositions): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmMoveToJointPositionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmMoveToJointPositionsRequest): ArmMoveToJointPositionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmMoveToJointPositionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmMoveToJointPositionsRequest;
  static deserializeBinaryFromReader(message: ArmMoveToJointPositionsRequest, reader: jspb.BinaryReader): ArmMoveToJointPositionsRequest;
}

export namespace ArmMoveToJointPositionsRequest {
  export type AsObject = {
    name: string,
    to?: JointPositions.AsObject,
  }
}

export class ArmMoveToJointPositionsResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmMoveToJointPositionsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmMoveToJointPositionsResponse): ArmMoveToJointPositionsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmMoveToJointPositionsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmMoveToJointPositionsResponse;
  static deserializeBinaryFromReader(message: ArmMoveToJointPositionsResponse, reader: jspb.BinaryReader): ArmMoveToJointPositionsResponse;
}

export namespace ArmMoveToJointPositionsResponse {
  export type AsObject = {
  }
}

export class ArmJointMoveDeltaRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getJoint(): number;
  setJoint(value: number): void;

  getAmountDegs(): number;
  setAmountDegs(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmJointMoveDeltaRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmJointMoveDeltaRequest): ArmJointMoveDeltaRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmJointMoveDeltaRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmJointMoveDeltaRequest;
  static deserializeBinaryFromReader(message: ArmJointMoveDeltaRequest, reader: jspb.BinaryReader): ArmJointMoveDeltaRequest;
}

export namespace ArmJointMoveDeltaRequest {
  export type AsObject = {
    name: string,
    joint: number,
    amountDegs: number,
  }
}

export class ArmJointMoveDeltaResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmJointMoveDeltaResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmJointMoveDeltaResponse): ArmJointMoveDeltaResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmJointMoveDeltaResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmJointMoveDeltaResponse;
  static deserializeBinaryFromReader(message: ArmJointMoveDeltaResponse, reader: jspb.BinaryReader): ArmJointMoveDeltaResponse;
}

export namespace ArmJointMoveDeltaResponse {
  export type AsObject = {
  }
}

export class BaseMoveStraightRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getDistanceMillis(): number;
  setDistanceMillis(value: number): void;

  getMillisPerSec(): number;
  setMillisPerSec(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseMoveStraightRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BaseMoveStraightRequest): BaseMoveStraightRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseMoveStraightRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseMoveStraightRequest;
  static deserializeBinaryFromReader(message: BaseMoveStraightRequest, reader: jspb.BinaryReader): BaseMoveStraightRequest;
}

export namespace BaseMoveStraightRequest {
  export type AsObject = {
    name: string,
    distanceMillis: number,
    millisPerSec: number,
  }
}

export class BaseMoveStraightResponse extends jspb.Message {
  getSuccess(): boolean;
  setSuccess(value: boolean): void;

  getError(): string;
  setError(value: string): void;

  getDistanceMillis(): number;
  setDistanceMillis(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseMoveStraightResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BaseMoveStraightResponse): BaseMoveStraightResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseMoveStraightResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseMoveStraightResponse;
  static deserializeBinaryFromReader(message: BaseMoveStraightResponse, reader: jspb.BinaryReader): BaseMoveStraightResponse;
}

export namespace BaseMoveStraightResponse {
  export type AsObject = {
    success: boolean,
    error: string,
    distanceMillis: number,
  }
}

export class BaseSpinRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getAngleDeg(): number;
  setAngleDeg(value: number): void;

  getDegsPerSec(): number;
  setDegsPerSec(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseSpinRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BaseSpinRequest): BaseSpinRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseSpinRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseSpinRequest;
  static deserializeBinaryFromReader(message: BaseSpinRequest, reader: jspb.BinaryReader): BaseSpinRequest;
}

export namespace BaseSpinRequest {
  export type AsObject = {
    name: string,
    angleDeg: number,
    degsPerSec: number,
  }
}

export class BaseSpinResponse extends jspb.Message {
  getSuccess(): boolean;
  setSuccess(value: boolean): void;

  getError(): string;
  setError(value: string): void;

  getAngleDeg(): number;
  setAngleDeg(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseSpinResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BaseSpinResponse): BaseSpinResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseSpinResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseSpinResponse;
  static deserializeBinaryFromReader(message: BaseSpinResponse, reader: jspb.BinaryReader): BaseSpinResponse;
}

export namespace BaseSpinResponse {
  export type AsObject = {
    success: boolean,
    error: string,
    angleDeg: number,
  }
}

export class BaseStopRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseStopRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BaseStopRequest): BaseStopRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseStopRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseStopRequest;
  static deserializeBinaryFromReader(message: BaseStopRequest, reader: jspb.BinaryReader): BaseStopRequest;
}

export namespace BaseStopRequest {
  export type AsObject = {
    name: string,
  }
}

export class BaseStopResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseStopResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BaseStopResponse): BaseStopResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseStopResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseStopResponse;
  static deserializeBinaryFromReader(message: BaseStopResponse, reader: jspb.BinaryReader): BaseStopResponse;
}

export namespace BaseStopResponse {
  export type AsObject = {
  }
}

export class BaseWidthMillisRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseWidthMillisRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BaseWidthMillisRequest): BaseWidthMillisRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseWidthMillisRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseWidthMillisRequest;
  static deserializeBinaryFromReader(message: BaseWidthMillisRequest, reader: jspb.BinaryReader): BaseWidthMillisRequest;
}

export namespace BaseWidthMillisRequest {
  export type AsObject = {
    name: string,
  }
}

export class BaseWidthMillisResponse extends jspb.Message {
  getWidthMillis(): number;
  setWidthMillis(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseWidthMillisResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BaseWidthMillisResponse): BaseWidthMillisResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseWidthMillisResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseWidthMillisResponse;
  static deserializeBinaryFromReader(message: BaseWidthMillisResponse, reader: jspb.BinaryReader): BaseWidthMillisResponse;
}

export namespace BaseWidthMillisResponse {
  export type AsObject = {
    widthMillis: number,
  }
}

export class GripperOpenRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GripperOpenRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GripperOpenRequest): GripperOpenRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GripperOpenRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GripperOpenRequest;
  static deserializeBinaryFromReader(message: GripperOpenRequest, reader: jspb.BinaryReader): GripperOpenRequest;
}

export namespace GripperOpenRequest {
  export type AsObject = {
    name: string,
  }
}

export class GripperOpenResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GripperOpenResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GripperOpenResponse): GripperOpenResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GripperOpenResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GripperOpenResponse;
  static deserializeBinaryFromReader(message: GripperOpenResponse, reader: jspb.BinaryReader): GripperOpenResponse;
}

export namespace GripperOpenResponse {
  export type AsObject = {
  }
}

export class GripperGrabRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GripperGrabRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GripperGrabRequest): GripperGrabRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GripperGrabRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GripperGrabRequest;
  static deserializeBinaryFromReader(message: GripperGrabRequest, reader: jspb.BinaryReader): GripperGrabRequest;
}

export namespace GripperGrabRequest {
  export type AsObject = {
    name: string,
  }
}

export class GripperGrabResponse extends jspb.Message {
  getGrabbed(): boolean;
  setGrabbed(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GripperGrabResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GripperGrabResponse): GripperGrabResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GripperGrabResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GripperGrabResponse;
  static deserializeBinaryFromReader(message: GripperGrabResponse, reader: jspb.BinaryReader): GripperGrabResponse;
}

export namespace GripperGrabResponse {
  export type AsObject = {
    grabbed: boolean,
  }
}

export class CameraFrameRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMimeType(): string;
  setMimeType(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraFrameRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CameraFrameRequest): CameraFrameRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraFrameRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraFrameRequest;
  static deserializeBinaryFromReader(message: CameraFrameRequest, reader: jspb.BinaryReader): CameraFrameRequest;
}

export namespace CameraFrameRequest {
  export type AsObject = {
    name: string,
    mimeType: string,
  }
}

export class CameraRenderFrameRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMimeType(): string;
  setMimeType(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraRenderFrameRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CameraRenderFrameRequest): CameraRenderFrameRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraRenderFrameRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraRenderFrameRequest;
  static deserializeBinaryFromReader(message: CameraRenderFrameRequest, reader: jspb.BinaryReader): CameraRenderFrameRequest;
}

export namespace CameraRenderFrameRequest {
  export type AsObject = {
    name: string,
    mimeType: string,
  }
}

export class CameraFrameResponse extends jspb.Message {
  getMimeType(): string;
  setMimeType(value: string): void;

  getFrame(): Uint8Array | string;
  getFrame_asU8(): Uint8Array;
  getFrame_asB64(): string;
  setFrame(value: Uint8Array | string): void;

  getDimX(): number;
  setDimX(value: number): void;

  getDimY(): number;
  setDimY(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraFrameResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CameraFrameResponse): CameraFrameResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraFrameResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraFrameResponse;
  static deserializeBinaryFromReader(message: CameraFrameResponse, reader: jspb.BinaryReader): CameraFrameResponse;
}

export namespace CameraFrameResponse {
  export type AsObject = {
    mimeType: string,
    frame: Uint8Array | string,
    dimX: number,
    dimY: number,
  }
}

export class PointCloudRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMimeType(): string;
  setMimeType(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PointCloudRequest.AsObject;
  static toObject(includeInstance: boolean, msg: PointCloudRequest): PointCloudRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PointCloudRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PointCloudRequest;
  static deserializeBinaryFromReader(message: PointCloudRequest, reader: jspb.BinaryReader): PointCloudRequest;
}

export namespace PointCloudRequest {
  export type AsObject = {
    name: string,
    mimeType: string,
  }
}

export class PointCloudResponse extends jspb.Message {
  getMimeType(): string;
  setMimeType(value: string): void;

  getFrame(): Uint8Array | string;
  getFrame_asU8(): Uint8Array;
  getFrame_asB64(): string;
  setFrame(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PointCloudResponse.AsObject;
  static toObject(includeInstance: boolean, msg: PointCloudResponse): PointCloudResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PointCloudResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PointCloudResponse;
  static deserializeBinaryFromReader(message: PointCloudResponse, reader: jspb.BinaryReader): PointCloudResponse;
}

export namespace PointCloudResponse {
  export type AsObject = {
    mimeType: string,
    frame: Uint8Array | string,
  }
}

export class ObjectPointCloudsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMimeType(): string;
  setMimeType(value: string): void;

  getMinPointsInPlane(): number;
  setMinPointsInPlane(value: number): void;

  getMinPointsInSegment(): number;
  setMinPointsInSegment(value: number): void;

  getClusteringRadius(): number;
  setClusteringRadius(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ObjectPointCloudsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ObjectPointCloudsRequest): ObjectPointCloudsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ObjectPointCloudsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ObjectPointCloudsRequest;
  static deserializeBinaryFromReader(message: ObjectPointCloudsRequest, reader: jspb.BinaryReader): ObjectPointCloudsRequest;
}

export namespace ObjectPointCloudsRequest {
  export type AsObject = {
    name: string,
    mimeType: string,
    minPointsInPlane: number,
    minPointsInSegment: number,
    clusteringRadius: number,
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

export class BoxGeometry extends jspb.Message {
  getWidth(): number;
  setWidth(value: number): void;

  getLength(): number;
  setLength(value: number): void;

  getDepth(): number;
  setDepth(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoxGeometry.AsObject;
  static toObject(includeInstance: boolean, msg: BoxGeometry): BoxGeometry.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoxGeometry, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoxGeometry;
  static deserializeBinaryFromReader(message: BoxGeometry, reader: jspb.BinaryReader): BoxGeometry;
}

export namespace BoxGeometry {
  export type AsObject = {
    width: number,
    length: number,
    depth: number,
  }
}

export class ObjectPointCloudsResponse extends jspb.Message {
  getMimeType(): string;
  setMimeType(value: string): void;

  clearFramesList(): void;
  getFramesList(): Array<Uint8Array | string>;
  getFramesList_asU8(): Array<Uint8Array>;
  getFramesList_asB64(): Array<string>;
  setFramesList(value: Array<Uint8Array | string>): void;
  addFrames(value: Uint8Array | string, index?: number): Uint8Array | string;

  clearCentersList(): void;
  getCentersList(): Array<Vector3>;
  setCentersList(value: Array<Vector3>): void;
  addCenters(value?: Vector3, index?: number): Vector3;

  clearBoundingBoxesList(): void;
  getBoundingBoxesList(): Array<BoxGeometry>;
  setBoundingBoxesList(value: Array<BoxGeometry>): void;
  addBoundingBoxes(value?: BoxGeometry, index?: number): BoxGeometry;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ObjectPointCloudsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ObjectPointCloudsResponse): ObjectPointCloudsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ObjectPointCloudsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ObjectPointCloudsResponse;
  static deserializeBinaryFromReader(message: ObjectPointCloudsResponse, reader: jspb.BinaryReader): ObjectPointCloudsResponse;
}

export namespace ObjectPointCloudsResponse {
  export type AsObject = {
    mimeType: string,
    framesList: Array<Uint8Array | string>,
    centersList: Array<Vector3.AsObject>,
    boundingBoxesList: Array<BoxGeometry.AsObject>,
  }
}

export class LidarMeasurement extends jspb.Message {
  getAngle(): number;
  setAngle(value: number): void;

  getAngleDeg(): number;
  setAngleDeg(value: number): void;

  getDistance(): number;
  setDistance(value: number): void;

  getX(): number;
  setX(value: number): void;

  getY(): number;
  setY(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarMeasurement.AsObject;
  static toObject(includeInstance: boolean, msg: LidarMeasurement): LidarMeasurement.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarMeasurement, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarMeasurement;
  static deserializeBinaryFromReader(message: LidarMeasurement, reader: jspb.BinaryReader): LidarMeasurement;
}

export namespace LidarMeasurement {
  export type AsObject = {
    angle: number,
    angleDeg: number,
    distance: number,
    x: number,
    y: number,
  }
}

export class LidarInfoRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarInfoRequest.AsObject;
  static toObject(includeInstance: boolean, msg: LidarInfoRequest): LidarInfoRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarInfoRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarInfoRequest;
  static deserializeBinaryFromReader(message: LidarInfoRequest, reader: jspb.BinaryReader): LidarInfoRequest;
}

export namespace LidarInfoRequest {
  export type AsObject = {
    name: string,
  }
}

export class LidarInfoResponse extends jspb.Message {
  hasInfo(): boolean;
  clearInfo(): void;
  getInfo(): google_protobuf_struct_pb.Struct | undefined;
  setInfo(value?: google_protobuf_struct_pb.Struct): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarInfoResponse.AsObject;
  static toObject(includeInstance: boolean, msg: LidarInfoResponse): LidarInfoResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarInfoResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarInfoResponse;
  static deserializeBinaryFromReader(message: LidarInfoResponse, reader: jspb.BinaryReader): LidarInfoResponse;
}

export namespace LidarInfoResponse {
  export type AsObject = {
    info?: google_protobuf_struct_pb.Struct.AsObject,
  }
}

export class LidarStartRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarStartRequest.AsObject;
  static toObject(includeInstance: boolean, msg: LidarStartRequest): LidarStartRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarStartRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarStartRequest;
  static deserializeBinaryFromReader(message: LidarStartRequest, reader: jspb.BinaryReader): LidarStartRequest;
}

export namespace LidarStartRequest {
  export type AsObject = {
    name: string,
  }
}

export class LidarStartResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarStartResponse.AsObject;
  static toObject(includeInstance: boolean, msg: LidarStartResponse): LidarStartResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarStartResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarStartResponse;
  static deserializeBinaryFromReader(message: LidarStartResponse, reader: jspb.BinaryReader): LidarStartResponse;
}

export namespace LidarStartResponse {
  export type AsObject = {
  }
}

export class LidarStopRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarStopRequest.AsObject;
  static toObject(includeInstance: boolean, msg: LidarStopRequest): LidarStopRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarStopRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarStopRequest;
  static deserializeBinaryFromReader(message: LidarStopRequest, reader: jspb.BinaryReader): LidarStopRequest;
}

export namespace LidarStopRequest {
  export type AsObject = {
    name: string,
  }
}

export class LidarStopResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarStopResponse.AsObject;
  static toObject(includeInstance: boolean, msg: LidarStopResponse): LidarStopResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarStopResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarStopResponse;
  static deserializeBinaryFromReader(message: LidarStopResponse, reader: jspb.BinaryReader): LidarStopResponse;
}

export namespace LidarStopResponse {
  export type AsObject = {
  }
}

export class LidarScanRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getCount(): number;
  setCount(value: number): void;

  getNoFilter(): boolean;
  setNoFilter(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarScanRequest.AsObject;
  static toObject(includeInstance: boolean, msg: LidarScanRequest): LidarScanRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarScanRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarScanRequest;
  static deserializeBinaryFromReader(message: LidarScanRequest, reader: jspb.BinaryReader): LidarScanRequest;
}

export namespace LidarScanRequest {
  export type AsObject = {
    name: string,
    count: number,
    noFilter: boolean,
  }
}

export class LidarScanResponse extends jspb.Message {
  clearMeasurementsList(): void;
  getMeasurementsList(): Array<LidarMeasurement>;
  setMeasurementsList(value: Array<LidarMeasurement>): void;
  addMeasurements(value?: LidarMeasurement, index?: number): LidarMeasurement;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarScanResponse.AsObject;
  static toObject(includeInstance: boolean, msg: LidarScanResponse): LidarScanResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarScanResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarScanResponse;
  static deserializeBinaryFromReader(message: LidarScanResponse, reader: jspb.BinaryReader): LidarScanResponse;
}

export namespace LidarScanResponse {
  export type AsObject = {
    measurementsList: Array<LidarMeasurement.AsObject>,
  }
}

export class LidarRangeRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarRangeRequest.AsObject;
  static toObject(includeInstance: boolean, msg: LidarRangeRequest): LidarRangeRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarRangeRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarRangeRequest;
  static deserializeBinaryFromReader(message: LidarRangeRequest, reader: jspb.BinaryReader): LidarRangeRequest;
}

export namespace LidarRangeRequest {
  export type AsObject = {
    name: string,
  }
}

export class LidarRangeResponse extends jspb.Message {
  getRange(): number;
  setRange(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarRangeResponse.AsObject;
  static toObject(includeInstance: boolean, msg: LidarRangeResponse): LidarRangeResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarRangeResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarRangeResponse;
  static deserializeBinaryFromReader(message: LidarRangeResponse, reader: jspb.BinaryReader): LidarRangeResponse;
}

export namespace LidarRangeResponse {
  export type AsObject = {
    range: number,
  }
}

export class LidarBoundsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarBoundsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: LidarBoundsRequest): LidarBoundsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarBoundsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarBoundsRequest;
  static deserializeBinaryFromReader(message: LidarBoundsRequest, reader: jspb.BinaryReader): LidarBoundsRequest;
}

export namespace LidarBoundsRequest {
  export type AsObject = {
    name: string,
  }
}

export class LidarBoundsResponse extends jspb.Message {
  getX(): number;
  setX(value: number): void;

  getY(): number;
  setY(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarBoundsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: LidarBoundsResponse): LidarBoundsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarBoundsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarBoundsResponse;
  static deserializeBinaryFromReader(message: LidarBoundsResponse, reader: jspb.BinaryReader): LidarBoundsResponse;
}

export namespace LidarBoundsResponse {
  export type AsObject = {
    x: number,
    y: number,
  }
}

export class LidarAngularResolutionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarAngularResolutionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: LidarAngularResolutionRequest): LidarAngularResolutionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarAngularResolutionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarAngularResolutionRequest;
  static deserializeBinaryFromReader(message: LidarAngularResolutionRequest, reader: jspb.BinaryReader): LidarAngularResolutionRequest;
}

export namespace LidarAngularResolutionRequest {
  export type AsObject = {
    name: string,
  }
}

export class LidarAngularResolutionResponse extends jspb.Message {
  getAngularResolution(): number;
  setAngularResolution(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LidarAngularResolutionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: LidarAngularResolutionResponse): LidarAngularResolutionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LidarAngularResolutionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LidarAngularResolutionResponse;
  static deserializeBinaryFromReader(message: LidarAngularResolutionResponse, reader: jspb.BinaryReader): LidarAngularResolutionResponse;
}

export namespace LidarAngularResolutionResponse {
  export type AsObject = {
    angularResolution: number,
  }
}

export class BoardStatus extends jspb.Message {
  getAnalogsMap(): jspb.Map<string, AnalogStatus>;
  clearAnalogsMap(): void;
  getDigitalInterruptsMap(): jspb.Map<string, DigitalInterruptStatus>;
  clearDigitalInterruptsMap(): void;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardStatus.AsObject;
  static toObject(includeInstance: boolean, msg: BoardStatus): BoardStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardStatus;
  static deserializeBinaryFromReader(message: BoardStatus, reader: jspb.BinaryReader): BoardStatus;
}

export namespace BoardStatus {
  export type AsObject = {
    analogsMap: Array<[string, AnalogStatus.AsObject]>,
    digitalInterruptsMap: Array<[string, DigitalInterruptStatus.AsObject]>,
  }
}

export class AnalogStatus extends jspb.Message {
  getValue(): number;
  setValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AnalogStatus.AsObject;
  static toObject(includeInstance: boolean, msg: AnalogStatus): AnalogStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: AnalogStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AnalogStatus;
  static deserializeBinaryFromReader(message: AnalogStatus, reader: jspb.BinaryReader): AnalogStatus;
}

export namespace AnalogStatus {
  export type AsObject = {
    value: number,
  }
}

export class DigitalInterruptStatus extends jspb.Message {
  getValue(): number;
  setValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DigitalInterruptStatus.AsObject;
  static toObject(includeInstance: boolean, msg: DigitalInterruptStatus): DigitalInterruptStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DigitalInterruptStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DigitalInterruptStatus;
  static deserializeBinaryFromReader(message: DigitalInterruptStatus, reader: jspb.BinaryReader): DigitalInterruptStatus;
}

export namespace DigitalInterruptStatus {
  export type AsObject = {
    value: number,
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

export class BoardStatusRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardStatusRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardStatusRequest): BoardStatusRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardStatusRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardStatusRequest;
  static deserializeBinaryFromReader(message: BoardStatusRequest, reader: jspb.BinaryReader): BoardStatusRequest;
}

export namespace BoardStatusRequest {
  export type AsObject = {
    name: string,
  }
}

export class BoardStatusResponse extends jspb.Message {
  hasStatus(): boolean;
  clearStatus(): void;
  getStatus(): BoardStatus | undefined;
  setStatus(value?: BoardStatus): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardStatusResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardStatusResponse): BoardStatusResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardStatusResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardStatusResponse;
  static deserializeBinaryFromReader(message: BoardStatusResponse, reader: jspb.BinaryReader): BoardStatusResponse;
}

export namespace BoardStatusResponse {
  export type AsObject = {
    status?: BoardStatus.AsObject,
  }
}

export class BoardGPIOSetRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  getHigh(): boolean;
  setHigh(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardGPIOSetRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardGPIOSetRequest): BoardGPIOSetRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardGPIOSetRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardGPIOSetRequest;
  static deserializeBinaryFromReader(message: BoardGPIOSetRequest, reader: jspb.BinaryReader): BoardGPIOSetRequest;
}

export namespace BoardGPIOSetRequest {
  export type AsObject = {
    name: string,
    pin: string,
    high: boolean,
  }
}

export class BoardGPIOSetResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardGPIOSetResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardGPIOSetResponse): BoardGPIOSetResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardGPIOSetResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardGPIOSetResponse;
  static deserializeBinaryFromReader(message: BoardGPIOSetResponse, reader: jspb.BinaryReader): BoardGPIOSetResponse;
}

export namespace BoardGPIOSetResponse {
  export type AsObject = {
  }
}

export class BoardGPIOGetRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardGPIOGetRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardGPIOGetRequest): BoardGPIOGetRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardGPIOGetRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardGPIOGetRequest;
  static deserializeBinaryFromReader(message: BoardGPIOGetRequest, reader: jspb.BinaryReader): BoardGPIOGetRequest;
}

export namespace BoardGPIOGetRequest {
  export type AsObject = {
    name: string,
    pin: string,
  }
}

export class BoardGPIOGetResponse extends jspb.Message {
  getHigh(): boolean;
  setHigh(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardGPIOGetResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardGPIOGetResponse): BoardGPIOGetResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardGPIOGetResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardGPIOGetResponse;
  static deserializeBinaryFromReader(message: BoardGPIOGetResponse, reader: jspb.BinaryReader): BoardGPIOGetResponse;
}

export namespace BoardGPIOGetResponse {
  export type AsObject = {
    high: boolean,
  }
}

export class BoardPWMSetRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  getDutyCycle(): number;
  setDutyCycle(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardPWMSetRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardPWMSetRequest): BoardPWMSetRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardPWMSetRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardPWMSetRequest;
  static deserializeBinaryFromReader(message: BoardPWMSetRequest, reader: jspb.BinaryReader): BoardPWMSetRequest;
}

export namespace BoardPWMSetRequest {
  export type AsObject = {
    name: string,
    pin: string,
    dutyCycle: number,
  }
}

export class BoardPWMSetResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardPWMSetResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardPWMSetResponse): BoardPWMSetResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardPWMSetResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardPWMSetResponse;
  static deserializeBinaryFromReader(message: BoardPWMSetResponse, reader: jspb.BinaryReader): BoardPWMSetResponse;
}

export namespace BoardPWMSetResponse {
  export type AsObject = {
  }
}

export class BoardPWMSetFrequencyResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardPWMSetFrequencyResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardPWMSetFrequencyResponse): BoardPWMSetFrequencyResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardPWMSetFrequencyResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardPWMSetFrequencyResponse;
  static deserializeBinaryFromReader(message: BoardPWMSetFrequencyResponse, reader: jspb.BinaryReader): BoardPWMSetFrequencyResponse;
}

export namespace BoardPWMSetFrequencyResponse {
  export type AsObject = {
  }
}

export class BoardPWMSetFrequencyRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  getFrequency(): number;
  setFrequency(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardPWMSetFrequencyRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardPWMSetFrequencyRequest): BoardPWMSetFrequencyRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardPWMSetFrequencyRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardPWMSetFrequencyRequest;
  static deserializeBinaryFromReader(message: BoardPWMSetFrequencyRequest, reader: jspb.BinaryReader): BoardPWMSetFrequencyRequest;
}

export namespace BoardPWMSetFrequencyRequest {
  export type AsObject = {
    name: string,
    pin: string,
    frequency: number,
  }
}

export class BoardAnalogReaderReadRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getAnalogReaderName(): string;
  setAnalogReaderName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardAnalogReaderReadRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardAnalogReaderReadRequest): BoardAnalogReaderReadRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardAnalogReaderReadRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardAnalogReaderReadRequest;
  static deserializeBinaryFromReader(message: BoardAnalogReaderReadRequest, reader: jspb.BinaryReader): BoardAnalogReaderReadRequest;
}

export namespace BoardAnalogReaderReadRequest {
  export type AsObject = {
    boardName: string,
    analogReaderName: string,
  }
}

export class BoardAnalogReaderReadResponse extends jspb.Message {
  getValue(): number;
  setValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardAnalogReaderReadResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardAnalogReaderReadResponse): BoardAnalogReaderReadResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardAnalogReaderReadResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardAnalogReaderReadResponse;
  static deserializeBinaryFromReader(message: BoardAnalogReaderReadResponse, reader: jspb.BinaryReader): BoardAnalogReaderReadResponse;
}

export namespace BoardAnalogReaderReadResponse {
  export type AsObject = {
    value: number,
  }
}

export class DigitalInterruptConfig extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  getType(): string;
  setType(value: string): void;

  getFormula(): string;
  setFormula(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DigitalInterruptConfig.AsObject;
  static toObject(includeInstance: boolean, msg: DigitalInterruptConfig): DigitalInterruptConfig.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DigitalInterruptConfig, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DigitalInterruptConfig;
  static deserializeBinaryFromReader(message: DigitalInterruptConfig, reader: jspb.BinaryReader): DigitalInterruptConfig;
}

export namespace DigitalInterruptConfig {
  export type AsObject = {
    name: string,
    pin: string,
    type: string,
    formula: string,
  }
}

export class BoardDigitalInterruptConfigRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getDigitalInterruptName(): string;
  setDigitalInterruptName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardDigitalInterruptConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardDigitalInterruptConfigRequest): BoardDigitalInterruptConfigRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardDigitalInterruptConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardDigitalInterruptConfigRequest;
  static deserializeBinaryFromReader(message: BoardDigitalInterruptConfigRequest, reader: jspb.BinaryReader): BoardDigitalInterruptConfigRequest;
}

export namespace BoardDigitalInterruptConfigRequest {
  export type AsObject = {
    boardName: string,
    digitalInterruptName: string,
  }
}

export class BoardDigitalInterruptConfigResponse extends jspb.Message {
  hasConfig(): boolean;
  clearConfig(): void;
  getConfig(): DigitalInterruptConfig | undefined;
  setConfig(value?: DigitalInterruptConfig): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardDigitalInterruptConfigResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardDigitalInterruptConfigResponse): BoardDigitalInterruptConfigResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardDigitalInterruptConfigResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardDigitalInterruptConfigResponse;
  static deserializeBinaryFromReader(message: BoardDigitalInterruptConfigResponse, reader: jspb.BinaryReader): BoardDigitalInterruptConfigResponse;
}

export namespace BoardDigitalInterruptConfigResponse {
  export type AsObject = {
    config?: DigitalInterruptConfig.AsObject,
  }
}

export class BoardDigitalInterruptValueRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getDigitalInterruptName(): string;
  setDigitalInterruptName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardDigitalInterruptValueRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardDigitalInterruptValueRequest): BoardDigitalInterruptValueRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardDigitalInterruptValueRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardDigitalInterruptValueRequest;
  static deserializeBinaryFromReader(message: BoardDigitalInterruptValueRequest, reader: jspb.BinaryReader): BoardDigitalInterruptValueRequest;
}

export namespace BoardDigitalInterruptValueRequest {
  export type AsObject = {
    boardName: string,
    digitalInterruptName: string,
  }
}

export class BoardDigitalInterruptValueResponse extends jspb.Message {
  getValue(): number;
  setValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardDigitalInterruptValueResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardDigitalInterruptValueResponse): BoardDigitalInterruptValueResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardDigitalInterruptValueResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardDigitalInterruptValueResponse;
  static deserializeBinaryFromReader(message: BoardDigitalInterruptValueResponse, reader: jspb.BinaryReader): BoardDigitalInterruptValueResponse;
}

export namespace BoardDigitalInterruptValueResponse {
  export type AsObject = {
    value: number,
  }
}

export class BoardDigitalInterruptTickRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getDigitalInterruptName(): string;
  setDigitalInterruptName(value: string): void;

  getHigh(): boolean;
  setHigh(value: boolean): void;

  getNanos(): number;
  setNanos(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardDigitalInterruptTickRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardDigitalInterruptTickRequest): BoardDigitalInterruptTickRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardDigitalInterruptTickRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardDigitalInterruptTickRequest;
  static deserializeBinaryFromReader(message: BoardDigitalInterruptTickRequest, reader: jspb.BinaryReader): BoardDigitalInterruptTickRequest;
}

export namespace BoardDigitalInterruptTickRequest {
  export type AsObject = {
    boardName: string,
    digitalInterruptName: string,
    high: boolean,
    nanos: number,
  }
}

export class BoardDigitalInterruptTickResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardDigitalInterruptTickResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardDigitalInterruptTickResponse): BoardDigitalInterruptTickResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardDigitalInterruptTickResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardDigitalInterruptTickResponse;
  static deserializeBinaryFromReader(message: BoardDigitalInterruptTickResponse, reader: jspb.BinaryReader): BoardDigitalInterruptTickResponse;
}

export namespace BoardDigitalInterruptTickResponse {
  export type AsObject = {
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

export class CompassHeadingRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CompassHeadingRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CompassHeadingRequest): CompassHeadingRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CompassHeadingRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CompassHeadingRequest;
  static deserializeBinaryFromReader(message: CompassHeadingRequest, reader: jspb.BinaryReader): CompassHeadingRequest;
}

export namespace CompassHeadingRequest {
  export type AsObject = {
    name: string,
  }
}

export class CompassHeadingResponse extends jspb.Message {
  getHeading(): number;
  setHeading(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CompassHeadingResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CompassHeadingResponse): CompassHeadingResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CompassHeadingResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CompassHeadingResponse;
  static deserializeBinaryFromReader(message: CompassHeadingResponse, reader: jspb.BinaryReader): CompassHeadingResponse;
}

export namespace CompassHeadingResponse {
  export type AsObject = {
    heading: number,
  }
}

export class CompassStartCalibrationRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CompassStartCalibrationRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CompassStartCalibrationRequest): CompassStartCalibrationRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CompassStartCalibrationRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CompassStartCalibrationRequest;
  static deserializeBinaryFromReader(message: CompassStartCalibrationRequest, reader: jspb.BinaryReader): CompassStartCalibrationRequest;
}

export namespace CompassStartCalibrationRequest {
  export type AsObject = {
    name: string,
  }
}

export class CompassStartCalibrationResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CompassStartCalibrationResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CompassStartCalibrationResponse): CompassStartCalibrationResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CompassStartCalibrationResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CompassStartCalibrationResponse;
  static deserializeBinaryFromReader(message: CompassStartCalibrationResponse, reader: jspb.BinaryReader): CompassStartCalibrationResponse;
}

export namespace CompassStartCalibrationResponse {
  export type AsObject = {
  }
}

export class CompassStopCalibrationRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CompassStopCalibrationRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CompassStopCalibrationRequest): CompassStopCalibrationRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CompassStopCalibrationRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CompassStopCalibrationRequest;
  static deserializeBinaryFromReader(message: CompassStopCalibrationRequest, reader: jspb.BinaryReader): CompassStopCalibrationRequest;
}

export namespace CompassStopCalibrationRequest {
  export type AsObject = {
    name: string,
  }
}

export class CompassStopCalibrationResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CompassStopCalibrationResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CompassStopCalibrationResponse): CompassStopCalibrationResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CompassStopCalibrationResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CompassStopCalibrationResponse;
  static deserializeBinaryFromReader(message: CompassStopCalibrationResponse, reader: jspb.BinaryReader): CompassStopCalibrationResponse;
}

export namespace CompassStopCalibrationResponse {
  export type AsObject = {
  }
}

export class CompassMarkRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CompassMarkRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CompassMarkRequest): CompassMarkRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CompassMarkRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CompassMarkRequest;
  static deserializeBinaryFromReader(message: CompassMarkRequest, reader: jspb.BinaryReader): CompassMarkRequest;
}

export namespace CompassMarkRequest {
  export type AsObject = {
    name: string,
  }
}

export class CompassMarkResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CompassMarkResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CompassMarkResponse): CompassMarkResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CompassMarkResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CompassMarkResponse;
  static deserializeBinaryFromReader(message: CompassMarkResponse, reader: jspb.BinaryReader): CompassMarkResponse;
}

export namespace CompassMarkResponse {
  export type AsObject = {
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

export class ServoMoveRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getAngleDeg(): number;
  setAngleDeg(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServoMoveRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ServoMoveRequest): ServoMoveRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServoMoveRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServoMoveRequest;
  static deserializeBinaryFromReader(message: ServoMoveRequest, reader: jspb.BinaryReader): ServoMoveRequest;
}

export namespace ServoMoveRequest {
  export type AsObject = {
    name: string,
    angleDeg: number,
  }
}

export class ServoMoveResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServoMoveResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ServoMoveResponse): ServoMoveResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServoMoveResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServoMoveResponse;
  static deserializeBinaryFromReader(message: ServoMoveResponse, reader: jspb.BinaryReader): ServoMoveResponse;
}

export namespace ServoMoveResponse {
  export type AsObject = {
  }
}

export class ServoCurrentRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServoCurrentRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ServoCurrentRequest): ServoCurrentRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServoCurrentRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServoCurrentRequest;
  static deserializeBinaryFromReader(message: ServoCurrentRequest, reader: jspb.BinaryReader): ServoCurrentRequest;
}

export namespace ServoCurrentRequest {
  export type AsObject = {
    name: string,
  }
}

export class ServoCurrentResponse extends jspb.Message {
  getAngleDeg(): number;
  setAngleDeg(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServoCurrentResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ServoCurrentResponse): ServoCurrentResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServoCurrentResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServoCurrentResponse;
  static deserializeBinaryFromReader(message: ServoCurrentResponse, reader: jspb.BinaryReader): ServoCurrentResponse;
}

export namespace ServoCurrentResponse {
  export type AsObject = {
    angleDeg: number,
  }
}

export class MotorPowerRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPowerPct(): number;
  setPowerPct(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorPowerRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorPowerRequest): MotorPowerRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorPowerRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorPowerRequest;
  static deserializeBinaryFromReader(message: MotorPowerRequest, reader: jspb.BinaryReader): MotorPowerRequest;
}

export namespace MotorPowerRequest {
  export type AsObject = {
    name: string,
    powerPct: number,
  }
}

export class MotorPowerResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorPowerResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorPowerResponse): MotorPowerResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorPowerResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorPowerResponse;
  static deserializeBinaryFromReader(message: MotorPowerResponse, reader: jspb.BinaryReader): MotorPowerResponse;
}

export namespace MotorPowerResponse {
  export type AsObject = {
  }
}

export class MotorGoRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getDirection(): DirectionRelativeMap[keyof DirectionRelativeMap];
  setDirection(value: DirectionRelativeMap[keyof DirectionRelativeMap]): void;

  getPowerPct(): number;
  setPowerPct(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorGoRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorGoRequest): MotorGoRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorGoRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorGoRequest;
  static deserializeBinaryFromReader(message: MotorGoRequest, reader: jspb.BinaryReader): MotorGoRequest;
}

export namespace MotorGoRequest {
  export type AsObject = {
    name: string,
    direction: DirectionRelativeMap[keyof DirectionRelativeMap],
    powerPct: number,
  }
}

export class MotorGoResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorGoResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorGoResponse): MotorGoResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorGoResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorGoResponse;
  static deserializeBinaryFromReader(message: MotorGoResponse, reader: jspb.BinaryReader): MotorGoResponse;
}

export namespace MotorGoResponse {
  export type AsObject = {
  }
}

export class MotorGoForRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getDirection(): DirectionRelativeMap[keyof DirectionRelativeMap];
  setDirection(value: DirectionRelativeMap[keyof DirectionRelativeMap]): void;

  getRpm(): number;
  setRpm(value: number): void;

  getRevolutions(): number;
  setRevolutions(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorGoForRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorGoForRequest): MotorGoForRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorGoForRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorGoForRequest;
  static deserializeBinaryFromReader(message: MotorGoForRequest, reader: jspb.BinaryReader): MotorGoForRequest;
}

export namespace MotorGoForRequest {
  export type AsObject = {
    name: string,
    direction: DirectionRelativeMap[keyof DirectionRelativeMap],
    rpm: number,
    revolutions: number,
  }
}

export class MotorGoForResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorGoForResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorGoForResponse): MotorGoForResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorGoForResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorGoForResponse;
  static deserializeBinaryFromReader(message: MotorGoForResponse, reader: jspb.BinaryReader): MotorGoForResponse;
}

export namespace MotorGoForResponse {
  export type AsObject = {
  }
}

export class MotorGoToRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getRpm(): number;
  setRpm(value: number): void;

  getPosition(): number;
  setPosition(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorGoToRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorGoToRequest): MotorGoToRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorGoToRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorGoToRequest;
  static deserializeBinaryFromReader(message: MotorGoToRequest, reader: jspb.BinaryReader): MotorGoToRequest;
}

export namespace MotorGoToRequest {
  export type AsObject = {
    name: string,
    rpm: number,
    position: number,
  }
}

export class MotorGoToResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorGoToResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorGoToResponse): MotorGoToResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorGoToResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorGoToResponse;
  static deserializeBinaryFromReader(message: MotorGoToResponse, reader: jspb.BinaryReader): MotorGoToResponse;
}

export namespace MotorGoToResponse {
  export type AsObject = {
  }
}

export class MotorGoTillStopRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getDirection(): DirectionRelativeMap[keyof DirectionRelativeMap];
  setDirection(value: DirectionRelativeMap[keyof DirectionRelativeMap]): void;

  getRpm(): number;
  setRpm(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorGoTillStopRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorGoTillStopRequest): MotorGoTillStopRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorGoTillStopRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorGoTillStopRequest;
  static deserializeBinaryFromReader(message: MotorGoTillStopRequest, reader: jspb.BinaryReader): MotorGoTillStopRequest;
}

export namespace MotorGoTillStopRequest {
  export type AsObject = {
    name: string,
    direction: DirectionRelativeMap[keyof DirectionRelativeMap],
    rpm: number,
  }
}

export class MotorGoTillStopResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorGoTillStopResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorGoTillStopResponse): MotorGoTillStopResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorGoTillStopResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorGoTillStopResponse;
  static deserializeBinaryFromReader(message: MotorGoTillStopResponse, reader: jspb.BinaryReader): MotorGoTillStopResponse;
}

export namespace MotorGoTillStopResponse {
  export type AsObject = {
  }
}

export class MotorZeroRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getOffset(): number;
  setOffset(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorZeroRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorZeroRequest): MotorZeroRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorZeroRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorZeroRequest;
  static deserializeBinaryFromReader(message: MotorZeroRequest, reader: jspb.BinaryReader): MotorZeroRequest;
}

export namespace MotorZeroRequest {
  export type AsObject = {
    name: string,
    offset: number,
  }
}

export class MotorZeroResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorZeroResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorZeroResponse): MotorZeroResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorZeroResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorZeroResponse;
  static deserializeBinaryFromReader(message: MotorZeroResponse, reader: jspb.BinaryReader): MotorZeroResponse;
}

export namespace MotorZeroResponse {
  export type AsObject = {
  }
}

export class MotorPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorPositionRequest): MotorPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorPositionRequest;
  static deserializeBinaryFromReader(message: MotorPositionRequest, reader: jspb.BinaryReader): MotorPositionRequest;
}

export namespace MotorPositionRequest {
  export type AsObject = {
    name: string,
  }
}

export class MotorPositionResponse extends jspb.Message {
  getPosition(): number;
  setPosition(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorPositionResponse): MotorPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorPositionResponse;
  static deserializeBinaryFromReader(message: MotorPositionResponse, reader: jspb.BinaryReader): MotorPositionResponse;
}

export namespace MotorPositionResponse {
  export type AsObject = {
    position: number,
  }
}

export class MotorPositionSupportedRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorPositionSupportedRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorPositionSupportedRequest): MotorPositionSupportedRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorPositionSupportedRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorPositionSupportedRequest;
  static deserializeBinaryFromReader(message: MotorPositionSupportedRequest, reader: jspb.BinaryReader): MotorPositionSupportedRequest;
}

export namespace MotorPositionSupportedRequest {
  export type AsObject = {
    name: string,
  }
}

export class MotorPositionSupportedResponse extends jspb.Message {
  getSupported(): boolean;
  setSupported(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorPositionSupportedResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorPositionSupportedResponse): MotorPositionSupportedResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorPositionSupportedResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorPositionSupportedResponse;
  static deserializeBinaryFromReader(message: MotorPositionSupportedResponse, reader: jspb.BinaryReader): MotorPositionSupportedResponse;
}

export namespace MotorPositionSupportedResponse {
  export type AsObject = {
    supported: boolean,
  }
}

export class MotorOffRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorOffRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorOffRequest): MotorOffRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorOffRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorOffRequest;
  static deserializeBinaryFromReader(message: MotorOffRequest, reader: jspb.BinaryReader): MotorOffRequest;
}

export namespace MotorOffRequest {
  export type AsObject = {
    name: string,
  }
}

export class MotorOffResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorOffResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorOffResponse): MotorOffResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorOffResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorOffResponse;
  static deserializeBinaryFromReader(message: MotorOffResponse, reader: jspb.BinaryReader): MotorOffResponse;
}

export namespace MotorOffResponse {
  export type AsObject = {
  }
}

export class MotorIsOnRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorIsOnRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorIsOnRequest): MotorIsOnRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorIsOnRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorIsOnRequest;
  static deserializeBinaryFromReader(message: MotorIsOnRequest, reader: jspb.BinaryReader): MotorIsOnRequest;
}

export namespace MotorIsOnRequest {
  export type AsObject = {
    name: string,
  }
}

export class MotorIsOnResponse extends jspb.Message {
  getIsOn(): boolean;
  setIsOn(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorIsOnResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorIsOnResponse): MotorIsOnResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorIsOnResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorIsOnResponse;
  static deserializeBinaryFromReader(message: MotorIsOnResponse, reader: jspb.BinaryReader): MotorIsOnResponse;
}

export namespace MotorIsOnResponse {
  export type AsObject = {
    isOn: boolean,
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
  getLocation(): GeoPoint | undefined;
  setLocation(value?: GeoPoint): void;

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
    location?: GeoPoint.AsObject,
  }
}

export class GeoPoint extends jspb.Message {
  getLatitude(): number;
  setLatitude(value: number): void;

  getLongitude(): number;
  setLongitude(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GeoPoint.AsObject;
  static toObject(includeInstance: boolean, msg: GeoPoint): GeoPoint.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GeoPoint, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GeoPoint;
  static deserializeBinaryFromReader(message: GeoPoint, reader: jspb.BinaryReader): GeoPoint;
}

export namespace GeoPoint {
  export type AsObject = {
    latitude: number,
    longitude: number,
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
  getLocation(): GeoPoint | undefined;
  setLocation(value?: GeoPoint): void;

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
    location?: GeoPoint.AsObject,
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
  getLocation(): GeoPoint | undefined;
  setLocation(value?: GeoPoint): void;

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
    location?: GeoPoint.AsObject,
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

export interface DirectionRelativeMap {
  DIRECTION_RELATIVE_UNSPECIFIED: 0;
  DIRECTION_RELATIVE_FORWARD: 1;
  DIRECTION_RELATIVE_BACKWARD: 2;
}

export const DirectionRelative: DirectionRelativeMap;

export interface NavigationServiceModeMap {
  NAVIGATION_SERVICE_MODE_UNSPECIFIED: 0;
  NAVIGATION_SERVICE_MODE_MANUAL: 1;
  NAVIGATION_SERVICE_MODE_WAYPOINT: 2;
}

export const NavigationServiceMode: NavigationServiceModeMap;

