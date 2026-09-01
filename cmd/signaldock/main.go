package main

import (
	"log"
	"net/http"

	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/web"
)

const port = "8080"

func main() {
	store := endpoint.Store{}

	store.Add("https://example.com/")

	handler, err := web.NewHandler(&store)
	if err != nil {
		log.Fatal(err)
	}

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
