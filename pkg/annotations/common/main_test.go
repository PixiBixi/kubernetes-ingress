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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSet(t *testing.T) {
	annotations := map[string]string{"check": "false"}
	require.True(t, IsSet("check", annotations), "present annotation is set")
	require.True(t, IsSet("check", map[string]string{"check": ""}), "an explicitly empty value is still a use")
	require.False(t, IsSet("load-balance", annotations), "absent annotation is not set")
	require.False(t, IsSet("check"), "no maps means nothing is set")
}
