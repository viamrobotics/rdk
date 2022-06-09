// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import "strconv"

type BuiltinOptions byte

const (
	BuiltinOptionsNONE                              BuiltinOptions = 0
	BuiltinOptionsConv2DOptions                     BuiltinOptions = 1
	BuiltinOptionsDepthwiseConv2DOptions            BuiltinOptions = 2
	BuiltinOptionsConcatEmbeddingsOptions           BuiltinOptions = 3
	BuiltinOptionsLSHProjectionOptions              BuiltinOptions = 4
	BuiltinOptionsPool2DOptions                     BuiltinOptions = 5
	BuiltinOptionsSVDFOptions                       BuiltinOptions = 6
	BuiltinOptionsRNNOptions                        BuiltinOptions = 7
	BuiltinOptionsFullyConnectedOptions             BuiltinOptions = 8
	BuiltinOptionsSoftmaxOptions                    BuiltinOptions = 9
	BuiltinOptionsConcatenationOptions              BuiltinOptions = 10
	BuiltinOptionsAddOptions                        BuiltinOptions = 11
	BuiltinOptionsL2NormOptions                     BuiltinOptions = 12
	BuiltinOptionsLocalResponseNormalizationOptions BuiltinOptions = 13
	BuiltinOptionsLSTMOptions                       BuiltinOptions = 14
	BuiltinOptionsResizeBilinearOptions             BuiltinOptions = 15
	BuiltinOptionsCallOptions                       BuiltinOptions = 16
	BuiltinOptionsReshapeOptions                    BuiltinOptions = 17
	BuiltinOptionsSkipGramOptions                   BuiltinOptions = 18
	BuiltinOptionsSpaceToDepthOptions               BuiltinOptions = 19
	BuiltinOptionsEmbeddingLookupSparseOptions      BuiltinOptions = 20
	BuiltinOptionsMulOptions                        BuiltinOptions = 21
	BuiltinOptionsPadOptions                        BuiltinOptions = 22
	BuiltinOptionsGatherOptions                     BuiltinOptions = 23
	BuiltinOptionsBatchToSpaceNDOptions             BuiltinOptions = 24
	BuiltinOptionsSpaceToBatchNDOptions             BuiltinOptions = 25
	BuiltinOptionsTransposeOptions                  BuiltinOptions = 26
	BuiltinOptionsReducerOptions                    BuiltinOptions = 27
	BuiltinOptionsSubOptions                        BuiltinOptions = 28
	BuiltinOptionsDivOptions                        BuiltinOptions = 29
	BuiltinOptionsSqueezeOptions                    BuiltinOptions = 30
	BuiltinOptionsSequenceRNNOptions                BuiltinOptions = 31
	BuiltinOptionsStridedSliceOptions               BuiltinOptions = 32
	BuiltinOptionsExpOptions                        BuiltinOptions = 33
	BuiltinOptionsTopKV2Options                     BuiltinOptions = 34
	BuiltinOptionsSplitOptions                      BuiltinOptions = 35
	BuiltinOptionsLogSoftmaxOptions                 BuiltinOptions = 36
	BuiltinOptionsCastOptions                       BuiltinOptions = 37
	BuiltinOptionsDequantizeOptions                 BuiltinOptions = 38
	BuiltinOptionsMaximumMinimumOptions             BuiltinOptions = 39
	BuiltinOptionsArgMaxOptions                     BuiltinOptions = 40
	BuiltinOptionsLessOptions                       BuiltinOptions = 41
	BuiltinOptionsNegOptions                        BuiltinOptions = 42
	BuiltinOptionsPadV2Options                      BuiltinOptions = 43
	BuiltinOptionsGreaterOptions                    BuiltinOptions = 44
	BuiltinOptionsGreaterEqualOptions               BuiltinOptions = 45
	BuiltinOptionsLessEqualOptions                  BuiltinOptions = 46
	BuiltinOptionsSelectOptions                     BuiltinOptions = 47
	BuiltinOptionsSliceOptions                      BuiltinOptions = 48
	BuiltinOptionsTransposeConvOptions              BuiltinOptions = 49
	BuiltinOptionsSparseToDenseOptions              BuiltinOptions = 50
	BuiltinOptionsTileOptions                       BuiltinOptions = 51
	BuiltinOptionsExpandDimsOptions                 BuiltinOptions = 52
	BuiltinOptionsEqualOptions                      BuiltinOptions = 53
	BuiltinOptionsNotEqualOptions                   BuiltinOptions = 54
	BuiltinOptionsShapeOptions                      BuiltinOptions = 55
	BuiltinOptionsPowOptions                        BuiltinOptions = 56
	BuiltinOptionsArgMinOptions                     BuiltinOptions = 57
	BuiltinOptionsFakeQuantOptions                  BuiltinOptions = 58
	BuiltinOptionsPackOptions                       BuiltinOptions = 59
	BuiltinOptionsLogicalOrOptions                  BuiltinOptions = 60
	BuiltinOptionsOneHotOptions                     BuiltinOptions = 61
	BuiltinOptionsLogicalAndOptions                 BuiltinOptions = 62
	BuiltinOptionsLogicalNotOptions                 BuiltinOptions = 63
	BuiltinOptionsUnpackOptions                     BuiltinOptions = 64
	BuiltinOptionsFloorDivOptions                   BuiltinOptions = 65
	BuiltinOptionsSquareOptions                     BuiltinOptions = 66
	BuiltinOptionsZerosLikeOptions                  BuiltinOptions = 67
	BuiltinOptionsFillOptions                       BuiltinOptions = 68
	BuiltinOptionsBidirectionalSequenceLSTMOptions  BuiltinOptions = 69
	BuiltinOptionsBidirectionalSequenceRNNOptions   BuiltinOptions = 70
	BuiltinOptionsUnidirectionalSequenceLSTMOptions BuiltinOptions = 71
	BuiltinOptionsFloorModOptions                   BuiltinOptions = 72
	BuiltinOptionsRangeOptions                      BuiltinOptions = 73
	BuiltinOptionsResizeNearestNeighborOptions      BuiltinOptions = 74
	BuiltinOptionsLeakyReluOptions                  BuiltinOptions = 75
	BuiltinOptionsSquaredDifferenceOptions          BuiltinOptions = 76
	BuiltinOptionsMirrorPadOptions                  BuiltinOptions = 77
	BuiltinOptionsAbsOptions                        BuiltinOptions = 78
	BuiltinOptionsSplitVOptions                     BuiltinOptions = 79
	BuiltinOptionsUniqueOptions                     BuiltinOptions = 80
	BuiltinOptionsReverseV2Options                  BuiltinOptions = 81
	BuiltinOptionsAddNOptions                       BuiltinOptions = 82
	BuiltinOptionsGatherNdOptions                   BuiltinOptions = 83
	BuiltinOptionsCosOptions                        BuiltinOptions = 84
	BuiltinOptionsWhereOptions                      BuiltinOptions = 85
	BuiltinOptionsRankOptions                       BuiltinOptions = 86
	BuiltinOptionsReverseSequenceOptions            BuiltinOptions = 87
	BuiltinOptionsMatrixDiagOptions                 BuiltinOptions = 88
	BuiltinOptionsQuantizeOptions                   BuiltinOptions = 89
	BuiltinOptionsMatrixSetDiagOptions              BuiltinOptions = 90
	BuiltinOptionsHardSwishOptions                  BuiltinOptions = 91
	BuiltinOptionsIfOptions                         BuiltinOptions = 92
	BuiltinOptionsWhileOptions                      BuiltinOptions = 93
	BuiltinOptionsDepthToSpaceOptions               BuiltinOptions = 94
	BuiltinOptionsNonMaxSuppressionV4Options        BuiltinOptions = 95
	BuiltinOptionsNonMaxSuppressionV5Options        BuiltinOptions = 96
	BuiltinOptionsScatterNdOptions                  BuiltinOptions = 97
	BuiltinOptionsSelectV2Options                   BuiltinOptions = 98
	BuiltinOptionsDensifyOptions                    BuiltinOptions = 99
	BuiltinOptionsSegmentSumOptions                 BuiltinOptions = 100
	BuiltinOptionsBatchMatMulOptions                BuiltinOptions = 101
	BuiltinOptionsCumsumOptions                     BuiltinOptions = 102
	BuiltinOptionsCallOnceOptions                   BuiltinOptions = 103
	BuiltinOptionsBroadcastToOptions                BuiltinOptions = 104
	BuiltinOptionsRfft2dOptions                     BuiltinOptions = 105
	BuiltinOptionsConv3DOptions                     BuiltinOptions = 106
	BuiltinOptionsHashtableOptions                  BuiltinOptions = 107
	BuiltinOptionsHashtableFindOptions              BuiltinOptions = 108
	BuiltinOptionsHashtableImportOptions            BuiltinOptions = 109
	BuiltinOptionsHashtableSizeOptions              BuiltinOptions = 110
	BuiltinOptionsVarHandleOptions                  BuiltinOptions = 111
	BuiltinOptionsReadVariableOptions               BuiltinOptions = 112
	BuiltinOptionsAssignVariableOptions             BuiltinOptions = 113
	BuiltinOptionsRandomOptions                     BuiltinOptions = 114
	BuiltinOptionsBucketizeOptions                  BuiltinOptions = 115
	BuiltinOptionsGeluOptions                       BuiltinOptions = 116
	BuiltinOptionsDynamicUpdateSliceOptions         BuiltinOptions = 117
)

