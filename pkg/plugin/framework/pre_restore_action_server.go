/*
Copyright the Velero contributors.

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
	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// PreRestoreActionGRPCServer implements the proto-generated PreRestoreActionServer interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type PreRestoreActionGRPCServer struct {
	mux *common.ServerMux
}

func (s *PreRestoreActionGRPCServer) getImpl(name string) (velero.PreRestoreAction, error) {
	impl, err := s.mux.GetHandler(name)
	if err != nil {
		return nil, err
	}

	itemAction, ok := impl.(velero.PreRestoreAction)
	if !ok {
		return nil, errors.Errorf("%T is not a pre restore item action", impl)
	}

	return itemAction, nil
}

func (s *PreRestoreActionGRPCServer) Execute(ctx context.Context, req *proto.PreRestoreActionExecuteRequest) (_ *proto.Empty, err error) {
	defer func() {
		if recoveredErr := common.HandlePanic(recover()); recoveredErr != nil {
			err = recoveredErr
		}
	}()
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}
	var restore api.Restore
	if err = json.Unmarshal(req.Restore, &restore); err != nil {
		return nil, common.NewGRPCError(errors.WithStack(err))
	}
	if err := impl.Execute(&restore); err != nil {
		return nil, common.NewGRPCError(err)
	}
	return &proto.Empty{}, nil
}
