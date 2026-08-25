// Package streaming implements the ability to stream joint positions to an arm resource:
// waypoints pushed to a session are time-parameterized on the fly and trickled into the
// arm's MoveThroughJointPositionsStreamed RPC, keeping a small runway of trajectory
// buffered in the arm.
//
// See README.md in this directory for the DoCommand protocol, usage guidance, and an
// architecture overview, and examples/armstreaming for a runnable client.
package streaming
