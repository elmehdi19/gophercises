package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

var port int

func main() {
	flag.IntVar(&port, "port", 5000, "port to run the web app on")
	flag.Parse()

	handler := newHandler()

	log.Printf("Running on http://127.0.0.1:%d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), handler))
}

func newHandler() http.HandlerFunc {
	mux := http.NewServeMux()
	mux.HandleFunc("/", home)
	mux.HandleFunc("/panic", panicDemo)
	mux.HandleFunc("/debug/", sourceCodeHandler)

	return loggingMw(recoverMw(mux, isDevMode()))
}
