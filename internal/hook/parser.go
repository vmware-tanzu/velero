package hook

import (
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
)

const (
	nameFromAnnotation = "<from-annotation>"

	annotationKeyPreBackupHookContainer  = "pre.hook.backup.velero.io/container"
	annotationKeyPreBackupHookCommand    = "pre.hook.backup.velero.io/command"
	annotationKeyPreBackupHookOnError    = "pre.hook.backup.velero.io/on-error"
	annotationKeyPreBackupHookTimeout    = "pre.hook.backup.velero.io/timeout"
	annotationKeyPostBackupHookContainer = "post.hook.backup.velero.io/container"
	annotationKeyPostBackupHookCommand   = "post.hook.backup.velero.io/command"
	annotationKeyPostBackupHookOnError   = "post.hook.backup.velero.io/on-error"
	annotationKeyPostBackupHookTimeout   = "post.hook.backup.velero.io/timeout"

	defaultExecTimeout = 30 * time.Second
)

// Parser parses the different hook definitions into the general "ResourceHook" that the Handler can handle
type Parser interface {
	// ListApplicableResourcePreBackupHooks returns the pre backup "ResourceHook" list applicable to the provided resource
	ListApplicableResourcePreBackupHooks(log logrus.FieldLogger, res *unstructured.Unstructured, gr schema.GroupResource, hooks []velerov1.BackupResourceHookSpec) ([]*ResourceHook, error)
	// ListApplicableResourcePostBackupHooks returns the post backup "ResourceHook" list applicable to the provided resource
	ListApplicableResourcePostBackupHooks(log logrus.FieldLogger, res *unstructured.Unstructured, gr schema.GroupResource, hooks []velerov1.BackupResourceHookSpec) ([]*ResourceHook, error)
	// ListApplicableResourcePreRestoreHooks returns the pre restore "ResourceHook" list applicable to the provided resource
	ListApplicableResourcePreRestoreHooks(log logrus.FieldLogger, res *unstructured.Unstructured, gr schema.GroupResource, hooks []velerov1.RestoreResourceHookSpec) ([]*ResourceHook, error)
	// ListApplicableResourcePostRestoreHooks returns the post restore "ResourceHook" list applicable to the provided resource
	ListApplicableResourcePostRestoreHooks(log logrus.FieldLogger, res *unstructured.Unstructured, gr schema.GroupResource, hooks []velerov1.RestoreResourceHookSpec) ([]*ResourceHook, error)
}

var _ Parser = &parser{}

func NewParser(discoveryHelper discovery.Helper) Parser {
	return &parser{
		discoveryHelper: discoveryHelper,
	}
}

type parser struct {
	discoveryHelper discovery.Helper
}

func (p *parser) ListApplicableResourcePreBackupHooks(log logrus.FieldLogger, res *unstructured.Unstructured, gr schema.GroupResource, hookSpecs []velerov1.BackupResourceHookSpec) ([]*ResourceHook, error) {
	return p.listApplicableResourceBackupHooks(log, res, gr, hookSpecs, true)
}

func (p *parser) ListApplicableResourcePostBackupHooks(log logrus.FieldLogger, res *unstructured.Unstructured, gr schema.GroupResource, hookSpecs []velerov1.BackupResourceHookSpec) ([]*ResourceHook, error) {
	return p.listApplicableResourceBackupHooks(log, res, gr, hookSpecs, false)
}

func (p *parser) ListApplicableResourcePreRestoreHooks(log logrus.FieldLogger, res *unstructured.Unstructured, gr schema.GroupResource, hooks []velerov1.RestoreResourceHookSpec) ([]*ResourceHook, error) {
	return nil, errors.New("not implemented")
}

func (p *parser) ListApplicableResourcePostRestoreHooks(log logrus.FieldLogger, res *unstructured.Unstructured, gr schema.GroupResource, hooks []velerov1.RestoreResourceHookSpec) ([]*ResourceHook, error) {
	return nil, errors.New("not implemented")
}

