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

package output

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"

	"github.com/fvbommel/sortorder"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/jsonpath"
	"k8s.io/klog/v2"
	"k8s.io/utils/integer"
)

// Sorter sorts the given obj list.
// Non-list types are simply passed through
type Sorter struct {
	SortField string
	Decoder   runtime.Decoder
}

func (s *Sorter) SortObj(obj runtime.Object) error {
	objs, err := meta.ExtractList(obj)
	if err != nil {
		return err
	}
	if len(objs) == 0 {
		return nil
	}

	sorter, err := SortObjects(nil, objs, s.SortField)
	if err != nil {
		return err
	}

	switch list := obj.(type) {
	case *corev1.List:
		outputList := make([]runtime.RawExtension, len(objs))
		for ix := range objs {
			outputList[ix] = list.Items[sorter.OriginalPosition(ix)]
		}
		list.Items = outputList
		return nil
	}
	return meta.SetList(obj, objs)
}

var jsonRegexp = regexp.MustCompile(`^\{\.?([^{}]+)\}$|^\.?([^{}]+)$`)

// RelaxedJSONPathExpression attempts to be flexible with JSONPath expressions, it accepts:
//   - metadata.name (no leading '.' or curly braces '{...}'
//   - {metadata.name} (no leading '.')
//   - .metadata.name (no curly braces '{...}')
//   - {.metadata.name} (complete expression)
//
// And transforms them all into a valid jsonpath expression:
//
//	{.metadata.name}
func RelaxedJSONPathExpression(pathExpression string) (string, error) {
	if len(pathExpression) == 0 {
		return pathExpression, nil
	}
	submatches := jsonRegexp.FindStringSubmatch(pathExpression)
	if submatches == nil {
		return "", fmt.Errorf("unexpected path string, expected a 'name1.name2' or '.name1.name2' or '{name1.name2}' or '{.name1.name2}'")
	}
	if len(submatches) != 3 {
		return "", fmt.Errorf("unexpected submatch list: %v", submatches)
	}
	var fieldSpec string
	if len(submatches[1]) != 0 {
		fieldSpec = submatches[1]
	} else {
		fieldSpec = submatches[2]
	}
	return fmt.Sprintf("{.%s}", fieldSpec), nil
}

// SortObjects sorts the runtime.Object based on fieldInput and returns RuntimeSort that implements
// the golang sort interface
func SortObjects(decoder runtime.Decoder, objs []runtime.Object, fieldInput string) (*RuntimeSort, error) {
	for ix := range objs {
		item := objs[ix]
		switch u := item.(type) {
		case *runtime.Unknown:
			var err error
			// decode runtime.Unknown to runtime.Unstructured for sorting.
			// we don't actually want the internal versions of known types.
			if objs[ix], _, err = decoder.Decode(u.Raw, nil, &unstructured.Unstructured{}); err != nil {
				return nil, err
			}
		}
	}

	field, err := RelaxedJSONPathExpression(fieldInput)
	if err != nil {
		return nil, err
	}

	parser := jsonpath.New("sorting").AllowMissingKeys(true)
	if err := parser.Parse(field); err != nil {
		return nil, err
	}

	// We don't do any model validation here, so we traverse all objects to be sorted
	// and, if the field is valid to at least one of them, we consider it to be a
	// valid field; otherwise error out.
	// Note that this requires empty fields to be considered later, when sorting.
	var fieldFoundOnce bool
	for _, obj := range objs {
		values, err := findJSONPathResults(parser, obj)
		if err != nil {
			return nil, err
		}
		if len(values) > 0 && len(values[0]) > 0 {
			fieldFoundOnce = true
			break
		}
	}
	if !fieldFoundOnce {
		return nil, fmt.Errorf("couldn't find any field with path %q in the list of objects", field)
	}

	sorter := NewRuntimeSort(field, objs)
	sort.Sort(sorter)
	return sorter, nil
}

// RuntimeSort is an implementation of the golang sort interface that knows how to sort
// lists of runtime.Object
type RuntimeSort struct {
	field        string
	objs         []runtime.Object
	origPosition []int
}

// NewRuntimeSort creates a new RuntimeSort struct that implements golang sort interface
func NewRuntimeSort(field string, objs []runtime.Object) *RuntimeSort {
	sorter := &RuntimeSort{field: field, objs: objs, origPosition: make([]int, len(objs))}
	for ix := range objs {
		sorter.origPosition[ix] = ix
	}
	return sorter
}

func (r *RuntimeSort) Len() int {
	return len(r.objs)
}

func (r *RuntimeSort) Swap(i, j int) {
	r.objs[i], r.objs[j] = r.objs[j], r.objs[i]
	r.origPosition[i], r.origPosition[j] = r.origPosition[j], r.origPosition[i]
}

