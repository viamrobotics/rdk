// export_collectors_test.go adds functionality to the package that we only want to use and expose during testing.
package arm

// Exported variables for testing collectors, see unexported collectors for implementation details.
var (
	NewEndPositionCollector    = newEndPositionCollector
	NewJointPositionsCollector = newJointPositionsCollector
)
