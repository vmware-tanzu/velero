/*
Copyright 2018 the Heptio Ark contributors.

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

package sync

import "sync"

// An ErrorGroup waits for a collection of goroutines that return errors to finish.
// The main goroutine calls Go one or more times to execute a function that returns
// an error in a goroutine. Then it calls Wait to wait for all goroutines to finish
// and collect the results of each.
type ErrorGroup struct {
	wg      sync.WaitGroup
	errChan chan error
}

// Go runs the specified function in a goroutine.
func (eg *ErrorGroup) Go(action func() error) {
	if eg.errChan == nil {
		eg.errChan = make(chan error)
	}

	eg.wg.Add(1)
	go func() {
		eg.errChan <- action()
		eg.wg.Done()
	}()
}

// GoErrorSlice runs a function that returns a slice of errors
// in a goroutine.
func (eg *ErrorGroup) GoErrorSlice(action func() []error) {
	if eg.errChan == nil {
		eg.errChan = make(chan error)
	}

	eg.wg.Add(1)
	go func() {
		for _, err := range action() {
			eg.errChan <- err
		}
		eg.wg.Done()
	}()
}

// Wait waits for all functions run via Go to finish,
// and returns all of their errors.
func (eg *ErrorGroup) Wait() []error {
	var errs []error
	go func() {
		for {
			errs = append(errs, <-eg.errChan)
		}
	}()

	eg.wg.Wait()
	return errs
}
