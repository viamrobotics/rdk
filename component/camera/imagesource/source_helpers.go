package imagesource

import (
	"bufio"
	"bytes"
	"context"
	"image"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"go.viam.com/rdk/rimage"
	viamutils "go.viam.com/utils"
)

func decodeColor(colorData []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewBuffer(colorData))
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
		return nil, nil, errors.Wrap(err, "couldn't ready color url")
	}
	return decodeColor(colorData)
}

func readDepthURL(ctx context.Context, client http.Client, url string) (*rimage.DepthMap, error) {
	depthData, err := readyBytesFromURL(ctx, client, url)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't ready depth url")
	}
	// do this first and make sure ok before creating any mats
	return decodeDepth(depthData)
}
