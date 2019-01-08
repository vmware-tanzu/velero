package plugin

import (
	"io"
	"time"

	"github.com/hashicorp/go-plugin"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	proto "github.com/heptio/velero/pkg/plugin/generated"
)

const byteChunkSize = 16384

// ObjectStorePlugin is an implementation of go-plugin's Plugin
// interface with support for gRPC for the cloudprovider/ObjectStore
// interface.
type ObjectStorePlugin struct {
	plugin.NetRPCUnsupportedPlugin
	*pluginBase
}

// NewObjectStorePlugin construct an ObjectStorePlugin.
func NewObjectStorePlugin(options ...pluginOption) *ObjectStorePlugin {
	return &ObjectStorePlugin{
		pluginBase: newPluginBase(options...),
	}
}

// GRPCClient returns an ObjectStore gRPC client.
func (p *ObjectStorePlugin) GRPCClient(c *grpc.ClientConn) (interface{}, error) {
	return newClientDispenser(p.clientLogger, c, newObjectStoreGRPCClient), nil

}

// ObjectStoreGRPCClient implements the cloudprovider.ObjectStore interface and uses a
// gRPC client to make calls to the plugin server.
type ObjectStoreGRPCClient struct {
	*clientBase
	grpcClient proto.ObjectStoreClient
}

func newObjectStoreGRPCClient(base *clientBase, clientConn *grpc.ClientConn) interface{} {
	return &ObjectStoreGRPCClient{
		clientBase: base,
		grpcClient: proto.NewObjectStoreClient(clientConn),
	}
}

// Init prepares the ObjectStore for usage using the provided map of
// configuration key-value pairs. It returns an error if the ObjectStore
// cannot be initialized from the provided config.
func (c *ObjectStoreGRPCClient) Init(config map[string]string) error {
	_, err := c.grpcClient.Init(context.Background(), &proto.InitRequest{Plugin: c.plugin, Config: config})

	return err
}

// PutObject creates a new object using the data in body within the specified
// object storage bucket with the given key.
func (c *ObjectStoreGRPCClient) PutObject(bucket, key string, body io.Reader) error {
	stream, err := c.grpcClient.PutObject(context.Background())
	if err != nil {
		return err
	}

	// read from the provider io.Reader into chunks, and send each one over
	// the gRPC stream
	chunk := make([]byte, byteChunkSize)
	for {
		n, err := body.Read(chunk)
		if err == io.EOF {
			_, resErr := stream.CloseAndRecv()
			return resErr
		}
		if err != nil {
			stream.CloseSend()
			return err
		}

		if err := stream.Send(&proto.PutObjectRequest{Plugin: c.plugin, Bucket: bucket, Key: key, Body: chunk[0:n]}); err != nil {
			return err
		}
	}
}

// GetObject retrieves the object with the given key from the specified
// bucket in object storage.
func (c *ObjectStoreGRPCClient) GetObject(bucket, key string) (io.ReadCloser, error) {
	stream, err := c.grpcClient.GetObject(context.Background(), &proto.GetObjectRequest{Plugin: c.plugin, Bucket: bucket, Key: key})
	if err != nil {
		return nil, err
	}

	receive := func() ([]byte, error) {
		data, err := stream.Recv()
		if err != nil {
			return nil, err
		}

		return data.Data, nil
	}

	close := func() error {
		return stream.CloseSend()
	}

	return &StreamReadCloser{receive: receive, close: close}, nil
}

// ListCommonPrefixes gets a list of all object key prefixes that come
// after the provided prefix and before the provided delimiter (this is
// often used to simulate a directory hierarchy in object storage).
func (c *ObjectStoreGRPCClient) ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error) {
	req := &proto.ListCommonPrefixesRequest{
		Plugin:    c.plugin,
		Bucket:    bucket,
		Prefix:    prefix,
		Delimiter: delimiter,
	}

	res, err := c.grpcClient.ListCommonPrefixes(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return res.Prefixes, nil
}

// ListObjects gets a list of all objects in bucket that have the same prefix.
func (c *ObjectStoreGRPCClient) ListObjects(bucket, prefix string) ([]string, error) {
	res, err := c.grpcClient.ListObjects(context.Background(), &proto.ListObjectsRequest{Plugin: c.plugin, Bucket: bucket, Prefix: prefix})
	if err != nil {
		return nil, err
	}

	return res.Keys, nil
}

// DeleteObject removes object with the specified key from the given
// bucket.
func (c *ObjectStoreGRPCClient) DeleteObject(bucket, key string) error {
	_, err := c.grpcClient.DeleteObject(context.Background(), &proto.DeleteObjectRequest{Plugin: c.plugin, Bucket: bucket, Key: key})

	return err
}

// CreateSignedURL creates a pre-signed URL for the given bucket and key that expires after ttl.
func (c *ObjectStoreGRPCClient) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	res, err := c.grpcClient.CreateSignedURL(context.Background(), &proto.CreateSignedURLRequest{
		Plugin: c.plugin,
		Bucket: bucket,
		Key:    key,
		Ttl:    int64(ttl),
	})
	if err != nil {
		return "", nil
	}

	return res.Url, nil
}
