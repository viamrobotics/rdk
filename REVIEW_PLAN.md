# Review Plan: RSDK-11726 - Consolidate GetImage and GetImages

## Overview
This plan reviews the branch RSDK-11726 against the design docs to ensure comprehensive, safe migration from GetImage to GetImages.

---

# REVIEW FINDINGS

## Summary
Overall, the branch is **comprehensive and aligns well with the design documents**. The migration from GetImage to GetImages has been implemented correctly with proper test coverage. A few minor issues and observations are noted below.

## ✅ Confirmed Implementations

### Proto Compliance
- ✅ `filter_source_names` field properly used in server.go and client.go
- ✅ `mime_type` field used instead of `format` in Image messages
- ✅ Empty filter returns all sources correctly
- ✅ Duplicate source names validated at server (returns error)
- ✅ Invalid source names return appropriate errors

### NamedImage Struct (camera.go:87-166)
- ✅ Private fields: `data []byte`, `img image.Image`, `mimeType string`
- ✅ Public field: `SourceName string`
- ✅ `Annotations` field for data capture support
- ✅ `NamedImageFromBytes` returns error if data is nil
- ✅ `NamedImageFromBytes` returns error if mimeType is empty
- ✅ `NamedImageFromImage` returns error if img is nil
- ✅ `Image()` method caches decoded image
- ✅ `Bytes()` method caches encoded bytes
- ✅ `ErrMIMETypeBytesMismatch` returned when bytes don't match mimeType

### Server Implementation (server.go:38-86)
- ✅ GetImages RPC handler properly extracts filter_source_names
- ✅ Validates duplicate source names before calling Images()
- ✅ Calls cam.Images() with filter
- ✅ Populates response with mime_type (not format)
- ✅ Returns captured_at timestamp in ResponseMetadata
- ✅ GetImage RPC handler removed

### Client Implementation (client.go:178-213)
- ✅ Images() properly constructs GetImagesRequest with FilterSourceNames
- ✅ Handles nil vs empty filter correctly
- ✅ Converts proto Image to NamedImage using NamedImageFromBytes
- ✅ Uses mime_type from response
- ✅ Image() method returns deprecation error

### Stream Server (robot/web/stream/camera/)
- ✅ VideoSourceFromCamera uses Images() method
- ✅ Feature flag removed (no environment variable check)
- ✅ State machine for source name selection implemented
- ✅ Recovery logic when errors occur
- ✅ Handles cameras with multiple sources
- ✅ GetStreamableNamedImageFromCamera filters for streamable MIME types

### Data Collectors (collectors.go)
- ✅ ReadImage collector calls Images() under the hood
- ✅ Selects image by mime_type if specified
- ✅ Falls back to first image if no match
- ✅ GetImages collector captures all images
- ✅ Adds source_name as classification label

### Test Coverage
- ✅ TestImages: filter_source_names nil, empty, single, multiple, invalid
- ✅ TestNamedImage: constructors, Image(), Bytes(), caching, error cases
- ✅ Server tests: GetImages with filters, duplicates, unknown sources
- ✅ Client tests: Images filtering, order preservation, error handling
- ✅ Stream server tests: source selection, recovery, odd dimensions

### Removed Code
- ✅ `MimeTypeToFormat` mapping removed
- ✅ `FormatToMimeType` mapping removed
- ✅ GetImage RPC handler removed from server.go
- ✅ Feature flag code removed

## ⚠️ Observations (Not Necessarily Issues)

### 1. NamedImageFromImage Defaults to JPEG (camera.go:114)
**Spec says**: Return error if mimeType is empty
**Code does**: Defaults to `utils.MimeTypeJPEG` if empty
**Assessment**: This is a conscious design decision with test coverage (camera_test.go:472-481). The spec may have been updated to allow this behavior for convenience. **No action needed** if this was intentionally changed.