func isLess(i, j reflect.Value) (bool, error) {
	switch i.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return i.Int() < j.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return i.Uint() < j.Uint(), nil
	case reflect.Float32, reflect.Float64:
		return i.Float() < j.Float(), nil
	case reflect.String:
		return sortorder.NaturalLess(i.String(), j.String()), nil
	case reflect.Ptr:
		return isLess(i.Elem(), j.Elem())
	case reflect.Struct:
		// sort metav1.Time
		in := i.Interface()
		if t, ok := in.(metav1.Time); ok {
			time := j.Interface().(metav1.Time)
			return t.Before(&time), nil
		}
		// sort resource.Quantity
		if iQuantity, ok := in.(resource.Quantity); ok {
			jQuantity := j.Interface().(resource.Quantity)
			return iQuantity.Cmp(jQuantity) < 0, nil
		}
		// fallback to the fields comparison
		for idx := 0; idx < i.NumField(); idx++ {
			less, err := isLess(i.Field(idx), j.Field(idx))
			if err != nil || !less {
				return less, err
			}
		}
		return true, nil
	case reflect.Array, reflect.Slice:
		// note: the length of i and j may be different
		for idx := 0; idx < integer.IntMin(i.Len(), j.Len()); idx++ {
			less, err := isLess(i.Index(idx), j.Index(idx))
			if err != nil || !less {
				return less, err
			}
		}
		return true, nil
	case reflect.Interface:
		if i.IsNil() && j.IsNil() {
			return false, nil
		} else if i.IsNil() {
			return true, nil
		} else if j.IsNil() {
			return false, nil
		}
		switch itype := i.Interface().(type) {
		case uint8:
			if jtype, ok := j.Interface().(uint8); ok {
				return itype < jtype, nil
			}
		case uint16:
			if jtype, ok := j.Interface().(uint16); ok {
				return itype < jtype, nil
			}
		case uint32:
			if jtype, ok := j.Interface().(uint32); ok {
				return itype < jtype, nil
			}
		case uint64:
			if jtype, ok := j.Interface().(uint64); ok {
				return itype < jtype, nil
			}
		case int8:
			if jtype, ok := j.Interface().(int8); ok {
				return itype < jtype, nil
			}
		case int16:
			if jtype, ok := j.Interface().(int16); ok {
				return itype < jtype, nil
			}
		case int32:
			if jtype, ok := j.Interface().(int32); ok {
				return itype < jtype, nil
			}
		case int64:
			if jtype, ok := j.Interface().(int64); ok {
				return itype < jtype, nil
			}
		case uint:
			if jtype, ok := j.Interface().(uint); ok {
				return itype < jtype, nil
			}
		case int:
			if jtype, ok := j.Interface().(int); ok {
				return itype < jtype, nil
			}
		case float32:
			if jtype, ok := j.Interface().(float32); ok {
				return itype < jtype, nil
			}
		case float64:
			if jtype, ok := j.Interface().(float64); ok {
				return itype < jtype, nil
			}
		case string:
			if jtype, ok := j.Interface().(string); ok {
				// check if it's a Quantity
				itypeQuantity, err := resource.ParseQuantity(itype)
				if err != nil {
					return sortorder.NaturalLess(itype, jtype), nil
				}
				jtypeQuantity, err := resource.ParseQuantity(jtype)
				if err != nil {
					return sortorder.NaturalLess(itype, jtype), nil
				}
				// Both strings are quantity
				return itypeQuantity.Cmp(jtypeQuantity) < 0, nil
			}
		default:
			return false, fmt.Errorf("unsortable type: %T", itype)
		}
		return false, fmt.Errorf("unsortable interface: %v", i.Kind())

	default:
		return false, fmt.Errorf("unsortable type: %v", i.Kind())
	}
}

func (r *RuntimeSort) Less(i, j int) bool {
	iObj := r.objs[i]
	jObj := r.objs[j]

	var iValues [][]reflect.Value
	var jValues [][]reflect.Value
	var err error

	parser := jsonpath.New("sorting").AllowMissingKeys(true)
	err = parser.Parse(r.field)
	if err != nil {
		panic(err)
	}

	iValues, err = findJSONPathResults(parser, iObj)
	if err != nil {
		klog.Fatalf("Failed to get i values for %#v using %s (%#v)", iObj, r.field, err)
	}

	jValues, err = findJSONPathResults(parser, jObj)
	if err != nil {
		klog.Fatalf("Failed to get j values for %#v using %s (%v)", jObj, r.field, err)
	}

	if len(iValues) == 0 || len(iValues[0]) == 0 {
		return true
	}
	if len(jValues) == 0 || len(jValues[0]) == 0 {
		return false
	}
	iField := iValues[0][0]
	jField := jValues[0][0]

	less, err := isLess(iField, jField)
	if err != nil {
		klog.Exitf("Field %s in %T is an unsortable type: %s, err: %v", r.field, iObj, iField.Kind().String(), err)
	}
	return less
}

// OriginalPosition returns the starting (original) position of a particular index.
// e.g. If OriginalPosition(0) returns 5 than the
// the item currently at position 0 was at position 5 in the original unsorted array.
func (r *RuntimeSort) OriginalPosition(ix int) int {
	if ix < 0 || ix > len(r.origPosition) {
		return -1
	}
	return r.origPosition[ix]
}

func findJSONPathResults(parser *jsonpath.JSONPath, from runtime.Object) ([][]reflect.Value, error) {
	if unstructuredObj, ok := from.(*unstructured.Unstructured); ok {
		return parser.FindResults(unstructuredObj.Object)
	}
	return parser.FindResults(reflect.ValueOf(from).Elem().Interface())
}
