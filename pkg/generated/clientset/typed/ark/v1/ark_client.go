package v1

import (
	v1 "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/scheme"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	rest "k8s.io/client-go/rest"
)

type ArkV1Interface interface {
	RESTClient() rest.Interface
	BackupsGetter
	ConfigsGetter
	DownloadRequestsGetter
	RestoresGetter
	SchedulesGetter
}

// ArkV1Client is used to interact with features provided by the ark.heptio.com group.
type ArkV1Client struct {
	restClient rest.Interface
}

func (c *ArkV1Client) Backups(namespace string) BackupInterface {
	return newBackups(c, namespace)
}

func (c *ArkV1Client) Configs(namespace string) ConfigInterface {
	return newConfigs(c, namespace)
}

func (c *ArkV1Client) DownloadRequests(namespace string) DownloadRequestInterface {
	return newDownloadRequests(c, namespace)
}

func (c *ArkV1Client) Restores(namespace string) RestoreInterface {
	return newRestores(c, namespace)
}

func (c *ArkV1Client) Schedules(namespace string) ScheduleInterface {
	return newSchedules(c, namespace)
}

// NewForConfig creates a new ArkV1Client for the given config.
func NewForConfig(c *rest.Config) (*ArkV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ArkV1Client{client}, nil
}

// NewForConfigOrDie creates a new ArkV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *ArkV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new ArkV1Client for the given RESTClient.
func New(c rest.Interface) *ArkV1Client {
	return &ArkV1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *ArkV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
