/*
Copyright 2017 Heptio Inc.

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

package encode

import (
	"bytes"
	"fmt"
	"io"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/scheme"
)

// Encode converts the provided object to the specified format
// and returns a byte slice of the encoded data.
func Encode(obj runtime.Object, format string) ([]byte, error) {
	buf := new(bytes.Buffer)

	if err := EncodeTo(obj, format, buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EncodeTo converts the provided object to the specified format and
// writes the encoded data to the provided io.Writer.
func EncodeTo(obj runtime.Object, format string, w io.Writer) error {
	encoder, err := EncoderFor(format)
	if err != nil {
		return err
	}

	return errors.WithStack(encoder.Encode(obj, w))
}

// EncoderFor gets the appropriate encoder for the specified format.
func EncoderFor(format string) (runtime.Encoder, error) {
	var encoder runtime.Encoder
	desiredMediaType := fmt.Sprintf("application/%s", format)
	serializerInfo, found := runtime.SerializerInfoForMediaType(scheme.Codecs.SupportedMediaTypes(), desiredMediaType)
	if !found {
		return nil, errors.Errorf("unable to locate an encoder for %q", desiredMediaType)
	}
	if serializerInfo.PrettySerializer != nil {
		encoder = serializerInfo.PrettySerializer
	} else {
		encoder = serializerInfo.Serializer
	}
	encoder = scheme.Codecs.EncoderForVersion(encoder, v1.SchemeGroupVersion)
	return encoder, nil
}
