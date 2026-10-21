// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// Fresh keys per test invocation: Once state lives for the whole process.
var onceKeySeq atomic.Int64

func TestOnceRunsOncePerKey(t *testing.T) {
	key := fmt.Sprintf("key-%d", onceKeySeq.Add(1))
	var count atomic.Int32
	run := func() { count.Add(1) }

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			Once(key, run)
		})
	}
	wg.Wait()
	require.Equal(t, int32(1), count.Load(), "concurrent duplicate calls must run fn once")

	Once(key, run)
	require.Equal(t, int32(1), count.Load(), "later calls with the same key must be dropped")

	Once(fmt.Sprintf("key-%d", onceKeySeq.Add(1)), run)
	require.Equal(t, int32(2), count.Load(), "a different key must run fn")
}
