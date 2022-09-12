package videosource

import (
	"bytes"
	"context"
	"image"
	"io"
	"net/http"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

func decodeImage(imgData []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewBuffer(imgData))
	return img, err
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
	return io.ReadAll(body)
}

func checkLazyFromData(ctx context.Context, data []byte) (image.Image, bool, error) {
	requestedMime := gostream.MIMETypeHint(ctx, "")
	if actualMime, isLazy := utils.CheckLazyMIMEType(requestedMime); isLazy {
		usedMimeType := http.DetectContentType(data)
		if actualMime != "" && actualMime != usedMimeType {
			return nil, false,
				errors.Errorf("mime type requested (%q) for lazy decode not returned (got %q)",
					actualMime, usedMimeType)
		}

		return rimage.NewLazyEncodedImage(data, usedMimeType, -1, -1), true, nil
	}
	return nil, false, nil
}

func readColorURL(ctx context.Context, client http.Client, url string) (image.Image, error) {
	colorData, err := readyBytesFromURL(ctx, client, url)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't ready color url")
	}

	if lazyImg, ok, err := checkLazyFromData(ctx, colorData); err != nil {
		return nil, err
	} else if ok {
		return lazyImg, nil
	}

	img, err := decodeImage(colorData)
	if err != nil {
		return nil, err
	}
	return rimage.ConvertImage(img), nil
}

func readDepthURL(ctx context.Context, client http.Client, url string, immediate bool) (image.Image, error) {
	depthData, err := readyBytesFromURL(ctx, client, url)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't ready depth url")
	}

	if !immediate {
		if lazyImg, ok, err := checkLazyFromData(ctx, depthData); err != nil {
			return nil, err
		} else if ok {
			return lazyImg, nil
		}
	}

	img, err := decodeImage(depthData)
	if err != nil {
		return nil, err
	}
	return rimage.ConvertImageToDepthMap(img)
}
