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
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stringByteReceiver struct {
	buf       *bytes.Buffer
	chunkSize int
}

func (r *stringByteReceiver) Receive() ([]byte, error) {
	chunk := make([]byte, r.chunkSize)

	n, err := r.buf.Read(chunk)
	if err != nil {
		return nil, err
	}

	return chunk[0:n], nil
}

func (r *stringByteReceiver) CloseSend() error {
	r.buf = nil
	return nil
}

func TestStreamReader(t *testing.T) {
	s := "hello world, it's me, streamreader!!!!!"

	rdr := &stringByteReceiver{
		buf:       bytes.NewBufferString(s),
		chunkSize: 3,
	}

	sr := &StreamReadCloser{
		receive: rdr.Receive,
		close:   rdr.CloseSend,
	}

	res, err := ioutil.ReadAll(sr)

	require.Nil(t, err)
	assert.Equal(t, s, string(res))
}
