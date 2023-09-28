// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOperations_Enqueue(t *testing.T) {
	ops := newOperations()
	for i := 0; i < 100; i++ {
		results := make([]int, 16)
		for i := range results {
			func(j int) {
				ops.Enqueue(func() {
					results[j] = j * j
				})
			}(i)
		}

		ops.Done()
		expected := []int{0, 1, 4, 9, 16, 25, 36, 49, 64, 81, 100, 121, 144, 169, 196, 225}
		assert.Equal(t, len(expected), len(results))
		assert.Equal(t, expected, results)
	}
}

func TestOperations_Done(*testing.T) {
	ops := newOperations()
	ops.Done()
}
