package rdk

import (
	"context"
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/edaniels/gostream"
	"go.viam.com/utils/rpc"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/camera"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot/client"
	robotimpl "go.viam.com/rdk/robot/impl"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/utils"
)

func BenchmarkEncodeDecodeRead(b *testing.B) {
	logger := golog.NewDebugLogger("foo")
	roboConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "fake",
				"name": "cam1",
				"type": "camera",
				"attributes": {
 					 "width": 640,
					"height": 480
				}
			}
		]
	}`)
	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(roboConfig), logger)
	test.That(b, err, test.ShouldBeNil)

	ctx := context.Background()
	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(b, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(b)
	err = r.StartWeb(ctx, options)
	test.That(b, err, test.ShouldBeNil)

	roboClient, err := client.New(ctx, addr, logger)
	test.That(b, err, test.ShouldBeNil)

	cam1, err := camera.FromRobot(roboClient, "cam1")
	test.That(b, err, test.ShouldBeNil)

	type intCamera interface {
		Read(ctx context.Context) (image.Image, func(), error)
	}
	cam1Int := utils.UnwrapProxy(cam1).(intCamera)
	ctx = gostream.WithMIMETypeHint(context.Background(), utils.MimeTypeJPEG)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := cam1Int.Read(ctx)
		test.That(b, err, test.ShouldBeNil)
	}

	test.That(b, roboClient.Close(ctx), test.ShouldBeNil)
	test.That(b, r.Close(ctx), test.ShouldBeNil)
}

func BenchmarkEncodeDecodeNext(b *testing.B) {
	logger := golog.NewDebugLogger("foo")
	roboConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "fake",
				"name": "cam1",
				"type": "camera"
			}
		]
	}`)
	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(roboConfig), logger)
	test.That(b, err, test.ShouldBeNil)

	ctx := context.Background()
	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(b, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(b)
	err = r.StartWeb(ctx, options)
	test.That(b, err, test.ShouldBeNil)

	roboClient, err := client.New(ctx, addr, logger)
	test.That(b, err, test.ShouldBeNil)

	cam1, err := camera.FromRobot(roboClient, "cam1")
	test.That(b, err, test.ShouldBeNil)
	ctx = gostream.WithMIMETypeHint(ctx, utils.MimeTypeJPEG)
	camStream, _ := cam1.Stream(ctx)

	ctx = gostream.WithMIMETypeHint(context.Background(), utils.MimeTypeJPEG)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := camStream.Next(ctx)
		test.That(b, err, test.ShouldBeNil)
	}

	test.That(b, roboClient.Close(ctx), test.ShouldBeNil)
	test.That(b, r.Close(ctx), test.ShouldBeNil)
}

func BenchmarkRSEncodeDecodeNext(b *testing.B) {
	logger := golog.NewDebugLogger("foo")
	roboClient, err := client.New(
		context.Background(),
		"monvie-main.wszwqu7wcv.viam.cloud",
		logger,
		client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
			Type:    utils.CredentialsTypeRobotLocationSecret,
			Payload: "adpcqgrzdhwa1q42bdeu4bsn6ee7vemx9fjuerl21ux75ydv",
		})),
	)
	if err != nil {
		logger.Fatal(err)
	}
	ctx := context.Background()
	defer roboClient.Close(ctx)
	logger.Info("Resources:")
	logger.Info(roboClient.ResourceNames())

	cam1, err := camera.FromRobot(roboClient, "test-ResFail-main:cam1")

	ctx = gostream.WithMIMETypeHint(ctx, utils.MimeTypeJPEG)
	camStream, _ := cam1.Stream(ctx)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := camStream.Next(ctx)
		if err != nil {
			panic("WESH")
		}

	}
}

// func TestCheckSize(t *testing.T) {
//	logger := golog.NewDebugLogger("foomalad")
//	roboClient, err := client.New(
//		context.Background(),
//		"mac-server-main.wszwqu7wcv.viam.cloud",
//		logger,
//		client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
//			Type:    utils.CredentialsTypeRobotLocationSecret,
//			Payload: "adpcqgrzdhwa1q42bdeu4bsn6ee7vemx9fjuerl21ux75ydv",
//		})),
//	)
//	if err != nil {
//		logger.Fatal(err)
//	}
//	defer roboClient.Close(context.Background())
//	logger.Info("Resources:")
//	logger.Info(roboClient.ResourceNames())
//
//	cam1, err := camera.FromRobot(roboClient, "webcam")
//	test.That(t, err, test.ShouldBeNil)
//	ctx := gostream.WithMIMETypeHint(context.Background(), utils.MimeTypeJPEG)
//	camStream, _ := cam1.Stream(ctx)
//	img, _, _ := camStream.Next(ctx)
//	rimage.WriteImageToFile("./image3.jpeg", img)
//
//}
