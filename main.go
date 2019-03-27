package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"./hn"
)

func main() {
	// parse flags
	var port, numStories int
	flag.IntVar(&port, "port", 3000, "the port to start the web server on")
	flag.IntVar(&numStories, "num_stories", 30, "the number of top stories to display")
	flag.Parse()

	tpl := template.Must(template.ParseFiles("./index.gohtml"))

  var client hn.Client
	http.HandleFunc("/", handler(&client, numStories, tpl))

	// Start the server
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func handler(client *hn.Client, numStories int, tpl *template.Template) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ids, err := client.TopItems()
		if err != nil {
			log.Fatal(err)
			return
		}
		fmt.Println("Retrieving stories")
    stories := client.RetrieveStories(numStories, ids)
		data := templateData{
			Stories: stories,
			Time:    time.Now().Sub(start),
		}
		err = tpl.Execute(w, data)
		if err != nil {
			log.Fatal(err)
			return
		}
	})
}

type templateData struct {
	Stories []*hn.ItemWithUrl
	Time    time.Duration
}
