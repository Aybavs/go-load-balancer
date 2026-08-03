package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	name := flag.String("name", "backend", "backend name")
	addr := flag.String("addr", ":9001", "listen address")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "handled by %s\n", *name)
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	log.Printf("%s listening on %s", *name, *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