var EnumNamesBuiltinOptions = map[BuiltinOptions]string{
	BuiltinOptionsNONE:                              "NONE",
	BuiltinOptionsConv2DOptions:                     "Conv2DOptions",
	BuiltinOptionsDepthwiseConv2DOptions:            "DepthwiseConv2DOptions",
	BuiltinOptionsConcatEmbeddingsOptions:           "ConcatEmbeddingsOptions",
	BuiltinOptionsLSHProjectionOptions:              "LSHProjectionOptions",
	BuiltinOptionsPool2DOptions:                     "Pool2DOptions",
	BuiltinOptionsSVDFOptions:                       "SVDFOptions",
	BuiltinOptionsRNNOptions:                        "RNNOptions",
	BuiltinOptionsFullyConnectedOptions:             "FullyConnectedOptions",
	BuiltinOptionsSoftmaxOptions:                    "SoftmaxOptions",
	BuiltinOptionsConcatenationOptions:              "ConcatenationOptions",
	BuiltinOptionsAddOptions:                        "AddOptions",
	BuiltinOptionsL2NormOptions:                     "L2NormOptions",
	BuiltinOptionsLocalResponseNormalizationOptions: "LocalResponseNormalizationOptions",
	BuiltinOptionsLSTMOptions:                       "LSTMOptions",
	BuiltinOptionsResizeBilinearOptions:             "ResizeBilinearOptions",
	BuiltinOptionsCallOptions:                       "CallOptions",
	BuiltinOptionsReshapeOptions:                    "ReshapeOptions",
	BuiltinOptionsSkipGramOptions:                   "SkipGramOptions",
	BuiltinOptionsSpaceToDepthOptions:               "SpaceToDepthOptions",
	BuiltinOptionsEmbeddingLookupSparseOptions:      "EmbeddingLookupSparseOptions",
	BuiltinOptionsMulOptions:                        "MulOptions",
	BuiltinOptionsPadOptions:                        "PadOptions",
	BuiltinOptionsGatherOptions:                     "GatherOptions",
	BuiltinOptionsBatchToSpaceNDOptions:             "BatchToSpaceNDOptions",
	BuiltinOptionsSpaceToBatchNDOptions:             "SpaceToBatchNDOptions",
	BuiltinOptionsTransposeOptions:                  "TransposeOptions",
	BuiltinOptionsReducerOptions:                    "ReducerOptions",
	BuiltinOptionsSubOptions:                        "SubOptions",
	BuiltinOptionsDivOptions:                        "DivOptions",
	BuiltinOptionsSqueezeOptions:                    "SqueezeOptions",
	BuiltinOptionsSequenceRNNOptions:                "SequenceRNNOptions",
	BuiltinOptionsStridedSliceOptions:               "StridedSliceOptions",
	BuiltinOptionsExpOptions:                        "ExpOptions",
	BuiltinOptionsTopKV2Options:                     "TopKV2Options",
	BuiltinOptionsSplitOptions:                      "SplitOptions",
	BuiltinOptionsLogSoftmaxOptions:                 "LogSoftmaxOptions",
	BuiltinOptionsCastOptions:                       "CastOptions",
	BuiltinOptionsDequantizeOptions:                 "DequantizeOptions",
	BuiltinOptionsMaximumMinimumOptions:             "MaximumMinimumOptions",
	BuiltinOptionsArgMaxOptions:                     "ArgMaxOptions",
	BuiltinOptionsLessOptions:                       "LessOptions",
	BuiltinOptionsNegOptions:                        "NegOptions",
	BuiltinOptionsPadV2Options:                      "PadV2Options",
	BuiltinOptionsGreaterOptions:                    "GreaterOptions",
	BuiltinOptionsGreaterEqualOptions:               "GreaterEqualOptions",
	BuiltinOptionsLessEqualOptions:                  "LessEqualOptions",
	BuiltinOptionsSelectOptions:                     "SelectOptions",
	BuiltinOptionsSliceOptions:                      "SliceOptions",
	BuiltinOptionsTransposeConvOptions:              "TransposeConvOptions",
	BuiltinOptionsSparseToDenseOptions:              "SparseToDenseOptions",
	BuiltinOptionsTileOptions:                       "TileOptions",
	BuiltinOptionsExpandDimsOptions:                 "ExpandDimsOptions",
	BuiltinOptionsEqualOptions:                      "EqualOptions",
	BuiltinOptionsNotEqualOptions:                   "NotEqualOptions",
	BuiltinOptionsShapeOptions:                      "ShapeOptions",
	BuiltinOptionsPowOptions:                        "PowOptions",
	BuiltinOptionsArgMinOptions:                     "ArgMinOptions",
	BuiltinOptionsFakeQuantOptions:                  "FakeQuantOptions",
	BuiltinOptionsPackOptions:                       "PackOptions",
	BuiltinOptionsLogicalOrOptions:                  "LogicalOrOptions",
	BuiltinOptionsOneHotOptions:                     "OneHotOptions",
	BuiltinOptionsLogicalAndOptions:                 "LogicalAndOptions",
	BuiltinOptionsLogicalNotOptions:                 "LogicalNotOptions",
	BuiltinOptionsUnpackOptions:                     "UnpackOptions",
	BuiltinOptionsFloorDivOptions:                   "FloorDivOptions",
	BuiltinOptionsSquareOptions:                     "SquareOptions",
	BuiltinOptionsZerosLikeOptions:                  "ZerosLikeOptions",
	BuiltinOptionsFillOptions:                       "FillOptions",
	BuiltinOptionsBidirectionalSequenceLSTMOptions:  "BidirectionalSequenceLSTMOptions",
	BuiltinOptionsBidirectionalSequenceRNNOptions:   "BidirectionalSequenceRNNOptions",
	BuiltinOptionsUnidirectionalSequenceLSTMOptions: "UnidirectionalSequenceLSTMOptions",
	BuiltinOptionsFloorModOptions:                   "FloorModOptions",
	BuiltinOptionsRangeOptions:                      "RangeOptions",
	BuiltinOptionsResizeNearestNeighborOptions:      "ResizeNearestNeighborOptions",
	BuiltinOptionsLeakyReluOptions:                  "LeakyReluOptions",
	BuiltinOptionsSquaredDifferenceOptions:          "SquaredDifferenceOptions",
	BuiltinOptionsMirrorPadOptions:                  "MirrorPadOptions",
	BuiltinOptionsAbsOptions:                        "AbsOptions",
	BuiltinOptionsSplitVOptions:                     "SplitVOptions",
	BuiltinOptionsUniqueOptions:                     "UniqueOptions",
	BuiltinOptionsReverseV2Options:                  "ReverseV2Options",
	BuiltinOptionsAddNOptions:                       "AddNOptions",
	BuiltinOptionsGatherNdOptions:                   "GatherNdOptions",
	BuiltinOptionsCosOptions:                        "CosOptions",
	BuiltinOptionsWhereOptions:                      "WhereOptions",
	BuiltinOptionsRankOptions:                       "RankOptions",
	BuiltinOptionsReverseSequenceOptions:            "ReverseSequenceOptions",
	BuiltinOptionsMatrixDiagOptions:                 "MatrixDiagOptions",
	BuiltinOptionsQuantizeOptions:                   "QuantizeOptions",
	BuiltinOptionsMatrixSetDiagOptions:              "MatrixSetDiagOptions",
	BuiltinOptionsHardSwishOptions:                  "HardSwishOptions",
	BuiltinOptionsIfOptions:                         "IfOptions",
	BuiltinOptionsWhileOptions:                      "WhileOptions",
	BuiltinOptionsDepthToSpaceOptions:               "DepthToSpaceOptions",
	BuiltinOptionsNonMaxSuppressionV4Options:        "NonMaxSuppressionV4Options",
	BuiltinOptionsNonMaxSuppressionV5Options:        "NonMaxSuppressionV5Options",
	BuiltinOptionsScatterNdOptions:                  "ScatterNdOptions",
	BuiltinOptionsSelectV2Options:                   "SelectV2Options",
	BuiltinOptionsDensifyOptions:                    "DensifyOptions",
	BuiltinOptionsSegmentSumOptions:                 "SegmentSumOptions",
	BuiltinOptionsBatchMatMulOptions:                "BatchMatMulOptions",
	BuiltinOptionsCumsumOptions:                     "CumsumOptions",
	BuiltinOptionsCallOnceOptions:                   "CallOnceOptions",
	BuiltinOptionsBroadcastToOptions:                "BroadcastToOptions",
	BuiltinOptionsRfft2dOptions:                     "Rfft2dOptions",
	BuiltinOptionsConv3DOptions:                     "Conv3DOptions",
	BuiltinOptionsHashtableOptions:                  "HashtableOptions",
	BuiltinOptionsHashtableFindOptions:              "HashtableFindOptions",
	BuiltinOptionsHashtableImportOptions:            "HashtableImportOptions",
	BuiltinOptionsHashtableSizeOptions:              "HashtableSizeOptions",
	BuiltinOptionsVarHandleOptions:                  "VarHandleOptions",
	BuiltinOptionsReadVariableOptions:               "ReadVariableOptions",
	BuiltinOptionsAssignVariableOptions:             "AssignVariableOptions",
	BuiltinOptionsRandomOptions:                     "RandomOptions",
	BuiltinOptionsBucketizeOptions:                  "BucketizeOptions",
	BuiltinOptionsGeluOptions:                       "GeluOptions",
	BuiltinOptionsDynamicUpdateSliceOptions:         "DynamicUpdateSliceOptions",
}

