package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sirupsen/logrus"
)

func TestArgsToFields(t *testing.T) {
	tests := []struct {
		name           string
		args           []interface{}
		expectedFields logrus.Fields
	}{
		{
			name:           "empty args results in empty map of fields",
			args:           []interface{}{},
			expectedFields: logrus.Fields(map[string]interface{}{}),
		},
		{
			name: "matching string keys/values are correctly set as fields",
			args: []interface{}{"key-1", "value-1", "key-2", "value-2"},
			expectedFields: logrus.Fields(map[string]interface{}{
				"key-1": "value-1",
				"key-2": "value-2",
			}),
		},
		{
			name: "time/timestamp/level entries are removed",
			args: []interface{}{"time", time.Now(), "key-1", "value-1", "timestamp", time.Now(), "key-2", "value-2", "level", "WARN"},
			expectedFields: logrus.Fields(map[string]interface{}{
				"key-1": "value-1",
				"key-2": "value-2",
			}),
		},
		{
			name: "odd number of args adds the last arg as a field with a nil value",
			args: []interface{}{"key-1", "value-1", "key-2", "value-2", "key-3"},
			expectedFields: logrus.Fields(map[string]interface{}{
				"key-1": "value-1",
				"key-2": "value-2",
				"key-3": nil,
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedFields, argsToFields(test.args...))
		})
	}
}
