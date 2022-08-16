package imagesource

import (
	"bufio"
	"bytes"
	"context"
	"image"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/rimage"
)

func decodeImage(imgData []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewBuffer(imgData))
	return img, err
}

func decodeDepth(depthData []byte) (*rimage.DepthMap, error) {
	return rimage.ReadDepthMap(bufio.NewReader(bytes.NewReader(depthData)))
}

func prepReadFromURL(ctx context.Context, client http.Client, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func readyBytesFromURL(ctx context.Context, client http.Client, url string) ([]byte, error) {
	body, err := prepReadFromURL(ctx, client, url)
	if err != nil {
		return nil, err
	}

	defer func() {
		viamutils.UncheckedError(body.Close())
	}()
	return ioutil.ReadAll(body)
}

func readColorURL(ctx context.Context, client http.Client, url string) (*rimage.Image, error) {
	colorData, err := readyBytesFromURL(ctx, client, url)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't ready color url")
	}
	img, err := decodeImage(colorData)
	if err != nil {
		return nil, err
	}
	return rimage.ConvertImage(img), nil
}

func readDepthURL(ctx context.Context, client http.Client, url string) (*rimage.DepthMap, error) {
	depthData, err := readyBytesFromURL(ctx, client, url)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't ready depth url")
	}
	// do this first and make sure ok before creating any mats
	// dat.gz files are deprecated, will be able to remove these soon.
	if strings.HasSuffix(url, "dat") || strings.HasSuffix(url, "dat.gz") {
		return decodeDepth(depthData)
	}
	img, err := decodeImage(depthData)
	if err != nil {
		return nil, err
	}
	return rimage.ConvertImageToDepthMap(img)
}
