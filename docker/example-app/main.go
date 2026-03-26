package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from example-app on port %s\n", port)
	})

	fmt.Printf("Listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil { // #nosec G114 -- example app, not production
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
