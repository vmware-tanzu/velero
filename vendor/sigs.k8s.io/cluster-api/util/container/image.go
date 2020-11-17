/*
Copyright 2020 The Kubernetes Authors.

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

package container

import (
	"fmt"
	"path"
	"regexp"

	//  Import the crypto sha256 algorithm for the docker image parser to work
	_ "crypto/sha256"
	//  Import the crypto/sha512 algorithm for the docker image parser to work with 384 and 512 sha hashes
	_ "crypto/sha512"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
)

var (
	ociTagAllowedChars = regexp.MustCompile(`[^-a-zA-Z0-9_\.]`)
)

// Image type represents the container image details
type Image struct {
	Repository string
	Name       string
	Tag        string
	Digest     string
}

// ImageFromString parses a docker image string into three parts: repo, tag and digest.
func ImageFromString(image string) (Image, error) {
	named, err := reference.ParseNamed(image)
	if err != nil {
		return Image{}, fmt.Errorf("couldn't parse image name: %v", err)
	}

	var repo, tag, digest string
	_, nameOnly := path.Split(reference.Path(named))
	if nameOnly != "" {
		// split out the part of the name after the last /
		lenOfCompleteName := len(named.Name())
		repo = named.Name()[:lenOfCompleteName-len(nameOnly)-1]
	}

	tagged, ok := named.(reference.Tagged)
	if ok {
		tag = tagged.Tag()
	}

	digested, ok := named.(reference.Digested)
	if ok {
		digest = digested.Digest().String()
	}

	return Image{Repository: repo, Name: nameOnly, Tag: tag, Digest: digest}, nil
}

func (i Image) String() string {
	// repo/name [ ":" tag ] [ "@" digest ]
	ref := fmt.Sprintf("%s/%s", i.Repository, i.Name)
	if i.Tag != "" {
		ref = fmt.Sprintf("%s:%s", ref, i.Tag)
	}
	if i.Digest != "" {
		ref = fmt.Sprintf("%s@%s", ref, i.Digest)
	}
	return ref
}

// ModifyImageRepository takes an imageName (e.g., repository/image:tag), and returns an image name with updated repository
func ModifyImageRepository(imageName, repositoryName string) (string, error) {
	image, err := ImageFromString(imageName)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse image name")
	}
	nameUpdated, err := reference.WithName(path.Join(repositoryName, image.Name))
	if err != nil {
		return "", errors.Wrap(err, "failed to update repository name")
	}
	if image.Tag != "" {
		retagged, err := reference.WithTag(nameUpdated, image.Tag)
		if err != nil {
			return "", errors.Wrap(err, "failed to parse image tag")
		}
		return reference.FamiliarString(retagged), nil
	}
	return "", errors.New("image must be tagged")
}

// ModifyImageTag takes an imageName (e.g., repository/image:tag), and returns an image name with updated tag
func ModifyImageTag(imageName, tagName string) (string, error) {
	normalisedTagName := SemverToOCIImageTag(tagName)

	namedRef, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse image name")
	}
	// return error if images use digest as version instead of tag
	if _, isCanonical := namedRef.(reference.Canonical); isCanonical {
		return "", errors.New("image uses digest as version, cannot update tag ")
	}

	// update the image tag with tagName
	namedTagged, err := reference.WithTag(namedRef, normalisedTagName)
	if err != nil {
		return "", errors.Wrap(err, "failed to update image tag")
	}

	return reference.FamiliarString(reference.TagNameOnly(namedTagged)), nil
}

// ImageTagIsValid ensures that a given image tag is compliant with the OCI spec
func ImageTagIsValid(tagName string) bool {
	return !ociTagAllowedChars.MatchString(tagName)
}

// SemverToOCIImageTag is a helper function that replaces all
// non-allowed symbols in tag strings with underscores.
// Image tag can only contain lowercase and uppercase letters, digits,
// underscores, periods and dashes.
// Current usage is for CI images where all of symbols except '+' are valid,
// but function is for generic usage where input can't be always pre-validated.
// Taken from k8s.io/cmd/kubeadm/app/util
func SemverToOCIImageTag(version string) string {
	return ociTagAllowedChars.ReplaceAllString(version, "_")
}
