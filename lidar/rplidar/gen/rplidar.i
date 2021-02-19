%module rplidargen

%include <stdint.i>
%include <carrays.i>
%array_functions(uint8_t, byteArray);

%{
	#include "./third_party/rplidar_sdk-release-v1.12.0/sdk/sdk/include/rplidar.h"	
%}

// swig -v -go -cgo -c++ -intgosize 64 rplidar.i

%apply unsigned long { int RESULT_FAIL_BIT }
%include "./third_party/rplidar_sdk-release-v1.12.0/sdk/sdk/include/rplidar.h"
%include "./third_party/rplidar_sdk-release-v1.12.0/sdk/sdk/src/hal/types.h"
%include "./third_party/rplidar_sdk-release-v1.12.0/sdk/sdk/include/rplidar_protocol.h"
%include "./third_party/rplidar_sdk-release-v1.12.0/sdk/sdk/include/rplidar_cmd.h"
%include "./third_party/rplidar_sdk-release-v1.12.0/sdk/sdk/include/rplidar_driver.h"

%array_functions(rplidar_response_measurement_node_hq_t, measurementNodeHqArray)

