package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/ElMehdi19/gophercises/quiet_hn/hn"
)

var (
	numStories, port int
)

func main() {
	flag.IntVar(&numStories, "num_stories", 30, "how many stories to display")
	flag.IntVar(&port, "port", 5000, "port to start the web server on")
	flag.Parse()

	tpl := template.Must(template.ParseFiles("./index.gohtml"))

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", handler(numStories, tpl))

	log.Printf("Running on http://127.0.0.1:%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))

}

func handler(numStories int, tpl *template.Template) http.HandlerFunc {
	sc := storyCache{
		duration: 6 * time.Second,
	}

	go func() {

		ticker := time.NewTicker(sc.duration - 3)

		for {
			temp := storyCache{
				duration: sc.duration,
			}
			temp.getStories()
			sc.mutex.Lock()
			sc.cache = temp.cache
			sc.expiration = temp.expiration
			sc.mutex.Unlock()
			<-ticker.C
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		stories, err := sc.getStories()
		if err != nil {
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			return
		}

		data := templateData{
			Stories: stories,
			Time:    time.Now().Sub(started),
		}

		if err := tpl.Execute(w, data); err != nil {
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
	})
}

type storyCache struct {
	cache      []item
	duration   time.Duration
	expiration time.Time
	mutex      sync.Mutex
}

// func (sc *storyCache) refreshCache(numStories int) {
// }

func (sc *storyCache) getStories() ([]item, error) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	if sc.expiration.Sub(time.Now()) > 0 {
		return sc.cache, nil
	}

	items, err := getStoriesAsync()
	if err != nil {
		return nil, err
	}

	sc.cache = items
	sc.expiration = time.Now().Add(sc.duration)
	return sc.cache, nil
}

func getStoriesAsync() ([]item, error) {
	var client hn.Client
	ids, err := client.GetItems()
	if err != nil {
		return nil, err
	}

	type result struct {
		idx  int
		item item
		err  error
	}

	resultChan := make(chan result)
	for i, id := range ids {
		go func(i, id int) {
			story, err := client.GetItem(id)
			if err != nil {
				resultChan <- result{err: err, idx: i}
			} else {
				resultChan <- result{item: parseItem(story), idx: i}
			}
		}(i, id)
		if i >= numStories {
			break
		}
	}

	var results []result
	for i := 0; i < numStories; i++ {
		results = append(results, <-resultChan)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].idx < results[j].idx
	})

	var stories []item
	for _, res := range results {
		if res.err != nil {
			continue
		}
		if isStoryLink(res.item) {
			stories = append(stories, res.item)
		}
	}

	return stories, nil
}

type item struct {
	hn.Item
	Host string
}

type templateData struct {
	Stories []item
	Time    time.Duration
}

func parseItem(hnItem hn.Item) item {
	newItem := item{Item: hnItem}
	uri, err := url.Parse(newItem.URL)
	if err == nil {
		newItem.Host = strings.TrimPrefix(uri.Host, "www.")
	}
	return newItem
}

func isStoryLink(hnItem item) bool {
	return hnItem.Type == "story" && hnItem.URL != ""
}
