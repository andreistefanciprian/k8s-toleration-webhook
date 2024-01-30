package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func main() {
	// Parse CLI params
	parameters := parseFlags()

	// Create a new https server
	httpsMux := mux.NewRouter()

	// webhookHandler handler
	httpsMux.HandleFunc("/mutate", webhookHandler)

	httpsAddr := ":" + strconv.Itoa(parameters.httpsPort)
	httpsServer := http.Server{
		Addr:    httpsAddr,
		Handler: httpsMux,
	}

	// Start the https server
	log.Printf("Starting https Server on port %s", httpsAddr)
	err := httpsServer.ListenAndServeTLS(parameters.certFile, parameters.keyFile)
	if err != nil {
		log.Fatal(err)
	}
}
