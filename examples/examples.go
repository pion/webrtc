package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Examples represents the examples loaded from examples.json.
type Examples []Example

// Example represents an example loaded from examples.json.
type Example struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

func main() {
	addr := flag.String("address", ":80", "Address to host the HTTP server on.")
	flag.Parse()

	log.Println("Listening on", *addr)
	err := serve(*addr)
	if err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func serve(addr string) error {
	// Load the examples
	examples, err := getExamples()
	if err != nil {
		return err
	}

	// Load the templates
	homeTemplate := template.Must(template.ParseFiles("index.html"))

	// Serve the required pages
	// DIY 'mux' to avoid additional dependencies
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) > 3 && // 1 / example:2 / link:3 / [ static: 4 ]
			parts[1] == "example" {
			exampleLink := parts[2]
			for _, example := range *examples {
				if example.Link != exampleLink {
					continue
				}
				fiddle := filepath.Join(exampleLink, "jsfiddle")
				if len(parts[3]) != 0 {
					http.StripPrefix("/example/"+exampleLink+"/", http.FileServer(http.Dir(fiddle))).ServeHTTP(w, r)
					return
				}

				temp := template.Must(template.ParseFiles("example.html"))
				_, err = temp.ParseFiles(filepath.Join(fiddle, "demo.html"))
				if err != nil {
					panic(err)
				}

				temp.Execute(w, example)
				return
			}
		}

		// Serve the main page
		homeTemplate.Execute(w, examples)
	})

	// Start the server
	return http.ListenAndServe(addr, nil)
}

// getExamples loads the examples from the examples.json file.
func getExamples() (*Examples, error) {
	file, err := os.Open("./examples.json")
	if err != nil {
		return nil, fmt.Errorf("failed to list examples (please run in the examples folder): %v", err)
	}
	defer file.Close()

	var examples Examples
	err = json.NewDecoder(file).Decode(&examples)
	if err != nil {
		return nil, fmt.Errorf("failed to parse examples: %v", err)
	}

	return &examples, nil
}
