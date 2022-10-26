// test io/pipe error
package main

import (
	"context"
	"os/exec"

	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/web/server"

	// registers all components.
	_ "go.viam.com/rdk/components/register"
	// registers all services.
	_ "go.viam.com/rdk/services/register"
)

func startLocalServer(logger golog.Logger) {
	args := []string{"", "-config", "cmd/error_message/local.json"}
	err := server.RunServer(context.Background(), args, logger)
	if err != nil {
		logger.Error("failed to start local server", err)
	}
}

func startRemoteServerInNewProcess(logger golog.Logger) {
	cmd := exec.Command("go", "run", "web/cmd/server/main.go", "-config", "cmd/error_message/remote.json")
	// cmd := exec.Command("./main", "-config", "cmd/error_message/remote.json")
	if err := cmd.Run(); err != nil {
		logger.Error("failed to start remote server", err)
	}
}

func killRemoteServer(delay time.Duration, logger golog.Logger) {
	time.Sleep(delay)

	cmd := exec.Command("lsof", "-ti", ":8081")
	out, err := cmd.Output()
	if err != nil {
		logger.Error(err)
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		logger.Error(err)
		return
	}

	if err = syscall.Kill(pid, syscall.SIGTERM); err != nil {
		logger.Error(err)
	}
}

func main() {
	ctx := context.Background()
	logger := golog.NewDevelopmentLogger("simulation")

	// start remote server (in a separate process)
	go startRemoteServerInNewProcess(logger)
	defer killRemoteServer(0, logger)

	time.Sleep(3 * time.Second)
	go startLocalServer(logger)

	// start client
	time.Sleep(3 * time.Second)
	robot, err := client.New(ctx, "localhost:8080", logger)
	if err != nil {
		logger.Error(err)
		return
	}
	defer robot.Close(ctx)

	a, err := arm.FromRobot(robot, "remote:arm")
	if err != nil {
		logger.Error(err)
		return
	}
	// Make a call to arm client
	if e, _ := a.EndPosition(ctx, map[string]interface{}{}); err == nil {
		logger.Info(e)
	}

	// Kill remote in a few seconds
	go killRemoteServer(2*time.Second, logger)

	// Make a second call to arm client
	time.Sleep(5 * time.Second)
	e, err := a.EndPosition(ctx, map[string]interface{}{})
	if err != nil {
		logger.Error(err)
		// return
	} else {
		logger.Info(e)
	}
}
