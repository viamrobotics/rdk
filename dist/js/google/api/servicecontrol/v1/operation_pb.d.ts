// package: google.api.servicecontrol.v1
// file: google/api/servicecontrol/v1/operation.proto

import * as jspb from "google-protobuf";
import * as google_api_servicecontrol_v1_log_entry_pb from "../../../../google/api/servicecontrol/v1/log_entry_pb";
import * as google_api_servicecontrol_v1_metric_value_pb from "../../../../google/api/servicecontrol/v1/metric_value_pb";
import * as google_protobuf_any_pb from "google-protobuf/google/protobuf/any_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class Operation extends jspb.Message {
  getOperationId(): string;
  setOperationId(value: string): void;

  getOperationName(): string;
  setOperationName(value: string): void;

  getConsumerId(): string;
  setConsumerId(value: string): void;

  hasStartTime(): boolean;
  clearStartTime(): void;
  getStartTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setStartTime(value?: google_protobuf_timestamp_pb.Timestamp): void;

  hasEndTime(): boolean;
  clearEndTime(): void;
  getEndTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setEndTime(value?: google_protobuf_timestamp_pb.Timestamp): void;

  getLabelsMap(): jspb.Map<string, string>;
  clearLabelsMap(): void;
  clearMetricValueSetsList(): void;
  getMetricValueSetsList(): Array<google_api_servicecontrol_v1_metric_value_pb.MetricValueSet>;
  setMetricValueSetsList(value: Array<google_api_servicecontrol_v1_metric_value_pb.MetricValueSet>): void;
  addMetricValueSets(value?: google_api_servicecontrol_v1_metric_value_pb.MetricValueSet, index?: number): google_api_servicecontrol_v1_metric_value_pb.MetricValueSet;

  clearLogEntriesList(): void;
  getLogEntriesList(): Array<google_api_servicecontrol_v1_log_entry_pb.LogEntry>;
  setLogEntriesList(value: Array<google_api_servicecontrol_v1_log_entry_pb.LogEntry>): void;
  addLogEntries(value?: google_api_servicecontrol_v1_log_entry_pb.LogEntry, index?: number): google_api_servicecontrol_v1_log_entry_pb.LogEntry;

  getImportance(): Operation.ImportanceMap[keyof Operation.ImportanceMap];
  setImportance(value: Operation.ImportanceMap[keyof Operation.ImportanceMap]): void;

  clearExtensionsList(): void;
  getExtensionsList(): Array<google_protobuf_any_pb.Any>;
  setExtensionsList(value: Array<google_protobuf_any_pb.Any>): void;
  addExtensions(value?: google_protobuf_any_pb.Any, index?: number): google_protobuf_any_pb.Any;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Operation.AsObject;
  static toObject(includeInstance: boolean, msg: Operation): Operation.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Operation, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Operation;
  static deserializeBinaryFromReader(message: Operation, reader: jspb.BinaryReader): Operation;
}

export namespace Operation {
  export type AsObject = {
    operationId: string,
    operationName: string,
    consumerId: string,
    startTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    endTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    labelsMap: Array<[string, string]>,
    metricValueSetsList: Array<google_api_servicecontrol_v1_metric_value_pb.MetricValueSet.AsObject>,
    logEntriesList: Array<google_api_servicecontrol_v1_log_entry_pb.LogEntry.AsObject>,
    importance: Operation.ImportanceMap[keyof Operation.ImportanceMap],
    extensionsList: Array<google_protobuf_any_pb.Any.AsObject>,
  }

  export interface ImportanceMap {
    LOW: 0;
    HIGH: 1;
  }

  export const Importance: ImportanceMap;
}

