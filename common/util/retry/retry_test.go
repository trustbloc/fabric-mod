/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package retry

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestWithMaxAttempt(t *testing.T) {
	count := 0
	_, err := Invoke(
		func() (interface{}, error) {
			count++
			return nil, errors.New("error")

		},
		WithMaxAttempts(4),
	)

	require.Error(t, err)
	require.Equal(t, 4, count)

	count = 0
	v, err := Invoke(
		func() (interface{}, error) {
			count++
			if count == 4 {
				return "success", nil
			}
			return nil, errors.New("error")

		},
		WithMaxAttempts(4),
	)

	require.NoError(t, err)
	require.Equal(t, 4, count)
	require.Equal(t, "success", v)
}

func TestWithBeforeRetry(t *testing.T) {
	count := 0
	_, err := Invoke(
		func() (interface{}, error) {
			count++
			if count == 2 {
				return nil, errors.New("noretry")
			}
			return nil, errors.New("retry")

		},
		WithBeforeRetry(func(err error, attempt int, backoff time.Duration) bool {
			require.Error(t, err)
			if err.Error() == "retry" {
				return true
			}
			require.Equal(t, "noretry", err.Error())
			require.Equal(t, 2, attempt)
			return false
		}),
		WithMaxAttempts(4),
	)

	require.Error(t, err)
	require.Equal(t, 2, count)
}
