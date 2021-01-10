package task

import (
	"context"
	"io"
	"log"

	"github.com/Djuno-Ltd/agent/djuno"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func HandleEvents(cli *client.Client) {
	messages, errs := cli.Events(context.Background(), types.EventsOptions{})

loop:
	for {
		select {
		case err := <-errs:
			if err != nil && err != io.EOF {
				log.Printf("ERROR: Event channel error: %s", err)
			}
			break loop
		case msg, ok := <-messages:
			if !ok {
				log.Printf("ERROR: Event channel closed.")
				break loop
			}
			djuno.SendEvent(djuno.EVENT, msg)
		}
	}
	panic("Event collector is broken. Shutdown!!!")
}
