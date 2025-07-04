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

package encryption

import (
	"fmt"
	"io"
	"os"

	"errors"

	"filippo.io/age"
	"filippo.io/age/agessh"
)

type ageConfig struct {
	Recipient  string
	PrivateKey string
}

type AgeEncryption struct {
	recipient age.Recipient
	identity  age.Identity
}

// NewAgeEncryption creates AgeEncryption from provided options.
func NewAgeEncryption(opts map[string]string) (Encryption, error) {
	cfg, err := newAgeConfig(opts)
	if err != nil {
		return nil, err
	}
	r, err := parseRecipient(cfg.Recipient)
	if err != nil {
		return nil, fmt.Errorf("invalid age recipient: %w", err)
	}
	id, err := parseIdentity(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid age private key: %w", err)
	}
	return &AgeEncryption{
		recipient: r,
		identity:  id,
	}, nil
}

func (a *AgeEncryption) Encrypt(data io.Writer) (io.WriteCloser, error) {
	return age.Encrypt(data, a.recipient)
}

func (a *AgeEncryption) Decrypt(rc io.ReadCloser) (io.ReadCloser, error) {
	plain, err := age.Decrypt(rc, a.identity)
	if err != nil {
		_ = rc.Close()
		return nil, fmt.Errorf("failed to decrypt object: %w", err)
	}
	return io.NopCloser(plain), nil
}

func loadOption(opts map[string]string, envKey, optKey string) (string, error) {
	if v := os.Getenv(envKey); v != "" {
		return v, nil
	}
	if v, ok := opts[optKey]; ok && v != "" {
		return v, nil
	}
	return "", fmt.Errorf("missing configuration for %s (env %s or opts[%q])", optKey, envKey, optKey)
}

func newAgeConfig(opts map[string]string) (*ageConfig, error) {
	recipient, err := loadOption(opts, "AGE_RECIPIENT", "recipient")
	if err != nil {
		return nil, err
	}
	privKey, err := loadOption(opts, "AGE_PRIVATE_KEY", "privateKey")
	if err != nil {
		return nil, err
	}
	return &ageConfig{
		Recipient:  recipient,
		PrivateKey: privKey,
	}, nil
}

type parser[T any] struct {
	name string
	fn   func() (T, error)
}

func runParsers[T any](ps []parser[T], s string) (T, error) {
	var errs []error
	for _, p := range ps {
		v, err := p.fn()
		if err == nil {
			return v, nil
		}
		errs = append(errs, fmt.Errorf("%s parser: %w", p.name, err))
	}
	var zero T
	return zero, fmt.Errorf("unsupported %s format: %w", s, errors.Join(errs...))
}

func parseRecipient(s string) (age.Recipient, error) {
	return runParsers[age.Recipient]([]parser[age.Recipient]{
		{"x25519", func() (age.Recipient, error) { return age.ParseX25519Recipient(s) }},
		{"ssh", func() (age.Recipient, error) { return agessh.ParseRecipient(s) }},
	}, "recipient")
}

func parseIdentity(s string) (age.Identity, error) {
	return runParsers[age.Identity]([]parser[age.Identity]{
		{"x25519", func() (age.Identity, error) { return age.ParseX25519Identity(s) }},
		{"ssh", func() (age.Identity, error) { return agessh.ParseIdentity([]byte(s)) }},
	}, "identity")
}