func (p *parser) listApplicableResourceBackupHooks(log logrus.FieldLogger, res *unstructured.Unstructured, gr schema.GroupResource, hookSpecs []velerov1.BackupResourceHookSpec, pre bool) ([]*ResourceHook, error) {
	// only support hooks on pods right now
	if gr != kuberesource.Pods {
		return nil, nil
	}

	annotationKeyContainer := annotationKeyPreBackupHookContainer
	annotationKeyCommand := annotationKeyPreBackupHookCommand
	annotationKeyTimeout := annotationKeyPreBackupHookTimeout
	annotationKeyOnError := annotationKeyPreBackupHookOnError
	hookType := TypePodBackupPreHook

	if !pre {
		annotationKeyContainer = annotationKeyPostBackupHookContainer
		annotationKeyCommand = annotationKeyPostBackupHookCommand
		annotationKeyTimeout = annotationKeyPostBackupHookTimeout
		annotationKeyOnError = annotationKeyPostBackupHookOnError
		hookType = TypePodBackupPostHook
	}

	var hooks []*ResourceHook

	// check the hook defined in the annotation
	annotations := res.GetAnnotations()
	if len(annotations) > 0 && len(annotations[annotationKeyCommand]) > 0 {
		execHook := assembleExecHook(log, annotations[annotationKeyContainer],
			annotations[annotationKeyCommand], annotations[annotationKeyTimeout],
			annotations[annotationKeyOnError])
		if execHook != nil {
			hooks = append(hooks, &ResourceHook{
				Name:            nameFromAnnotation,
				Type:            hookType,
				Spec:            execHook,
				ContinueOnError: execHook.OnError == velerov1.HookErrorModeContinue,
				Resource:        res,
			})
			return hooks, nil
		}
	}

	// check the hooks defined in the backup spec
	for _, hookSpec := range hookSpecs {
		matches, err := matchesResource(res, gr, hookSpec.IncludedNamespaces, hookSpec.ExcludedNamespaces, hookSpec.IncludedResources, hookSpec.ExcludedResources, hookSpec.LabelSelector, p.discoveryHelper)
		if err != nil {
			return nil, err
		}
		if !matches {
			continue
		}

		hks := hookSpec.PreHooks
		if !pre {
			hks = hookSpec.PostHooks
		}
		for i, preHook := range hks {
			hooks = append(hooks, &ResourceHook{
				Name:            hookSpec.Name,
				Type:            hookType,
				Index:           i,
				Spec:            preHook.Exec,
				ContinueOnError: preHook.Exec.OnError == velerov1.HookErrorModeContinue,
				Resource:        res,
			})
		}
	}

	return hooks, nil
}

func assembleExecHook(log logrus.FieldLogger, container, command, timeout, onError string) *velerov1.ExecHook {
	hook := &velerov1.ExecHook{
		Container: container,
		Command:   parseStringToCommand(command),
		Timeout:   metav1.Duration{Duration: defaultExecTimeout},
	}

	if len(timeout) > 0 {
		if duration, err := time.ParseDuration(timeout); err == nil {
			hook.Timeout = metav1.Duration{Duration: duration}
		} else {
			log.Warn(errors.Wrapf(err, "failed to parse the provided timeout %q, use the default value", timeout))
		}
	}

	if velerov1.HookErrorMode(onError) == velerov1.HookErrorModeContinue ||
		velerov1.HookErrorMode(onError) == velerov1.HookErrorModeFail {
		hook.OnError = velerov1.HookErrorMode(onError)
	}

	return hook
}

func matchesResource(res *unstructured.Unstructured, gr schema.GroupResource, includedNamespaces, excludedNamespaces, includedResources, excludedResources []string,
	labelSelector *metav1.LabelSelector, discoveryHelper discovery.Helper) (bool, error) {
	namespaceIncludesExcludes := collections.NewIncludesExcludes().Includes(includedNamespaces...).Excludes(excludedNamespaces...)
	if !namespaceIncludesExcludes.ShouldInclude(res.GetNamespace()) {
		return false, nil
	}

	resourceIncludesExcludes := collections.GetResourceIncludesExcludes(discoveryHelper, includedResources, excludedResources)
	if !resourceIncludesExcludes.ShouldInclude(gr.String()) {
		return false, nil
	}

	if labelSelector == nil {
		return true, nil
	}
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return false, errors.Wrap(err, "failed to convert to label selector")
	}
	return selector.Matches(labels.Set(res.GetLabels())), nil
}