var EnumValuesBuiltinOptions = map[string]BuiltinOptions{
	"NONE":                              BuiltinOptionsNONE,
	"Conv2DOptions":                     BuiltinOptionsConv2DOptions,
	"DepthwiseConv2DOptions":            BuiltinOptionsDepthwiseConv2DOptions,
	"ConcatEmbeddingsOptions":           BuiltinOptionsConcatEmbeddingsOptions,
	"LSHProjectionOptions":              BuiltinOptionsLSHProjectionOptions,
	"Pool2DOptions":                     BuiltinOptionsPool2DOptions,
	"SVDFOptions":                       BuiltinOptionsSVDFOptions,
	"RNNOptions":                        BuiltinOptionsRNNOptions,
	"FullyConnectedOptions":             BuiltinOptionsFullyConnectedOptions,
	"SoftmaxOptions":                    BuiltinOptionsSoftmaxOptions,
	"ConcatenationOptions":              BuiltinOptionsConcatenationOptions,
	"AddOptions":                        BuiltinOptionsAddOptions,
	"L2NormOptions":                     BuiltinOptionsL2NormOptions,
	"LocalResponseNormalizationOptions": BuiltinOptionsLocalResponseNormalizationOptions,
	"LSTMOptions":                       BuiltinOptionsLSTMOptions,
	"ResizeBilinearOptions":             BuiltinOptionsResizeBilinearOptions,
	"CallOptions":                       BuiltinOptionsCallOptions,
	"ReshapeOptions":                    BuiltinOptionsReshapeOptions,
	"SkipGramOptions":                   BuiltinOptionsSkipGramOptions,
	"SpaceToDepthOptions":               BuiltinOptionsSpaceToDepthOptions,
	"EmbeddingLookupSparseOptions":      BuiltinOptionsEmbeddingLookupSparseOptions,
	"MulOptions":                        BuiltinOptionsMulOptions,
	"PadOptions":                        BuiltinOptionsPadOptions,
	"GatherOptions":                     BuiltinOptionsGatherOptions,
	"BatchToSpaceNDOptions":             BuiltinOptionsBatchToSpaceNDOptions,
	"SpaceToBatchNDOptions":             BuiltinOptionsSpaceToBatchNDOptions,
	"TransposeOptions":                  BuiltinOptionsTransposeOptions,
	"ReducerOptions":                    BuiltinOptionsReducerOptions,
	"SubOptions":                        BuiltinOptionsSubOptions,
	"DivOptions":                        BuiltinOptionsDivOptions,
	"SqueezeOptions":                    BuiltinOptionsSqueezeOptions,
	"SequenceRNNOptions":                BuiltinOptionsSequenceRNNOptions,
	"StridedSliceOptions":               BuiltinOptionsStridedSliceOptions,
	"ExpOptions":                        BuiltinOptionsExpOptions,
	"TopKV2Options":                     BuiltinOptionsTopKV2Options,
	"SplitOptions":                      BuiltinOptionsSplitOptions,
	"LogSoftmaxOptions":                 BuiltinOptionsLogSoftmaxOptions,
	"CastOptions":                       BuiltinOptionsCastOptions,
	"DequantizeOptions":                 BuiltinOptionsDequantizeOptions,
	"MaximumMinimumOptions":             BuiltinOptionsMaximumMinimumOptions,
	"ArgMaxOptions":                     BuiltinOptionsArgMaxOptions,
	"LessOptions":                       BuiltinOptionsLessOptions,
	"NegOptions":                        BuiltinOptionsNegOptions,
	"PadV2Options":                      BuiltinOptionsPadV2Options,
	"GreaterOptions":                    BuiltinOptionsGreaterOptions,
	"GreaterEqualOptions":               BuiltinOptionsGreaterEqualOptions,
	"LessEqualOptions":                  BuiltinOptionsLessEqualOptions,
	"SelectOptions":                     BuiltinOptionsSelectOptions,
	"SliceOptions":                      BuiltinOptionsSliceOptions,
	"TransposeConvOptions":              BuiltinOptionsTransposeConvOptions,
	"SparseToDenseOptions":              BuiltinOptionsSparseToDenseOptions,
	"TileOptions":                       BuiltinOptionsTileOptions,
	"ExpandDimsOptions":                 BuiltinOptionsExpandDimsOptions,
	"EqualOptions":                      BuiltinOptionsEqualOptions,
	"NotEqualOptions":                   BuiltinOptionsNotEqualOptions,
	"ShapeOptions":                      BuiltinOptionsShapeOptions,
	"PowOptions":                        BuiltinOptionsPowOptions,
	"ArgMinOptions":                     BuiltinOptionsArgMinOptions,
	"FakeQuantOptions":                  BuiltinOptionsFakeQuantOptions,
	"PackOptions":                       BuiltinOptionsPackOptions,
	"LogicalOrOptions":                  BuiltinOptionsLogicalOrOptions,
	"OneHotOptions":                     BuiltinOptionsOneHotOptions,
	"LogicalAndOptions":                 BuiltinOptionsLogicalAndOptions,
	"LogicalNotOptions":                 BuiltinOptionsLogicalNotOptions,
	"UnpackOptions":                     BuiltinOptionsUnpackOptions,
	"FloorDivOptions":                   BuiltinOptionsFloorDivOptions,
	"SquareOptions":                     BuiltinOptionsSquareOptions,
	"ZerosLikeOptions":                  BuiltinOptionsZerosLikeOptions,
	"FillOptions":                       BuiltinOptionsFillOptions,
	"BidirectionalSequenceLSTMOptions":  BuiltinOptionsBidirectionalSequenceLSTMOptions,
	"BidirectionalSequenceRNNOptions":   BuiltinOptionsBidirectionalSequenceRNNOptions,
	"UnidirectionalSequenceLSTMOptions": BuiltinOptionsUnidirectionalSequenceLSTMOptions,
	"FloorModOptions":                   BuiltinOptionsFloorModOptions,
	"RangeOptions":                      BuiltinOptionsRangeOptions,
	"ResizeNearestNeighborOptions":      BuiltinOptionsResizeNearestNeighborOptions,
	"LeakyReluOptions":                  BuiltinOptionsLeakyReluOptions,
	"SquaredDifferenceOptions":          BuiltinOptionsSquaredDifferenceOptions,
	"MirrorPadOptions":                  BuiltinOptionsMirrorPadOptions,
	"AbsOptions":                        BuiltinOptionsAbsOptions,
	"SplitVOptions":                     BuiltinOptionsSplitVOptions,
	"UniqueOptions":                     BuiltinOptionsUniqueOptions,
	"ReverseV2Options":                  BuiltinOptionsReverseV2Options,
	"AddNOptions":                       BuiltinOptionsAddNOptions,
	"GatherNdOptions":                   BuiltinOptionsGatherNdOptions,
	"CosOptions":                        BuiltinOptionsCosOptions,
	"WhereOptions":                      BuiltinOptionsWhereOptions,
	"RankOptions":                       BuiltinOptionsRankOptions,
	"ReverseSequenceOptions":            BuiltinOptionsReverseSequenceOptions,
	"MatrixDiagOptions":                 BuiltinOptionsMatrixDiagOptions,
	"QuantizeOptions":                   BuiltinOptionsQuantizeOptions,
	"MatrixSetDiagOptions":              BuiltinOptionsMatrixSetDiagOptions,
	"HardSwishOptions":                  BuiltinOptionsHardSwishOptions,
	"IfOptions":                         BuiltinOptionsIfOptions,
	"WhileOptions":                      BuiltinOptionsWhileOptions,
	"DepthToSpaceOptions":               BuiltinOptionsDepthToSpaceOptions,
	"NonMaxSuppressionV4Options":        BuiltinOptionsNonMaxSuppressionV4Options,
	"NonMaxSuppressionV5Options":        BuiltinOptionsNonMaxSuppressionV5Options,
	"ScatterNdOptions":                  BuiltinOptionsScatterNdOptions,
	"SelectV2Options":                   BuiltinOptionsSelectV2Options,
	"DensifyOptions":                    BuiltinOptionsDensifyOptions,
	"SegmentSumOptions":                 BuiltinOptionsSegmentSumOptions,
	"BatchMatMulOptions":                BuiltinOptionsBatchMatMulOptions,
	"CumsumOptions":                     BuiltinOptionsCumsumOptions,
	"CallOnceOptions":                   BuiltinOptionsCallOnceOptions,
	"BroadcastToOptions":                BuiltinOptionsBroadcastToOptions,
	"Rfft2dOptions":                     BuiltinOptionsRfft2dOptions,
	"Conv3DOptions":                     BuiltinOptionsConv3DOptions,
	"HashtableOptions":                  BuiltinOptionsHashtableOptions,
	"HashtableFindOptions":              BuiltinOptionsHashtableFindOptions,
	"HashtableImportOptions":            BuiltinOptionsHashtableImportOptions,
	"HashtableSizeOptions":              BuiltinOptionsHashtableSizeOptions,
	"VarHandleOptions":                  BuiltinOptionsVarHandleOptions,
	"ReadVariableOptions":               BuiltinOptionsReadVariableOptions,
	"AssignVariableOptions":             BuiltinOptionsAssignVariableOptions,
	"RandomOptions":                     BuiltinOptionsRandomOptions,
	"BucketizeOptions":                  BuiltinOptionsBucketizeOptions,
	"GeluOptions":                       BuiltinOptionsGeluOptions,
	"DynamicUpdateSliceOptions":         BuiltinOptionsDynamicUpdateSliceOptions,
}

func (v BuiltinOptions) String() string {
	if s, ok := EnumNamesBuiltinOptions[v]; ok {
		return s
	}
	return "BuiltinOptions(" + strconv.FormatInt(int64(v), 10) + ")"
}
