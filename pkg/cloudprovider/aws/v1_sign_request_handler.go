/*
Copyright 2018 the Velero contributors.

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

package aws

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/pkg/errors"
)

var (
	errInvalidMethod = errors.New("v1 signer only handles HTTP GET")
)

type signer struct {
	// Values that must be populated from the request
	request     *request.Request
	time        time.Time
	credentials *credentials.Credentials
	debug       aws.LogLevelType
	logger      aws.Logger

	query        url.Values
	stringToSign string
	signature    string
}

// SignRequestHandler is a named request handler the SDK will use to sign
// service client request with using the V4 signature.
var v1SignRequestHandler = request.NamedHandler{
	Name: "v1.SignRequestHandler", Fn: signSDKRequest,
}

func signSDKRequest(req *request.Request) {
	// If the request does not need to be signed ignore the signing of the
	// request if the AnonymousCredentials object is used.
	if req.Config.Credentials == credentials.AnonymousCredentials {
		return
	}

	if req.HTTPRequest.Method != "GET" {
		// The V1 signer only supports GET
		req.Error = errInvalidMethod
		return
	}

	v1 := signer{
		request:     req,
		time:        req.Time,
		credentials: req.Config.Credentials,
		debug:       req.Config.LogLevel.Value(),
		logger:      req.Config.Logger,
	}

	req.Error = v1.sign()

	if req.Error != nil {
		return
	}

	req.HTTPRequest.URL.RawQuery = v1.query.Encode()
}

func (v1 *signer) sign() error {
	credentialsValue, err := v1.credentials.Get()
	if err != nil {
		return errors.Wrap(err, "error getting credentials")
	}

	httpRequest := v1.request.HTTPRequest

	v1.query = httpRequest.URL.Query()

	// Set new query parameters
	v1.query.Set("AWSAccessKeyId", credentialsValue.AccessKeyID)
	if credentialsValue.SessionToken != "" {
		v1.query.Set("SecurityToken", credentialsValue.SessionToken)
	}

	// in case this is a retry, ensure no signature present
	v1.query.Del("Signature")

	method := httpRequest.Method
	path := httpRequest.URL.Path
	if path == "" {
		path = "/"
	}

	duration := int64(v1.request.ExpireTime / time.Second)
	expires := strconv.FormatInt(duration, 10)
	// build the canonical string for the v1 signature
	v1.stringToSign = strings.Join([]string{
		method,
		"",
		"",
		expires,
		path,
	}, "\n")

	hash := hmac.New(sha1.New, []byte(credentialsValue.SecretAccessKey))
	hash.Write([]byte(v1.stringToSign))
	v1.signature = base64.StdEncoding.EncodeToString(hash.Sum(nil))
	v1.query.Set("Signature", v1.signature)
	v1.query.Set("Expires", expires)

	if v1.debug.Matches(aws.LogDebugWithSigning) {
		v1.logSigningInfo()
	}

	return nil
}

const logSignInfoMsg = `DEBUG: Request Signature:
---[ STRING TO SIGN ]--------------------------------
%s
---[ SIGNATURE ]-------------------------------------
%s
-----------------------------------------------------`

func (v1 *signer) logSigningInfo() {
	msg := fmt.Sprintf(logSignInfoMsg, v1.stringToSign, v1.query.Get("Signature"))
	v1.logger.Log(msg)
}
