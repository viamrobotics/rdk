package web

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robots/fake"
	"goji.io"
)

func checkStatus(t *testing.T, r api.Robot, client *Client) {
	statusLocal, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}

	statusRemote, err := client.Status()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, statusLocal, statusRemote)
}

func TestWeb(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	// setup arm
	r := robot.NewBlankRobot(logger)
	defer r.Close(cancelCtx)

	arm := fake.NewArm()
	r.AddArm(arm, api.Component{Name: "arm1"})

	// set up server
	mux := goji.NewMux()
	webCloser, err := InstallWeb(cancelCtx, mux, r, Options{}, logger)
	if err != nil {
		t.Fatal(err)
	}

	const port = 51211
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatal(err)
	}
	httpServer := &http.Server{
		Addr:           listener.Addr().String(),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        mux,
	}

	defer func() {
		cancelFunc()
		webCloser()
		if err := httpServer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	go func() {
		if err := httpServer.Serve(listener); err != http.ErrServerClosed {
			panic(err)
		}
	}()

	client := Client{fmt.Sprintf("http://localhost:%d", port)}
	checkStatus(t, r, &client)

	t.Run("Arm MoveToPosition", func(t *testing.T) {
		p := api.ArmPosition{1, 2, 3, 4, 5, 6}
		err = client.ArmByName("arm1").MoveToPosition(p)
		if err != nil {
			t.Fatal(err)
		}

		checkStatus(t, r, &client)
		p, err = arm.CurrentPosition()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 2.0, p.Y)
	})

	t.Run("Arm MoveToPosition", func(t *testing.T) {
		p, err := arm.CurrentJointPositions()
		if err != nil {
			t.Fatal(err)
		}
		p.Degrees[2] += 3.0

		err = client.ArmByName("arm1").MoveToJointPositions(p)
		if err != nil {
			t.Fatal(err)
		}

		checkStatus(t, r, &client)
		p, err = arm.CurrentJointPositions()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 3.0, p.Degrees[2])
	})

}
