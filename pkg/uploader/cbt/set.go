/*
Copyright The Velero Contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cbt

import (
	"context"

	"github.com/vmware-tanzu/velero/pkg/cbtservice"
)

// SetBitmapOrFull translates the allocated/changed blocks from CBT service to the given bitmap or set the bitmap to full when error happens
func SetBitmapOrFull(ctx context.Context, service cbtservice.Service, bitmap Bitmap) error {
	var err error
	if bitmap.ChangeID() == "" {
		err = setFromAllocatedBlocks(ctx, service, bitmap)
	} else {
		err = setFromChangedBlocks(ctx, service, bitmap)
	}

	if err != nil {
		bitmap.SetFull()
	}

	return err
}

// TODO implement in following PRs
func setFromAllocatedBlocks(_ context.Context, _ cbtservice.Service, _ Bitmap) error {
	return nil
}

// TODO implement in following PRs
func setFromChangedBlocks(_ context.Context, _ cbtservice.Service, _ Bitmap) error {
	return nil
}
