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
  }
}

export class ComponentConfig extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getType(): string;
  setType(value: string): void;

  getParent(): string;
  setParent(value: string): void;

  hasTranslation(): boolean;
  clearTranslation(): void;
  getTranslation(): ArmPosition | undefined;
  setTranslation(value?: ArmPosition): void;

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
    translation?: ArmPosition.AsObject,
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
  getMotorsMap(): jspb.Map<string, MotorStatus>;
  clearMotorsMap(): void;
  getServosMap(): jspb.Map<string, ServoStatus>;
  clearServosMap(): void;
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
    motorsMap: Array<[string, MotorStatus.AsObject]>,
    servosMap: Array<[string, ServoStatus.AsObject]>,
    analogsMap: Array<[string, AnalogStatus.AsObject]>,
    digitalInterruptsMap: Array<[string, DigitalInterruptStatus.AsObject]>,
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

export class BoardMotorStatusRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getMotorName(): string;
  setMotorName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorStatusRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorStatusRequest): BoardMotorStatusRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorStatusRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorStatusRequest;
  static deserializeBinaryFromReader(message: BoardMotorStatusRequest, reader: jspb.BinaryReader): BoardMotorStatusRequest;
}

export namespace BoardMotorStatusRequest {
  export type AsObject = {
    boardName: string,
    motorName: string,
  }
}

export class BoardMotorStatusResponse extends jspb.Message {
  hasStatus(): boolean;
  clearStatus(): void;
  getStatus(): MotorStatus | undefined;
  setStatus(value?: MotorStatus): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorStatusResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorStatusResponse): BoardMotorStatusResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorStatusResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorStatusResponse;
  static deserializeBinaryFromReader(message: BoardMotorStatusResponse, reader: jspb.BinaryReader): BoardMotorStatusResponse;
}

export namespace BoardMotorStatusResponse {
  export type AsObject = {
    status?: MotorStatus.AsObject,
  }
}

export class BoardMotorPowerRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getMotorName(): string;
  setMotorName(value: string): void;

  getPowerPct(): number;
  setPowerPct(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorPowerRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorPowerRequest): BoardMotorPowerRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorPowerRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorPowerRequest;
  static deserializeBinaryFromReader(message: BoardMotorPowerRequest, reader: jspb.BinaryReader): BoardMotorPowerRequest;
}

export namespace BoardMotorPowerRequest {
  export type AsObject = {
    boardName: string,
    motorName: string,
    powerPct: number,
  }
}

export class BoardMotorPowerResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorPowerResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorPowerResponse): BoardMotorPowerResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorPowerResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorPowerResponse;
  static deserializeBinaryFromReader(message: BoardMotorPowerResponse, reader: jspb.BinaryReader): BoardMotorPowerResponse;
}

export namespace BoardMotorPowerResponse {
  export type AsObject = {
  }
}

export class BoardMotorGoRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getMotorName(): string;
  setMotorName(value: string): void;

  getDirection(): DirectionRelativeMap[keyof DirectionRelativeMap];
  setDirection(value: DirectionRelativeMap[keyof DirectionRelativeMap]): void;

  getPowerPct(): number;
  setPowerPct(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorGoRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorGoRequest): BoardMotorGoRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorGoRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorGoRequest;
  static deserializeBinaryFromReader(message: BoardMotorGoRequest, reader: jspb.BinaryReader): BoardMotorGoRequest;
}

export namespace BoardMotorGoRequest {
  export type AsObject = {
    boardName: string,
    motorName: string,
    direction: DirectionRelativeMap[keyof DirectionRelativeMap],
    powerPct: number,
  }
}

export class BoardMotorGoResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorGoResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorGoResponse): BoardMotorGoResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorGoResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorGoResponse;
  static deserializeBinaryFromReader(message: BoardMotorGoResponse, reader: jspb.BinaryReader): BoardMotorGoResponse;
}

export namespace BoardMotorGoResponse {
  export type AsObject = {
  }
}

export class BoardMotorGoForRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getMotorName(): string;
  setMotorName(value: string): void;

  getDirection(): DirectionRelativeMap[keyof DirectionRelativeMap];
  setDirection(value: DirectionRelativeMap[keyof DirectionRelativeMap]): void;

  getRpm(): number;
  setRpm(value: number): void;

  getRevolutions(): number;
  setRevolutions(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorGoForRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorGoForRequest): BoardMotorGoForRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorGoForRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorGoForRequest;
  static deserializeBinaryFromReader(message: BoardMotorGoForRequest, reader: jspb.BinaryReader): BoardMotorGoForRequest;
}

export namespace BoardMotorGoForRequest {
  export type AsObject = {
    boardName: string,
    motorName: string,
    direction: DirectionRelativeMap[keyof DirectionRelativeMap],
    rpm: number,
    revolutions: number,
  }
}

export class BoardMotorGoForResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorGoForResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorGoForResponse): BoardMotorGoForResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorGoForResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorGoForResponse;
  static deserializeBinaryFromReader(message: BoardMotorGoForResponse, reader: jspb.BinaryReader): BoardMotorGoForResponse;
}

export namespace BoardMotorGoForResponse {
  export type AsObject = {
  }
}