### 2. Bytes() Method Returns ([]byte, error) Not ([]byte, string, error)
**Spec says**: `Bytes(ctx) ([]byte, string, error)` should return mimeType
**Code does**: Returns `([]byte, error)` without mimeType
**Assessment**: The mimeType can be retrieved via `MimeType()` method separately. This is a minor signature deviation. Consider if this is acceptable or if the spec should be updated.

### 3. webcam.go Image() Method Not Deprecated
**Location**: `components/camera/videosource/webcam.go:381-396`
**Observation**: The webcam's `Image()` method wraps `Images()` and returns first image rather than returning a deprecation error. This provides backward compatibility.
**Assessment**: This is likely intentional for the transition period. Should be tracked for eventual removal.

### 4. LazyEncodedImage Still Present
**Spec says**: Remove LazyEncodedImage in Phase 4 (Step 14)
**Status**: Still present in `rimage/lazy_encoded.go` and used in:
  - `robot/web/stream/camera/camera.go:31` (cropToEvenDimensions)
  - `gostream/stream.go:274` (H264 handling)
  - Various vision/rimage utilities
**Assessment**: This is expected as per the phased approach. Step 14 is a later breaking change.

### 5. Empty Byte Slice Handling in NamedImageFromBytes
**Spec says**: Return error if data slice is empty
**Code does**: Only checks for nil (camera.go:98-99), not `len(data) == 0`
**Assessment**: Empty byte slice would fail during Image() decode anyway, but an earlier error would be more user-friendly. **Consider adding**: `if len(data) == 0` check.

## 🔍 Things to Manually Verify

1. **End-to-end test with data manager**: The spec mentions ensuring capturing and syncing GetImages data works. Verify this is covered in integration tests.

2. **gRPC message size limit**: The spec mentions a risk about GetImages exceeding gRPC limits with many sources. The stream server's discovery logic should handle this gracefully.

3. **Performance profiling**: The spec mentions using pprof/htop to verify no degradation. Has this been done?

4. **Proto version compatibility**: Ensure `go.viam.com/api` v0.1.503 contains the correct proto definitions.

---

---

## Phase 1: Proto Compliance Verification

### 1.1 GetImagesRequest Changes
- [ ] Verify `filter_source_names` field is properly used in server.go
- [ ] Verify client.go properly sends filter_source_names in requests
- [ ] Check that empty filter returns all sources
- [ ] Check that invalid filter source names return appropriate errors

### 1.2 Image Message Changes
- [ ] Verify `mime_type` field is used instead of `format`
- [ ] Verify Format field handling is completely removed from server code
- [ ] Check that mime_type is always populated in responses
- [ ] Verify proto version (go.viam.com/api) is compatible

---

## Phase 2: NamedImage Struct Verification

### 2.1 Struct Definition (camera.go)
- [ ] Verify private fields: `data []byte`, `img image.Image`, `mimeType string`
- [ ] Verify public field: `SourceName string`
- [ ] Check Annotations field handling

### 2.2 Constructors
- [ ] **NamedImageFromBytes(sourceName, data, mimeType)**
  - [ ] Returns error if data is nil
  - [ ] Returns error if data slice is empty
  - [ ] Properly sets all fields

- [ ] **NamedImageFromImage(sourceName, img, mimeType)**
  - [ ] Returns error if img is nil
  - [ ] Properly sets all fields

### 2.3 Helper Methods
- [ ] **Image(ctx) (image.Image, error)**
  - [ ] Returns cached img if already populated
  - [ ] Decodes from data bytes if img is nil
  - [ ] Returns error if mimeType is unknown
  - [ ] Returns error if bytes don't match mimeType
  - [ ] Caches decoded image for future calls

- [ ] **Bytes(ctx) ([]byte, string, error)**
  - [ ] Returns cached data if already populated
  - [ ] Encodes from img if data is nil
  - [ ] Returns mimeType along with bytes
  - [ ] Returns error if encoding fails
  - [ ] Caches encoded bytes for future calls

