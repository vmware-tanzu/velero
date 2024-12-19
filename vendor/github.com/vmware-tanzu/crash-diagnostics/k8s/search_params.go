package k8s

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type SearchParams struct {
	Groups     []string
	Categories []string
	Kinds      []string
	Namespaces []string
	Versions   []string
	Names      []string
	Labels     []string
	Containers []string
}

func (sp SearchParams) ContainsGroup(group string) bool {
	return contains(sp.Groups, group)
}

func (sp SearchParams) ContainsVersion(version string) bool {
	return contains(sp.Versions, version)
}

func (sp SearchParams) ContainsKind(kind string) bool {
	return contains(sp.Kinds, kind)
}

func (sp SearchParams) ContainsContainer(container string) bool {
	return contains(sp.Containers, container)
}

func (sp SearchParams) ContainsName(name string) bool {
	return contains(sp.Names, name)
}

// contains performs a case-insensitive search for the item in the input array
func contains(arr []string, item string) bool {
	if len(arr) == 0 {
		return false
	}
	for _, str := range arr {
		if strings.EqualFold(str, item) {
			return true
		}
	}
	return false
}

// TODO: Change this to accept a string dictionary instead
func NewSearchParams(p *starlarkstruct.Struct) SearchParams {
	var (
		kinds      []string
		groups     []string
		names      []string
		namespaces []string
		versions   []string
		labels     []string
		containers []string
	)

	groups = parseStructAttr(p, "groups")
	kinds = parseStructAttr(p, "kinds")
	names = parseStructAttr(p, "names")
	namespaces = parseStructAttr(p, "namespaces")
	if len(namespaces) == 0 {
		namespaces = append(namespaces, "default")
	}
	versions = parseStructAttr(p, "versions")
	labels = parseStructAttr(p, "labels")
	containers = parseStructAttr(p, "containers")

	return SearchParams{
		Kinds:      kinds,
		Groups:     groups,
		Names:      names,
		Namespaces: namespaces,
		Versions:   versions,
		Labels:     labels,
		Containers: containers,
	}
}

func parseStructAttr(p *starlarkstruct.Struct, attrName string) []string {
	values := make([]string, 0)

	attrVal, err := p.Attr(attrName)
	if err == nil {
		values, err = parse(attrVal)
		if err != nil {
			logrus.Errorf("error while parsing attr %s: %v", attrName, err)
		}
	}
	return values
}

func parse(inputValue starlark.Value) ([]string, error) {
	var values []string
	var err error

	switch inputValue.Type() {
	case "string":
		val, ok := inputValue.(starlark.String)
		if !ok {
			err = errors.Errorf("cannot process starlark value %s", inputValue.String())
			break
		}
		values = append(values, val.GoString())
	case "list":
		val, ok := inputValue.(*starlark.List)
		if !ok {
			err = errors.Errorf("cannot process starlark value %s", inputValue.String())
			break
		}
		iter := val.Iterate()
		defer iter.Done()
		var x starlark.Value
		for iter.Next(&x) {
			str, _ := x.(starlark.String)
			values = append(values, str.GoString())
		}
	default:
		err = errors.New("unknown input type for parse()")
	}

	return values, err
}
