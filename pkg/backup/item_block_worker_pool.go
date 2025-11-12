/*
Copyright the Velero Contributors.

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

package backup

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ItemBlockWorkerPool struct {
	inputChannel chan ItemBlockInput
	wg           *sync.WaitGroup
	logger       logrus.FieldLogger
	cancelFunc   context.CancelFunc
	channelMu    sync.Mutex // Protects channelOpen
	channelOpen  bool       // Tracks if channel is still open
}

type ItemBlockInput struct {
	itemBlock  *BackupItemBlock
	returnChan chan ItemBlockReturn
}

type ItemBlockReturn struct {
	itemBlock *BackupItemBlock
	resources []schema.GroupResource
	err       error
}

func (p *ItemBlockWorkerPool) GetInputChannel() chan ItemBlockInput {
	return p.inputChannel
}

func StartItemBlockWorkerPool(ctx context.Context, workers int, log logrus.FieldLogger) *ItemBlockWorkerPool {
	// Buffer will hold up to 10 ItemBlocks waiting for processing
	inputChannel := make(chan ItemBlockInput, max(workers, 10))

	ctx, cancelFunc := context.WithCancel(ctx)
	wg := &sync.WaitGroup{}

	for i := 0; i < workers; i++ {
		logger := log.WithField("worker", i)
		wg.Add(1)
		go processItemBlockWorker(ctx, inputChannel, logger, wg)
	}

	pool := &ItemBlockWorkerPool{
		inputChannel: inputChannel,
		cancelFunc:   cancelFunc,
		logger:       log,
		wg:           wg,
		channelOpen:  true, // Channel starts open
	}
	return pool
}

func (p *ItemBlockWorkerPool) Stop() {
	p.cancelFunc()
	p.logger.Info("ItemBlock worker stopping")
	p.wg.Wait()
	p.logger.Info("ItemBlock worker stopped")
}

// CloseChannel safely closes the input channel to signal workers to stop accepting new work.
// This is used for Tier 1 cancellation - preventing new ItemBlocks from being queued.
// It's safe to call this multiple times (idempotent).
func (p *ItemBlockWorkerPool) CloseChannel() {
	p.channelMu.Lock()
	defer p.channelMu.Unlock()

	if p.channelOpen {
		close(p.inputChannel)
		p.channelOpen = false
		p.logger.Info("ItemBlock input channel closed for cancellation")
	}
}

// processItemBlockWorker is the main worker function for the ItemBlockWorkerPool.
// Each worker runs this func in a loop, processing ItemBlocks as they are received.
func processItemBlockWorker(ctx context.Context,
	inputChannel chan ItemBlockInput,
	logger logrus.FieldLogger,
	wg *sync.WaitGroup) {
	defer wg.Done() // Ensure WaitGroup is decremented when worker exits

	for {
		select {
		case m, ok := <-inputChannel:
			// TIER 1: Channel closed (no more work)
			if !ok {
				logger.Info("ItemBlock channel closed, worker exiting")
				return
			}

			// Get backup context for this ItemBlock
			// If not set (e.g., in tests), use background context
			backupCtx := m.itemBlock.itemBackupper.backupRequest.BackupContext
			if backupCtx == nil {
				backupCtx = context.Background()
			}

			// TIER 2: Check if backup is already cancelled before processing
			select {
			case <-backupCtx.Done():
				// Backup cancelled - send error result to ensure WaitGroup decrements
				logger.Info("Backup cancelled, skipping ItemBlock processing")
				m.returnChan <- ItemBlockReturn{
					itemBlock: m.itemBlock,
					resources: nil,
					err:       backupCtx.Err(), // context.Canceled
				}
				continue // Don't process, go to next iteration
			default:
				// Context still valid, continue processing
			}

			// Process ItemBlock with backup context
			logger.Infof("Processing ItemBlock for backup %v", m.itemBlock.itemBackupper.backupRequest.Name)
			grList := m.itemBlock.itemBackupper.kubernetesBackupper.backupItemBlock(
				backupCtx, // Pass context for Tier 2 cancellation
				m.itemBlock,
			)
			logger.Infof("Finished processing ItemBlock for backup %v", m.itemBlock.itemBackupper.backupRequest.Name)

			// Check if context was cancelled during processing
			var err error
			if backupCtx.Err() != nil {
				err = backupCtx.Err()
				logger.Infof("Context was cancelled during ItemBlock processing")
			}

			// Always send result (even if cancelled mid-processing)
			m.returnChan <- ItemBlockReturn{
				itemBlock: m.itemBlock,
				resources: grList,
				err:       err,
			}

		case <-ctx.Done():
			// Worker pool context cancelled (controller shutdown)
			logger.Info("Worker pool context cancelled, exiting")
			return
		}
	}
}