- [ ] **MimeType() string**
  - [ ] Returns the mimeType field

---

## Phase 3: Camera Interface Changes

### 3.1 Images Method Signature
- [ ] Verify signature: `Images(ctx, filterSourceNames []string, extra map[string]interface{}) ([]NamedImage, resource.ResponseMetadata, error)`
- [ ] Check ResponseMetadata includes captured_at timestamp

### 3.2 Image Method Removal
- [ ] Verify Image() method is deprecated/removed
- [ ] Check that calling Image() returns appropriate error
- [ ] Ensure no internal code paths still use Image()

---

## Phase 4: Server Implementation (server.go)

### 4.1 GetImages RPC Handler
- [ ] Properly extracts filter_source_names from request
- [ ] Validates for duplicate source names (returns error)
- [ ] Calls cam.Images() with filter
- [ ] Populates response with mime_type (not format)
- [ ] Returns captured_at timestamp in response metadata

### 4.2 Removed Methods
- [ ] GetImage RPC handler is removed
- [ ] RenderFrame handler is removed (if applicable)

### 4.3 Error Handling
- [ ] Invalid filter source names return proper error
- [ ] Empty response handling (no images returned)
- [ ] Camera errors propagate correctly

---

## Phase 5: Client Implementation (client.go)

### 5.1 Images Method
- [ ] Properly constructs GetImagesRequest with filter_source_names
- [ ] Handles nil vs empty filter correctly
- [ ] Converts proto Image to NamedImage correctly
- [ ] Uses mime_type from response (not format)
- [ ] Handles ResponseMetadata correctly

### 5.2 Removed Methods
- [ ] Image() method is deprecated/returns error
- [ ] No fallback to old GetImage RPC

### 5.3 Stream Method
- [ ] Uses Images() via DecodeImageFromCamera
- [ ] No longer uses Image() method

---

## Phase 6: Stream Server (robot/web/stream/camera/)

### 6.1 VideoSourceFromCamera
- [ ] Uses Images() method (not Image())
- [ ] Feature flag removed (env var STREAM_GET_IMAGES)
- [ ] State machine for source name selection works correctly
- [ ] Recovery logic when errors occur
- [ ] Handles cameras with multiple sources

### 6.2 Streamable Image Selection
- [ ] GetStreamableNamedImageFromCamera handles all MIME types
- [ ] Fallback logic when preferred format unavailable
- [ ] Handles empty image list

### 6.3 Discovery Logic (per spec risk)
- [ ] Initial call discovers available sources
- [ ] Maps source names for subsequent calls
- [ ] Handles gRPC size limit scenarios (large multi-source cameras)

---

## Phase 7: Data Collectors (collectors.go)

### 7.1 ReadImage Collector
- [ ] Calls Images() under the hood
- [ ] Properly selects image by mime_type if specified
- [ ] Falls back to first image if no match
- [ ] Captures timestamp correctly

### 7.2 GetImages Collector
- [ ] Captures all images from Images() call
- [ ] Adds source_name as classification label
- [ ] Includes annotations in capture

---

## Phase 8: Builtin Cameras

### 8.1 Fake Camera
- [ ] Implements new Images() signature
- [ ] Returns proper NamedImage with mime_type
- [ ] Supports filter_source_names

### 8.2 Transform Pipeline
- [ ] Underlying cameras use new Images() API
- [ ] Transforms work with NamedImage

### 8.3 Webcam/Video Sources
- [ ] Implements new Images() signature
- [ ] Returns proper metadata

---

## Phase 9: Test Coverage

### 9.1 Unit Tests
- [ ] NamedImage constructors tested (nil/empty data, nil img)
- [ ] NamedImage.Image() caching tested
- [ ] NamedImage.Bytes() caching tested
- [ ] MimeType mismatch error tested
- [ ] Decode failure error tested

