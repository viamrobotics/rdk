// Package main is the work-in-progress robotic boat from Viam.
package main

import (
	"bufio"
	"io"
	"time"

	"context"
	"flag"
	"fmt"

	"io/ioutil"
	"net/http"

	"go.viam.com/utils"

	"go.viam.com/core/config"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/services/web"
	webserver "go.viam.com/core/web/server"

	"github.com/edaniels/golog"
	"github.com/go-gnss/ntrip"
	slib "github.com/jacobsa/go-serial/serial"
)

var logger = golog.NewDevelopmentLogger("gpsTest")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

//generate sourcetable for different mountpoints on gpsnetwork
// good for debugging or finding a new working mountpoint
func ExampleNewClientRequest_sourcetable(urlStr string, username string, password string) (err error) {

	// talk to the gps network, looking for mount points
	req, _ := ntrip.NewClientRequest(urlStr)
	req.SetBasicAuth(username, password)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("error making NTRIP request: %s\n", err)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("received non-200 response code: %d", resp.StatusCode)
		return err
	}

	defer resp.Body.Close()
	// read all of the mount points we got back
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error reading from response body")
	}
	// print the mountpoints
	sourcetable, _ := ntrip.ParseSourcetable(string(data))
	fmt.Printf("caster has %d mountpoints available\n", len(sourcetable.Mounts))
	for i := 0; i < len(sourcetable.Mounts); i++ {
		fmt.Println(sourcetable.Mounts[i])
	}

	return nil
}

// setup the system as a ntrip client, and write corrections to the gps via serial
func ExampleNewClientRequest(urlStr string, username string, password string, gpsport string, baud uint) (err error) {

	// talk to the gps network, looking for mount points
	req, _ := ntrip.NewClientRequest(urlStr)
	req.SetBasicAuth(username, password)
	resp, err := http.DefaultClient.Do(req)
	reconnFlag := 0
	if err != nil {
		fmt.Printf("error making NTRIP request: %s\n", err)
		reconnFlag = 1
		fmt.Println(reconnFlag)
		// return err
	} else if resp.StatusCode != http.StatusOK {
		fmt.Printf("received non-200 response code: %d", resp.StatusCode)
		reconnFlag = 1
	}

	for reconnFlag == 1 {
		fmt.Println("yo")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("error making NTRIP request: %s\n", err)
			// return err
		} else if resp.StatusCode != http.StatusOK {
			fmt.Printf("received non-200 response code: %d", resp.StatusCode)
		} else {
			reconnFlag = 0
		}
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("received non-200 response code: %d", resp.StatusCode)
		// return err
	}
	defer resp.Body.Close()
	fmt.Printf("Status Code : %d \n", resp.StatusCode)

	// setup port to write to
	options := slib.OpenOptions{
		BaudRate:        baud,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}
	options.PortName = gpsport
	port, err := slib.Open(options)
	w := bufio.NewWriter(port)

	// Read from resp.Body until EOF
	r := io.TeeReader(resp.Body, w)
	io.ReadAll(r)

	if err != nil {
		fmt.Printf("Error with RTCM stream: %s\n", err)
		return err
	}

	return nil

}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()
	cfg, err := config.Read(flag.Arg(0))
	if err != nil {
		return err
	}

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close()

	webOpts := web.NewOptions()
	webOpts.Insecure = true

	// gpsport := "/dev/serial/by-id/usb-u-blox_AG_-_www.u-blox.com_u-blox_GNSS_receiver-if00"

	// go ExampleNewClientRequest_sourcetable("http://rtn.dot.ny.gov:8080","viam","checkmate")
	// go ExampleNewClientRequest_sourcetable("http://rtn.dot.ny.gov:8082","viam","checkmate")
	// go ExampleNewClientRequest("http://rtn.dot.ny.gov:8082/NJI2", "viam", "checkmate", gpsport, 38400)
	// go dummyFunc()
	return webserver.RunWeb(ctx, myRobot, webOpts, logger)
}

func dummyFunc() {
	for {
		time.Sleep(200 * time.Millisecond)
	}
}
