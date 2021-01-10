package main

import (
	"log"
	"net/http"

	"github.com/Djuno-Ltd/agent/djuno/task"
	"github.com/docker/docker/client"
)

func main() {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Printf("ERROR: Docker client initialization failed.")
		panic(err)
	}
	log.Printf("INFO: Waiting for Djuno...")
	djuno.HealthCheck()

	go task.HandleEvents(cli)
	log.Printf("INFO: Event collector started.")
	go task.HandleStats(cli)
	log.Printf("INFO: Stats collector started.")

	router := NewRouter(cli)
	log.Fatal(http.ListenAndServe(":8080", router))
	log.Printf("INFO: Djuno agent listening on port 8080")
}
