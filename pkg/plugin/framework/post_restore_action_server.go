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

package framework

import (
	"encoding/json"

	"github.com/pkg/errors"
	"golang.org/x/net/context"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// PostRestoreActionGRPCServer implements the proto-generated PostRestoreAction interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type PostRestoreActionGRPCServer struct {
	mux *serverMux
}

func (s *PostRestoreActionGRPCServer) getImpl(name string) (velero.PostRestoreAction, error) {
	impl, err := s.mux.getHandler(name)
	if err != nil {
		return nil, err
	}

	action, ok := impl.(velero.PostRestoreAction)
	if !ok {
		return nil, errors.Errorf("%T is not a pre-restore action", impl)
	}

	return action, nil
}

// Execute the call to the plugin
func (s *PostRestoreActionGRPCServer) Execute(ctx context.Context, req *proto.PostRestoreActionExecuteRequest) (_ *proto.Empty, err error) {
	defer func() {
		if recoveredErr := handlePanic(recover()); recoveredErr != nil {
			err = recoveredErr
		}
	}()

	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, newGRPCError(err)
	}

	var restore api.Restore

	if err := json.Unmarshal(req.Restore, &restore); err != nil {
		return nil, newGRPCError(errors.WithStack(err))
	}

	err = impl.Execute(&restore)
	if err != nil {
		return nil, newGRPCError(err)
	}

	return &proto.Empty{}, nil
}
