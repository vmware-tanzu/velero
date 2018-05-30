package backup

import (
	"encoding/json"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

type objectMarshaler interface {
	Marshal(obj interface{}) ([]byte, error)
	Extension() string
}

type jsonObjectMarshaler struct {
	indent string
	pretty bool
}

func newJsonObjectMarshaler(opts ...jsonObjectMarshalerOption) objectMarshaler {
	m := &jsonObjectMarshaler{
		indent: "    ",
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

type jsonObjectMarshalerOption func(m *jsonObjectMarshaler)

func jsonIndent(indent string) jsonObjectMarshalerOption {
	return func(m *jsonObjectMarshaler) {
		m.indent = indent
	}
}

func jsonPretty(pretty bool) jsonObjectMarshalerOption {
	return func(m *jsonObjectMarshaler) {
		m.pretty = pretty
	}
}

func (m *jsonObjectMarshaler) Marshal(obj interface{}) ([]byte, error) {
	var (
		data []byte
		err  error
	)
	if m.pretty {
		data, err = json.MarshalIndent(obj, "", m.indent)
	} else {
		data, err = json.Marshal(obj)
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil

}

func (m *jsonObjectMarshaler) Extension() string {
	return "json"
}

type yamlObjectMarshaler struct {
}

func newYamlObjectMarshaler() objectMarshaler {
	m := &yamlObjectMarshaler{}
	return m
}

func (m *yamlObjectMarshaler) Marshal(obj interface{}) ([]byte, error) {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (m *yamlObjectMarshaler) Extension() string {
	return "yaml"
}
