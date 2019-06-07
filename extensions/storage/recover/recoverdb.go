/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package recover

//RecoverDBHandler provides extension for recover db handler
func RecoverDBHandler(handle func() error) func() error {
	return handle
}
