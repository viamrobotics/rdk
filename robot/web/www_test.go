package web

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robots/fake"
)

func TestWeb(t *testing.T) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	// setup arm
	r := robot.NewBlankRobot()
	defer r.Close(cancelCtx)

	arm := &fake.Arm{}
	r.AddArm(arm, api.Component{Name: "arm1"})

	// set up server
	mux := http.NewServeMux()
	webCloser, err := InstallWeb(cancelCtx, mux, r, Options{})
	if err != nil {
		t.Fatal(err)
	}

	const port = 51211
	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
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
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}
	}()

	client := Client{fmt.Sprintf("http://localhost:%d", port)}

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
