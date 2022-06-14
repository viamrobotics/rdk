
package nmea

import (
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestConnect(t *testing.T) { 
	logger := golog.NewTestLogger(t)
	var g  = RTKGPS{logger: logger}
	url := "http://rtn.dot.ny.gov:8082"
	username := "evelyn"
	password := "checkmate"
	mountPoint := "NJI2"
	
	//create new ntrip client and connect
	_, err := g.Connect(url, username, password, 10)
	test.That(t, err, test.ShouldBeNil)

	//failed to get stream
	_, err = g.GetStream("", 10)
	test.That(t, err, test.ShouldNotBeNil)

	//successful get stream
	_, err = g.GetStream(mountPoint,10)
	test.That(t, err, test.ShouldBeNil)

}