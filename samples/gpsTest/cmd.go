// Package main is the work-in-progress robotic boat from Viam.
package main

import (
	"bufio"
	"io"

	"context"
	"flag"
	"fmt"

	// "io"
	"io/ioutil"
	"net/http"

	// "math"
	// "strconv"
	// "strings"
	// "time"

	// "github.com/go-errors/errors"

	// "go.uber.org/multierr"

	"go.viam.com/utils"

	"go.viam.com/core/config"
	// pb "go.viam.com/core/proto/api/v1"
	// "go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"
	// "go.viam.com/core/services/navigation"
	"go.viam.com/core/services/web"
	// "go.viam.com/core/sensor/gps"
	// "go.viam.com/core/spatialmath"
	// coreutils "go.viam.com/core/utils"
	webserver "go.viam.com/core/web/server"

	// "github.com/de-bkg/gognss/pkg/ntrip"
	"github.com/edaniels/golog"
	"github.com/go-gnss/ntrip"
	slib "github.com/jacobsa/go-serial/serial"
)

var logger = golog.NewDevelopmentLogger("gpsTest")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func connectClient(req *http.Request, i int) (resp *http.Response, err error, j int) {
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("error making NTRIP request: %s\n", err)

		return nil, err, i
	} else {
		j = 1
	}
	return resp, nil, j
}
func ExampleNewClientRequest_sourcetable(urlStr string) (err error) {
	// var resp http.DefaultClient
	// req, _ := ntrip.NewClientRequest("http://rtn.dot.ny.gov:8080")
	req, _ := ntrip.NewClientRequest(urlStr)
	// resp, err := http.DefaultClient.Do(req)

	req.SetBasicAuth("viam", "checkmate")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("error making NTRIP request: %s\n", err)
		return err
	}
	// resp, err, i := connectClient(req, 0)
	defer resp.Body.Close()
	// for i != 1 {
	// 	fmt.Println("Trying Again")
	// 	resp, err, i = connectClient(req, 0)
	// }

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("received non-200 response code: %d", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error reading from response body")
	}

	sourcetable, _ := ntrip.ParseSourcetable(string(data))
	fmt.Printf("caster has %d mountpoints available\n", len(sourcetable.Mounts))
	for i := 0; i < len(sourcetable.Mounts); i++ {
		fmt.Println(sourcetable.Mounts[i])
	}

	return nil
}

func ExampleNewClientRequest() (err error) {
	// readMount, writeMount := io.Pipe()
	req, _ := ntrip.NewClientRequest("http://rtn.dot.ny.gov:8082/NJI2")
	// req, _ := ntrip.NewClientRequest("http://rtn.dot.ny.gov:8080/near_msm")
	// c := http.Client{Timeout: time.Duration(1) * time.Second}
	// resp, _ := c.Get("https://www.google.com")//ntrip.NewClientRequest("http://rtn.dot.ny.gov:8080/near_msm")
	req.SetBasicAuth("viam", "checkmate")
	resp, err := http.DefaultClient.Do(req)
	// resp, err, i := connectClient(req, 0)
	// defer resp.Body.Close()
	// for i != 1 {
	// 	fmt.Println("Trying Again")
	// 	resp, err, i = connectClient(req, 0)
	// }
	if err != nil {
		fmt.Printf("error making NTRIP request: %s\n", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("received non-200 response code: %d", resp.StatusCode)
		return err
	}
	fmt.Printf("Status Code : %d \n", resp.StatusCode)
	// for{

	fmt.Println("Response Body 2: ", resp.Body)

	options := slib.OpenOptions{
		BaudRate:        38400,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}
	options.PortName = "/dev/serial/by-id/usb-u-blox_AG_-_www.u-blox.com_u-blox_GNSS_receiver-if00" // "/dev/serial0" //
	port, err := slib.Open(options)
	w := bufio.NewWriter(port)

	r := io.TeeReader(resp.Body, w)
	io.ReadAll(r)

	if err != nil {
		fmt.Printf("Error with RTCM stream: %s\n", err)
		return err
	}

	// fmt.Println("we might have written")
	return nil
	// Read from resp.Body until EOF
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

	// go ExampleNewClientRequest_sourcetable("http://rtn.dot.ny.gov:8080")
	// go ExampleNewClientRequest_sourcetable("http://rtn.dot.ny.gov:8082")
	go ExampleNewClientRequest()
	return webserver.RunWeb(ctx, myRobot, webOpts, logger)
}
