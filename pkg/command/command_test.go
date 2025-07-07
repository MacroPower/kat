package command_test

import (
	"time"

	"github.com/macropower/kat/pkg/command"
)

// collectEventsWithTimeout collects up to maxEvents from the channel with a timeout
func collectEventsWithTimeout(eventCh <-chan command.Event, maxEvents int, timeout time.Duration) []command.Event {
	var events []command.Event
	timeoutTimer := time.After(timeout)

	for len(events) < maxEvents {
		select {
		case event := <-eventCh:
			events = append(events, event)
		case <-timeoutTimer:
			return events
		}
	}

	return events
}
