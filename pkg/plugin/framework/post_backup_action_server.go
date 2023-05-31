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

// PostBackupActionGRPCServer implements the proto-generated PostBackupActionServer interface, and accepts
// gRPC calls and forwards them to an implementation of the pluggable interface.
type PostBackupActionGRPCServer struct {
	mux *common.ServerMux
}

func (s *PostBackupActionGRPCServer) getImpl(name string) (velero.PostBackupAction, error) {
	impl, err := s.mux.GetHandler(name)
	if err != nil {
		return nil, err
	}

	itemAction, ok := impl.(velero.PostBackupAction)
	if !ok {
		return nil, errors.Errorf("%T is not a post backup item action", impl)
	}

	return itemAction, nil
}

func (s *PostBackupActionGRPCServer) Execute(ctx context.Context, req *proto.PostBackupActionExecuteRequest) (_ *proto.Empty, err error) {
	defer func() {
		if recoveredErr := common.HandlePanic(recover()); recoveredErr != nil {
			err = recoveredErr
		}
	}()
	impl, err := s.getImpl(req.Plugin)
	if err != nil {
		return nil, common.NewGRPCError(err)
	}
	var backup api.Backup
	if err = json.Unmarshal(req.Backup, &backup); err != nil {
		return nil, common.NewGRPCError(errors.WithStack(err))
	}
	if err := impl.Execute(&backup); err != nil {
		return nil, common.NewGRPCError(err)
	}
	return &proto.Empty{}, nil
}