### 9.2 Integration Tests
- [ ] filter_source_names nil vs empty vs specific
- [ ] Duplicate source name validation
- [ ] Invalid source name error
- [ ] Multi-source camera scenarios

### 9.3 Server/Client Tests
- [ ] Round-trip GetImages with filters
- [ ] Metadata (captured_at) preserved
- [ ] Error propagation

### 9.4 Stream Server Tests
- [ ] VideoSourceFromCamera with new Images()
- [ ] Source discovery and selection
- [ ] Recovery from errors

### 9.5 Collector Tests
- [ ] ReadImage uses Images() correctly
- [ ] GetImages collector captures all sources

---

## Phase 10: Performance Verification

### 10.1 No Regression Checks
- [ ] Stream server at 20 FPS doesn't degrade
- [ ] Lazy encoding avoids unnecessary transcoding
- [ ] Caching in NamedImage works correctly

### 10.2 Memory Considerations
- [ ] []NamedImage vs []*NamedImage decision
- [ ] Large image handling

---

## Phase 11: Edge Cases & Error Handling

### 11.1 Edge Cases
- [ ] Camera with zero sources
- [ ] Camera with very many sources (gRPC limit)
- [ ] Empty source name handling
- [ ] Special characters in source names
- [ ] Unsupported MIME types

### 11.2 Error Messages
- [ ] MimeType/bytes mismatch error is informative
- [ ] Invalid filter source name error is actionable
- [ ] Duplicate source name error is clear

---

## Phase 12: Backwards Compatibility

### 12.1 SDK Consumers
- [ ] Old modules calling Image() get clear deprecation error
- [ ] Migration path is documented

### 12.2 Proto Compatibility
- [ ] Server handles clients sending old requests (no filter)
- [ ] Client handles servers returning both format and mime_type

---

## Phase 13: Removed Code Verification

### 13.1 Confirm Removals
- [ ] GetImage RPC handler removed from server.go
- [ ] Image() client method deprecated
- [ ] MimeTypeToFormat mapping removed
- [ ] FormatToMimeType mapping removed
- [ ] LazyEncodedImage handling (if applicable)
- [ ] CheckLazyMIMEType removed
- [ ] GetImageFromGetImages helper removed
- [ ] Feature flag code removed

---

## Phase 14: General Code Review

### 14.1 Code Quality
- [ ] No TODO/FIXME left unaddressed
- [ ] Error wrapping is consistent
- [ ] Logging is appropriate
- [ ] No dead code paths

### 14.2 Documentation
- [ ] Method comments updated for new signatures
- [ ] Deprecation notices are clear
- [ ] Breaking change documentation

### 14.3 Things That May Have Been Missed
- [ ] Search for any remaining "GetImage" references (excluding GetImages)
- [ ] Search for any remaining "Format" enum usage
- [ ] Search for any remaining "RenderFrame" references
- [ ] Check all files modified in recent commits for completeness
- [ ] Verify no panics can occur with nil values
- [ ] Check context cancellation is handled properly

---

## Checklist Summary

| Phase | Items | Priority |
|-------|-------|----------|
| Proto Compliance | 4 | Critical |
| NamedImage Struct | 12 | Critical |
| Camera Interface | 3 | Critical |
| Server Implementation | 7 | Critical |
| Client Implementation | 6 | Critical |
| Stream Server | 6 | High |
| Data Collectors | 4 | High |
| Builtin Cameras | 3 | Medium |
| Test Coverage | 9 | Critical |
| Performance | 3 | High |
| Edge Cases | 8 | High |
| Backwards Compat | 2 | Medium |
| Removed Code | 8 | High |
| General Review | 7 | Medium |

---

## Next Steps

1. Read through each file listed in exploration results
2. Check each item in this plan against the actual code
3. Document any discrepancies or concerns
4. Flag any missing test coverage
5. Identify any security or performance concerns
6. Compare against upstream main branch for unintended changes
