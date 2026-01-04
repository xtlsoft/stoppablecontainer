/*
Copyright 2026.

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

// Package main implements a minimal pause binary that keeps the container running.
// This is used as a fallback entrypoint for containers that don't have a shell.
//
// The binary is statically compiled and can be injected into any container
// via a volume mount, allowing StoppableContainer to work with scratch-based
// and distroless images.
//
// Build with: CGO_ENABLED=0 go build -ldflags="-s -w" -o pause ./cmd/pause
package main

import (
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create a channel to receive signals
	sigChan := make(chan os.Signal, 1)

	// Register for SIGTERM and SIGINT
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Block forever until we receive a signal
	<-sigChan
}