export class BoardMotorGoToRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getMotorName(): string;
  setMotorName(value: string): void;

  getRpm(): number;
  setRpm(value: number): void;

  getPosition(): number;
  setPosition(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorGoToRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorGoToRequest): BoardMotorGoToRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorGoToRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorGoToRequest;
  static deserializeBinaryFromReader(message: BoardMotorGoToRequest, reader: jspb.BinaryReader): BoardMotorGoToRequest;
}

export namespace BoardMotorGoToRequest {
  export type AsObject = {
    boardName: string,
    motorName: string,
    rpm: number,
    position: number,
  }
}

export class BoardMotorGoToResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorGoToResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorGoToResponse): BoardMotorGoToResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorGoToResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorGoToResponse;
  static deserializeBinaryFromReader(message: BoardMotorGoToResponse, reader: jspb.BinaryReader): BoardMotorGoToResponse;
}

export namespace BoardMotorGoToResponse {
  export type AsObject = {
  }
}

export class BoardMotorGoTillStopRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getMotorName(): string;
  setMotorName(value: string): void;

  getDirection(): DirectionRelativeMap[keyof DirectionRelativeMap];
  setDirection(value: DirectionRelativeMap[keyof DirectionRelativeMap]): void;

  getRpm(): number;
  setRpm(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorGoTillStopRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorGoTillStopRequest): BoardMotorGoTillStopRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorGoTillStopRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorGoTillStopRequest;
  static deserializeBinaryFromReader(message: BoardMotorGoTillStopRequest, reader: jspb.BinaryReader): BoardMotorGoTillStopRequest;
}

export namespace BoardMotorGoTillStopRequest {
  export type AsObject = {
    boardName: string,
    motorName: string,
    direction: DirectionRelativeMap[keyof DirectionRelativeMap],
    rpm: number,
  }
}

export class BoardMotorGoTillStopResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorGoTillStopResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorGoTillStopResponse): BoardMotorGoTillStopResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorGoTillStopResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorGoTillStopResponse;
  static deserializeBinaryFromReader(message: BoardMotorGoTillStopResponse, reader: jspb.BinaryReader): BoardMotorGoTillStopResponse;
}

export namespace BoardMotorGoTillStopResponse {
  export type AsObject = {
  }
}

export class BoardMotorZeroRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getMotorName(): string;
  setMotorName(value: string): void;

  getOffset(): number;
  setOffset(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorZeroRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorZeroRequest): BoardMotorZeroRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorZeroRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorZeroRequest;
  static deserializeBinaryFromReader(message: BoardMotorZeroRequest, reader: jspb.BinaryReader): BoardMotorZeroRequest;
}

export namespace BoardMotorZeroRequest {
  export type AsObject = {
    boardName: string,
    motorName: string,
    offset: number,
  }
}

export class BoardMotorZeroResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorZeroResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorZeroResponse): BoardMotorZeroResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorZeroResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorZeroResponse;
  static deserializeBinaryFromReader(message: BoardMotorZeroResponse, reader: jspb.BinaryReader): BoardMotorZeroResponse;
}

export namespace BoardMotorZeroResponse {
  export type AsObject = {
  }
}

export class BoardMotorOffRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getMotorName(): string;
  setMotorName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorOffRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorOffRequest): BoardMotorOffRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorOffRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorOffRequest;
  static deserializeBinaryFromReader(message: BoardMotorOffRequest, reader: jspb.BinaryReader): BoardMotorOffRequest;
}

export namespace BoardMotorOffRequest {
  export type AsObject = {
    boardName: string,
    motorName: string,
  }
}

export class BoardMotorOffResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardMotorOffResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardMotorOffResponse): BoardMotorOffResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardMotorOffResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardMotorOffResponse;
  static deserializeBinaryFromReader(message: BoardMotorOffResponse, reader: jspb.BinaryReader): BoardMotorOffResponse;
}

export namespace BoardMotorOffResponse {
  export type AsObject = {
  }
}

export class BoardServoMoveRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getServoName(): string;
  setServoName(value: string): void;

  getAngleDeg(): number;
  setAngleDeg(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServoMoveRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServoMoveRequest): BoardServoMoveRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServoMoveRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServoMoveRequest;
  static deserializeBinaryFromReader(message: BoardServoMoveRequest, reader: jspb.BinaryReader): BoardServoMoveRequest;
}

export namespace BoardServoMoveRequest {
  export type AsObject = {
    boardName: string,
    servoName: string,
    angleDeg: number,
  }
}

export class BoardServoMoveResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServoMoveResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServoMoveResponse): BoardServoMoveResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServoMoveResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServoMoveResponse;
  static deserializeBinaryFromReader(message: BoardServoMoveResponse, reader: jspb.BinaryReader): BoardServoMoveResponse;
}

export namespace BoardServoMoveResponse {
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

export interface DirectionRelativeMap {
  DIRECTION_RELATIVE_UNSPECIFIED: 0;
  DIRECTION_RELATIVE_FORWARD: 1;
  DIRECTION_RELATIVE_BACKWARD: 2;
}

export const DirectionRelative: DirectionRelativeMap;

