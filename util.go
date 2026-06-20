package main

import "github.com/google/uuid"

// randID returns a random identifier for tasks, connections and notifications.
func randID() string { return uuid.NewString() }
