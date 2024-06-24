// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOperations_Enqueue(t *testing.T) {
	updateNegotiationNeededFlagOnEmptyChain := &atomicBool{}
	onNegotiationNeededCalledCount := 0
	var onNegotiationNeededCalledCountMu sync.Mutex
	ops := newOperations(updateNegotiationNeededFlagOnEmptyChain, func() {
		onNegotiationNeededCalledCountMu.Lock()
		onNegotiationNeededCalledCount++
		onNegotiationNeededCalledCountMu.Unlock()
	})
	for resultSet := 0; resultSet < 100; resultSet++ {
		results := make([]int, 16)
		resultSetCopy := resultSet
		for i := range results {
			func(j int) {
				ops.Enqueue(func() {
					results[j] = j * j
					if resultSetCopy > 50 {
						updateNegotiationNeededFlagOnEmptyChain.set(true)
					}
				})
			}(i)
		}

		ops.Done()
		expected := []int{0, 1, 4, 9, 16, 25, 36, 49, 64, 81, 100, 121, 144, 169, 196, 225}
		assert.Equal(t, len(expected), len(results))
		assert.Equal(t, expected, results)
	}
	onNegotiationNeededCalledCountMu.Lock()
	defer onNegotiationNeededCalledCountMu.Unlock()
	assert.NotEqual(t, onNegotiationNeededCalledCount, 0)
}

func TestOperations_Done(*testing.T) {
	ops := newOperations(&atomicBool{}, func() {
	})
	ops.Done()
}
