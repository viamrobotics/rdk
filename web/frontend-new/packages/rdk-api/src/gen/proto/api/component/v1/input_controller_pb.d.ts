// package: proto.api.component.v1
// file: proto/api/component/v1/input_controller.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class InputControllerServiceGetControlsRequest extends jspb.Message {
  getController(): string;
  setController(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceGetControlsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceGetControlsRequest): InputControllerServiceGetControlsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceGetControlsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceGetControlsRequest;
  static deserializeBinaryFromReader(message: InputControllerServiceGetControlsRequest, reader: jspb.BinaryReader): InputControllerServiceGetControlsRequest;
}

export namespace InputControllerServiceGetControlsRequest {
  export type AsObject = {
    controller: string,
  }
}

export class InputControllerServiceGetControlsResponse extends jspb.Message {
  clearControlsList(): void;
  getControlsList(): Array<string>;
  setControlsList(value: Array<string>): void;
  addControls(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceGetControlsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceGetControlsResponse): InputControllerServiceGetControlsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceGetControlsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceGetControlsResponse;
  static deserializeBinaryFromReader(message: InputControllerServiceGetControlsResponse, reader: jspb.BinaryReader): InputControllerServiceGetControlsResponse;
}

export namespace InputControllerServiceGetControlsResponse {
  export type AsObject = {
    controlsList: Array<string>,
  }
}

export class InputControllerServiceGetEventsRequest extends jspb.Message {
  getController(): string;
  setController(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceGetEventsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceGetEventsRequest): InputControllerServiceGetEventsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceGetEventsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceGetEventsRequest;
  static deserializeBinaryFromReader(message: InputControllerServiceGetEventsRequest, reader: jspb.BinaryReader): InputControllerServiceGetEventsRequest;
}

export namespace InputControllerServiceGetEventsRequest {
  export type AsObject = {
    controller: string,
  }
}

export class InputControllerServiceGetEventsResponse extends jspb.Message {
  clearEventsList(): void;
  getEventsList(): Array<InputControllerServiceEvent>;
  setEventsList(value: Array<InputControllerServiceEvent>): void;
  addEvents(value?: InputControllerServiceEvent, index?: number): InputControllerServiceEvent;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceGetEventsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceGetEventsResponse): InputControllerServiceGetEventsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceGetEventsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceGetEventsResponse;
  static deserializeBinaryFromReader(message: InputControllerServiceGetEventsResponse, reader: jspb.BinaryReader): InputControllerServiceGetEventsResponse;
}

export namespace InputControllerServiceGetEventsResponse {
  export type AsObject = {
    eventsList: Array<InputControllerServiceEvent.AsObject>,
  }
}

export class InputControllerServiceTriggerEventRequest extends jspb.Message {
  getController(): string;
  setController(value: string): void;

  hasEvent(): boolean;
  clearEvent(): void;
  getEvent(): InputControllerServiceEvent | undefined;
  setEvent(value?: InputControllerServiceEvent): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceTriggerEventRequest.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceTriggerEventRequest): InputControllerServiceTriggerEventRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceTriggerEventRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceTriggerEventRequest;
  static deserializeBinaryFromReader(message: InputControllerServiceTriggerEventRequest, reader: jspb.BinaryReader): InputControllerServiceTriggerEventRequest;
}

export namespace InputControllerServiceTriggerEventRequest {
  export type AsObject = {
    controller: string,
    event?: InputControllerServiceEvent.AsObject,
  }
}

export class InputControllerServiceTriggerEventResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceTriggerEventResponse.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceTriggerEventResponse): InputControllerServiceTriggerEventResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceTriggerEventResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceTriggerEventResponse;
  static deserializeBinaryFromReader(message: InputControllerServiceTriggerEventResponse, reader: jspb.BinaryReader): InputControllerServiceTriggerEventResponse;
}

export namespace InputControllerServiceTriggerEventResponse {
  export type AsObject = {
  }
}

export class InputControllerServiceEvent extends jspb.Message {
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
  toObject(includeInstance?: boolean): InputControllerServiceEvent.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceEvent): InputControllerServiceEvent.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceEvent, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceEvent;
  static deserializeBinaryFromReader(message: InputControllerServiceEvent, reader: jspb.BinaryReader): InputControllerServiceEvent;
}

export namespace InputControllerServiceEvent {
  export type AsObject = {
    time?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    event: string,
    control: string,
    value: number,
  }
}

export class InputControllerServiceStreamEventsRequest extends jspb.Message {
  getController(): string;
  setController(value: string): void;

  clearEventsList(): void;
  getEventsList(): Array<InputControllerServiceStreamEventsRequest.Events>;
  setEventsList(value: Array<InputControllerServiceStreamEventsRequest.Events>): void;
  addEvents(value?: InputControllerServiceStreamEventsRequest.Events, index?: number): InputControllerServiceStreamEventsRequest.Events;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceStreamEventsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceStreamEventsRequest): InputControllerServiceStreamEventsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceStreamEventsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceStreamEventsRequest;
  static deserializeBinaryFromReader(message: InputControllerServiceStreamEventsRequest, reader: jspb.BinaryReader): InputControllerServiceStreamEventsRequest;
}

export namespace InputControllerServiceStreamEventsRequest {
  export type AsObject = {
    controller: string,
    eventsList: Array<InputControllerServiceStreamEventsRequest.Events.AsObject>,
  }

  export class Events extends jspb.Message {
    getControl(): string;
    setControl(value: string): void;

    clearEventsList(): void;
    getEventsList(): Array<string>;
    setEventsList(value: Array<string>): void;
    addEvents(value: string, index?: number): string;

    clearCancelledEventsList(): void;
    getCancelledEventsList(): Array<string>;
    setCancelledEventsList(value: Array<string>): void;
    addCancelledEvents(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Events.AsObject;
    static toObject(includeInstance: boolean, msg: Events): Events.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Events, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Events;
    static deserializeBinaryFromReader(message: Events, reader: jspb.BinaryReader): Events;
  }

  export namespace Events {
    export type AsObject = {
      control: string,
      eventsList: Array<string>,
      cancelledEventsList: Array<string>,
    }
  }
}

export class InputControllerServiceStreamEventsResponse extends jspb.Message {
  hasEvent(): boolean;
  clearEvent(): void;
  getEvent(): InputControllerServiceEvent | undefined;
  setEvent(value?: InputControllerServiceEvent): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceStreamEventsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceStreamEventsResponse): InputControllerServiceStreamEventsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceStreamEventsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceStreamEventsResponse;
  static deserializeBinaryFromReader(message: InputControllerServiceStreamEventsResponse, reader: jspb.BinaryReader): InputControllerServiceStreamEventsResponse;
}

export namespace InputControllerServiceStreamEventsResponse {
  export type AsObject = {
    event?: InputControllerServiceEvent.AsObject,
  }
}

