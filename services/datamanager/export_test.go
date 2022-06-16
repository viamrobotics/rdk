// export_test.go adds functionality to dataManagerService that we only use during testing.
package datamanager

import "context"

func (svc *dataManagerService) SetUploadFn(fn func(ctx context.Context, path string) error) {
	svc.uploadFn = fn
}
