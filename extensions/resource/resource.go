/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package resource

// Initialize is called at peer startup to initialize all registered resources.
func Initialize(providers ...interface{}) error {
	// Noop by default
	return nil
}

// ChannelJoined is called when the peer joins a channel.
func ChannelJoined(channelID string) {
	// Noop by default
}

// Close is called when the peer is shut down.
func Close() {
	// Noop by default
}
