// package: google.api.servicecontrol.v1
// file: google/api/servicecontrol/v1/metric_value.proto

import * as jspb from "google-protobuf";
import * as google_api_servicecontrol_v1_distribution_pb from "../../../../google/api/servicecontrol/v1/distribution_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class MetricValue extends jspb.Message {
  getLabelsMap(): jspb.Map<string, string>;
  clearLabelsMap(): void;
  hasStartTime(): boolean;
  clearStartTime(): void;
  getStartTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setStartTime(value?: google_protobuf_timestamp_pb.Timestamp): void;

  hasEndTime(): boolean;
  clearEndTime(): void;
  getEndTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setEndTime(value?: google_protobuf_timestamp_pb.Timestamp): void;

  hasBoolValue(): boolean;
  clearBoolValue(): void;
  getBoolValue(): boolean;
  setBoolValue(value: boolean): void;

  hasInt64Value(): boolean;
  clearInt64Value(): void;
  getInt64Value(): number;
  setInt64Value(value: number): void;

  hasDoubleValue(): boolean;
  clearDoubleValue(): void;
  getDoubleValue(): number;
  setDoubleValue(value: number): void;

  hasStringValue(): boolean;
  clearStringValue(): void;
  getStringValue(): string;
  setStringValue(value: string): void;

  hasDistributionValue(): boolean;
  clearDistributionValue(): void;
  getDistributionValue(): google_api_servicecontrol_v1_distribution_pb.Distribution | undefined;
  setDistributionValue(value?: google_api_servicecontrol_v1_distribution_pb.Distribution): void;

  getValueCase(): MetricValue.ValueCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MetricValue.AsObject;
  static toObject(includeInstance: boolean, msg: MetricValue): MetricValue.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MetricValue, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MetricValue;
  static deserializeBinaryFromReader(message: MetricValue, reader: jspb.BinaryReader): MetricValue;
}

export namespace MetricValue {
  export type AsObject = {
    labelsMap: Array<[string, string]>,
    startTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    endTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    boolValue: boolean,
    int64Value: number,
    doubleValue: number,
    stringValue: string,
    distributionValue?: google_api_servicecontrol_v1_distribution_pb.Distribution.AsObject,
  }

  export enum ValueCase {
    VALUE_NOT_SET = 0,
    BOOL_VALUE = 4,
    INT64_VALUE = 5,
    DOUBLE_VALUE = 6,
    STRING_VALUE = 7,
    DISTRIBUTION_VALUE = 8,
  }
}

export class MetricValueSet extends jspb.Message {
  getMetricName(): string;
  setMetricName(value: string): void;

  clearMetricValuesList(): void;
  getMetricValuesList(): Array<MetricValue>;
  setMetricValuesList(value: Array<MetricValue>): void;
  addMetricValues(value?: MetricValue, index?: number): MetricValue;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MetricValueSet.AsObject;
  static toObject(includeInstance: boolean, msg: MetricValueSet): MetricValueSet.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MetricValueSet, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MetricValueSet;
  static deserializeBinaryFromReader(message: MetricValueSet, reader: jspb.BinaryReader): MetricValueSet;
}

export namespace MetricValueSet {
  export type AsObject = {
    metricName: string,
    metricValuesList: Array<MetricValue.AsObject>,
  }
}

