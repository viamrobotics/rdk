// package: proto.api.component.v1
// file: proto/api/component/v1/input_controller.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class InputControllerServiceControlsRequest extends jspb.Message {
  getController(): string;
  setController(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceControlsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceControlsRequest): InputControllerServiceControlsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceControlsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceControlsRequest;
  static deserializeBinaryFromReader(message: InputControllerServiceControlsRequest, reader: jspb.BinaryReader): InputControllerServiceControlsRequest;
}

export namespace InputControllerServiceControlsRequest {
  export type AsObject = {
    controller: string,
  }
}

export class InputControllerServiceControlsResponse extends jspb.Message {
  clearControlsList(): void;
  getControlsList(): Array<string>;
  setControlsList(value: Array<string>): void;
  addControls(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceControlsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceControlsResponse): InputControllerServiceControlsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceControlsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceControlsResponse;
  static deserializeBinaryFromReader(message: InputControllerServiceControlsResponse, reader: jspb.BinaryReader): InputControllerServiceControlsResponse;
}

export namespace InputControllerServiceControlsResponse {
  export type AsObject = {
    controlsList: Array<string>,
  }
}

export class InputControllerServiceLastEventsRequest extends jspb.Message {
  getController(): string;
  setController(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceLastEventsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceLastEventsRequest): InputControllerServiceLastEventsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceLastEventsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceLastEventsRequest;
  static deserializeBinaryFromReader(message: InputControllerServiceLastEventsRequest, reader: jspb.BinaryReader): InputControllerServiceLastEventsRequest;
}

export namespace InputControllerServiceLastEventsRequest {
  export type AsObject = {
    controller: string,
  }
}

export class InputControllerServiceLastEventsResponse extends jspb.Message {
  clearEventsList(): void;
  getEventsList(): Array<InputControllerServiceEvent>;
  setEventsList(value: Array<InputControllerServiceEvent>): void;
  addEvents(value?: InputControllerServiceEvent, index?: number): InputControllerServiceEvent;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceLastEventsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceLastEventsResponse): InputControllerServiceLastEventsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceLastEventsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceLastEventsResponse;
  static deserializeBinaryFromReader(message: InputControllerServiceLastEventsResponse, reader: jspb.BinaryReader): InputControllerServiceLastEventsResponse;
}

export namespace InputControllerServiceLastEventsResponse {
  export type AsObject = {
    eventsList: Array<InputControllerServiceEvent.AsObject>,
  }
}

export class InputControllerServiceInjectEventRequest extends jspb.Message {
  getController(): string;
  setController(value: string): void;

  hasEvent(): boolean;
  clearEvent(): void;
  getEvent(): InputControllerServiceEvent | undefined;
  setEvent(value?: InputControllerServiceEvent): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceInjectEventRequest.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceInjectEventRequest): InputControllerServiceInjectEventRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceInjectEventRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceInjectEventRequest;
  static deserializeBinaryFromReader(message: InputControllerServiceInjectEventRequest, reader: jspb.BinaryReader): InputControllerServiceInjectEventRequest;
}

export namespace InputControllerServiceInjectEventRequest {
  export type AsObject = {
    controller: string,
    event?: InputControllerServiceEvent.AsObject,
  }
}

export class InputControllerServiceInjectEventResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceInjectEventResponse.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceInjectEventResponse): InputControllerServiceInjectEventResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceInjectEventResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceInjectEventResponse;
  static deserializeBinaryFromReader(message: InputControllerServiceInjectEventResponse, reader: jspb.BinaryReader): InputControllerServiceInjectEventResponse;
}

export namespace InputControllerServiceInjectEventResponse {
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

export class InputControllerServiceEventStreamRequest extends jspb.Message {
  getController(): string;
  setController(value: string): void;

  clearEventsList(): void;
  getEventsList(): Array<InputControllerServiceEventStreamRequest.Events>;
  setEventsList(value: Array<InputControllerServiceEventStreamRequest.Events>): void;
  addEvents(value?: InputControllerServiceEventStreamRequest.Events, index?: number): InputControllerServiceEventStreamRequest.Events;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceEventStreamRequest.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceEventStreamRequest): InputControllerServiceEventStreamRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceEventStreamRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceEventStreamRequest;
  static deserializeBinaryFromReader(message: InputControllerServiceEventStreamRequest, reader: jspb.BinaryReader): InputControllerServiceEventStreamRequest;
}

export namespace InputControllerServiceEventStreamRequest {
  export type AsObject = {
    controller: string,
    eventsList: Array<InputControllerServiceEventStreamRequest.Events.AsObject>,
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

export class InputControllerServiceEventStreamResponse extends jspb.Message {
  clearEventsList(): void;
  getEventsList(): Array<InputControllerServiceEvent>;
  setEventsList(value: Array<InputControllerServiceEvent>): void;
  addEvents(value?: InputControllerServiceEvent, index?: number): InputControllerServiceEvent;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputControllerServiceEventStreamResponse.AsObject;
  static toObject(includeInstance: boolean, msg: InputControllerServiceEventStreamResponse): InputControllerServiceEventStreamResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputControllerServiceEventStreamResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputControllerServiceEventStreamResponse;
  static deserializeBinaryFromReader(message: InputControllerServiceEventStreamResponse, reader: jspb.BinaryReader): InputControllerServiceEventStreamResponse;
}

export namespace InputControllerServiceEventStreamResponse {
  export type AsObject = {
    eventsList: Array<InputControllerServiceEvent.AsObject>,
  }
}

