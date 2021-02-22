package main

import (
	"fmt"

	mtigen "github.com/viamrobotics/robotcore/sensor/compass/mti/gen"

	"github.com/edaniels/golog"
)

func main() {
	control := mtigen.XsControlConstruct()
	defer control.Destruct()

	portInfoArray := mtigen.XSScannerScanPorts()
	portInfoArrayPtr := mtigen.SwigcptrXsArrayXsPortInfo(portInfoArray.Swigcptr())

	if portInfoArrayPtr.Empty() {
		golog.Global.Fatal("no devices found")
		return
	}

	mtPort := portInfoArrayPtr.First()

	golog.Global.Infow("found device",
		"id", mtPort.DeviceId().ToString().ToStdString(),
		"port", mtPort.PortName().ToStdString(),
		"baudrate", mtPort.Baudrate(),
	)
	if mtPort.Baudrate() != mtigen.XBR_115k2 {
		golog.Global.Fatalf("unknown baudrate %d", mtPort.Baudrate())
	}

	if !control.OpenPort(mtPort.PortName(), mtPort.Baudrate()) {
		golog.Global.Fatal("failed to open port")
	}

	device := control.Device(mtPort.DeviceId())
	if device.Swigcptr() == 0 {
		golog.Global.Fatal("expected device")
	}

	callback := mtigen.NewCallbackHandler()
	defer mtigen.DeleteCallbackHandler(callback)

	mtigen.AddCallbackHandler(callback, device)

	if !device.GotoMeasurement() {
		golog.Global.Fatal("failed to go to measurement mode")
	}

	for {
		if callback.PacketAvailable() {
			packet := callback.GetNextPacket()
			if packet.ContainsOrientation() {
				quaternion := packet.OrientationQuaternion()
				fmt.Printf("\rq0:%f, q1:%f, q2:%f, q3:%f",
					quaternion.W(),
					quaternion.X(),
					quaternion.Y(),
					quaternion.Z(),
				)

				euler := packet.OrientationEuler()
				fmt.Printf(" |Roll:%f, Pitch:%f, Yaw:%f",
					euler.Roll(),
					euler.Pitch(),
					euler.Yaw(),
				)
			}
		}
	}
}
