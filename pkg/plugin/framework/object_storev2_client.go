/*
Copyright 2017, 2019 the Velero contributors.

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
	"io"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	proto "github.com/vmware-tanzu/velero/pkg/plugin/generated"
)

// NewObjectStoreV2Plugin construct an ObjectStoreV2Plugin.
func NewObjectStoreV2Plugin(options ...PluginOption) *ObjectStoreV2Plugin {
	return &ObjectStoreV2Plugin{
		pluginBase: newPluginBase(options...),
	}
}

// ObjectStoreV2GRPCClient implements the ObjectStore interface and uses a
// gRPC client to make calls to the plugin server.
type ObjectStoreV2GRPCClient struct {
	*clientBase
	grpcClient proto.ObjectStoreV2Client
}

func newObjectStoreV2GRPCClient(base *clientBase, clientConn *grpc.ClientConn) interface{} {
	return &ObjectStoreV2GRPCClient{
		clientBase: base,
		grpcClient: proto.NewObjectStoreV2Client(clientConn),
	}
}

// Init prepares the ObjectStore for usage using the provided map of
// configuration key-value pairs. It returns an error if the ObjectStore
// cannot be initialized from the provided config.
func (c *ObjectStoreV2GRPCClient) Init(config map[string]string) error {
	req := &proto.ObjectStoreInitRequest{
		Plugin: c.plugin,
		Config: config,
	}

	if _, err := c.grpcClient.Init(context.Background(), req); err != nil {
		return fromGRPCError(err)
	}

	return nil
}

// PutObject creates a new object using the data in body within the specified
// object storage bucket with the given key.
func (c *ObjectStoreV2GRPCClient) PutObject(bucket, key string, body io.Reader) error {
	return c.PutObjectContext(context.Background(), bucket, key, body)
}

// PutObjectContext creates a new object using the data in body within the specified
// object storage bucket with the given key.
func (c *ObjectStoreV2GRPCClient) PutObjectContext(ctx context.Context, bucket, key string, body io.Reader) error {

	stream, err := c.grpcClient.PutObject(ctx)
	if err != nil {
		return fromGRPCError(err)
	}

	// read from the provider io.Reader into chunks, and send each one over
	// the gRPC stream
	chunk := make([]byte, byteChunkSize)
	for {
		n, err := body.Read(chunk)
		if err == io.EOF {
			if _, resErr := stream.CloseAndRecv(); resErr != nil {
				return fromGRPCError(resErr)
			}
			return nil
		}
		if err != nil {
			stream.CloseSend()
			return errors.WithStack(err)
		}

		if err := stream.Send(&proto.PutObjectRequest{Plugin: c.plugin, Bucket: bucket, Key: key, Body: chunk[0:n]}); err != nil {
			return fromGRPCError(err)
		}
	}
}

// ObjectExists checks if there is an object with the given key in the object storage bucket.
func (c *ObjectStoreV2GRPCClient) ObjectExists(bucket, key string) (bool, error) {
	return c.ObjectExistsContext(context.Background(), bucket, key)
}

func (c *ObjectStoreV2GRPCClient) ObjectExistsContext(ctx context.Context, bucket, key string) (bool, error) {
	req := &proto.ObjectExistsRequest{
		Plugin: c.plugin,
		Bucket: bucket,
		Key:    key,
	}

	res, err := c.grpcClient.ObjectExists(ctx, req)
	if err != nil {
		return false, err
	}

	return res.Exists, nil
}

// GetObject retrieves the object with the given key from the specified
// bucket in object storage.
func (c *ObjectStoreV2GRPCClient) GetObject(bucket, key string) (io.ReadCloser, error) {
	return c.GetObjectContext(context.Background(), bucket, key)
}

func (c *ObjectStoreV2GRPCClient) GetObjectContext(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	req := &proto.GetObjectRequest{
		Plugin: c.plugin,
		Bucket: bucket,
		Key:    key,
	}

	stream, err := c.grpcClient.GetObject(ctx, req)
	if err != nil {
		return nil, fromGRPCError(err)
	}

	receive := func() ([]byte, error) {
		data, err := stream.Recv()
		if err == io.EOF {
			// we need to return io.EOF errors unwrapped so that
			// calling code sees them as io.EOF and knows to stop
			// reading.
			return nil, err
		}
		if err != nil {
			return nil, fromGRPCError(err)
		}

		return data.Data, nil
	}

	close := func() error {
		if err := stream.CloseSend(); err != nil {
			return fromGRPCError(err)
		}
		return nil
	}

	return &StreamReadCloser{receive: receive, close: close}, nil
}

// ListCommonPrefixes gets a list of all object key prefixes that come
// after the provided prefix and before the provided delimiter (this is
// often used to simulate a directory hierarchy in object storage).
func (c *ObjectStoreV2GRPCClient) ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error) {
	return c.ListCommonPrefixesContext(context.Background(), bucket, prefix, delimiter)
}

func (c *ObjectStoreV2GRPCClient) ListCommonPrefixesContext(ctx context.Context, bucket, prefix, delimiter string) ([]string, error) {
	req := &proto.ListCommonPrefixesRequest{
		Plugin:    c.plugin,
		Bucket:    bucket,
		Prefix:    prefix,
		Delimiter: delimiter,
	}

	res, err := c.grpcClient.ListCommonPrefixes(ctx, req)
	if err != nil {
		return nil, fromGRPCError(err)
	}

	return res.Prefixes, nil
}

// ListObjects gets a list of all objects in bucket that have the same prefix.
func (c *ObjectStoreV2GRPCClient) ListObjects(bucket, prefix string) ([]string, error) {
	return c.ListObjectsContext(context.Background(), bucket, prefix)
}

func (c *ObjectStoreV2GRPCClient) ListObjectsContext(ctx context.Context, bucket, prefix string) ([]string, error) {
	req := &proto.ListObjectsRequest{
		Plugin: c.plugin,
		Bucket: bucket,
		Prefix: prefix,
	}

	res, err := c.grpcClient.ListObjects(ctx, req)
	if err != nil {
		return nil, fromGRPCError(err)
	}

	return res.Keys, nil
}

// DeleteObject removes object with the specified key from the given
// bucket.
func (c *ObjectStoreV2GRPCClient) DeleteObject(bucket, key string) error {
	return c.DeleteObjectContext(context.Background(), bucket, key)
}

func (c *ObjectStoreV2GRPCClient) DeleteObjectContext(ctx context.Context, bucket, key string) error {
	req := &proto.DeleteObjectRequest{
		Plugin: c.plugin,
		Bucket: bucket,
		Key:    key,
	}

	if _, err := c.grpcClient.DeleteObject(ctx, req); err != nil {
		return fromGRPCError(err)
	}

	return nil
}

// CreateSignedURL creates a pre-signed URL for the given bucket and key that expires after ttl.
func (c *ObjectStoreV2GRPCClient) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	return c.CreateSignedURLContext(context.Background(), bucket, key, ttl)
}

func (c *ObjectStoreV2GRPCClient) CreateSignedURLContext(
	ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	req := &proto.CreateSignedURLRequest{
		Plugin: c.plugin,
		Bucket: bucket,
		Key:    key,
		Ttl:    int64(ttl),
	}

	res, err := c.grpcClient.CreateSignedURL(ctx, req)
	if err != nil {
		return "", fromGRPCError(err)
	}

	return res.Url, nil
}
