boat: samples/boat1/cmd.go
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) -o $(BIN_OUTPUT_PATH)/boat samples/boat1/cmd.go

boat2: samples/boat2/cmd.go
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) -o $(BIN_OUTPUT_PATH)/boat2 samples/boat2/cmd.go

gpstest: samples/gpsTest/cmd.go
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/gpstest samples/gpsTest/cmd.go

resetbox: samples/resetbox/cmd.go
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) -o $(BIN_OUTPUT_PATH)/resetbox samples/resetbox/cmd.go

gamepad: samples/gamepad/cmd.go
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) -o $(BIN_OUTPUT_PATH)/gamepad samples/gamepad/cmd.go
