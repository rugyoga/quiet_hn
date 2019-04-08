// Package hn implements a really basic Hacker News client
package hn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
)

const (
	apiBase = "https://hacker-news.firebaseio.com/v0"
)

type Client struct {
	apiBase string
	cache *ItemCache
}

func (client *Client) Singleton() {
	if client.cache == nil {
		client.apiBase = apiBase
		client.cache = ItemCacheSingleton()
	}
}

func (client *Client) TopItems() ([]int, error) {
	var ids []int
	resp, err := http.Get(fmt.Sprintf("%s/topstories.json", client.apiBase))
	if err != nil { return nil, err }
	err = json.NewDecoder(resp.Body).Decode(&ids)
	if err != nil { return nil, err }
	err = resp.Body.Close()
	if err != nil { return nil, err }
	return ids, err
}

func (client *Client) processItem(id int, items chan *ItemWithUrl, storyCount Counter) {
	item, ok := client.cache.Get(id)
	if !ok {
		resp, err := http.Get(fmt.Sprintf("%s/item/%d.json", client.apiBase, id))
		if err != nil { return }
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&item)
		if err != nil { return }
		err = resp.Body.Close()
		if err != nil { return }
		client.cache.Put(id, item)
	}
	if itemWithUrl := addHost(item); isStoryLink(itemWithUrl) {
	  items <- itemWithUrl
		storyCount.incr()
  }
}

func (client *Client) worker(w int, storyCount Counter, coreCount Counter, nextStoryId IntValue, items chan *ItemWithUrl) {
	for !storyCount.done() {
		client.processItem(nextStoryId(), items, storyCount)
  }
	coreCount.incr()
	if coreCount.done() {
		close(items)
	}
}

type IntValue func() int
type Action func()
type Condition func() bool
type Counter struct {
  incr Action
	done Condition
}

func mkNextItem(items []int) IntValue {
	var mutex sync.Mutex
	i := 0
  return func() int {
		mutex.Lock()
		defer mutex.Unlock()
		item := items[i]
		i++
		return item
	}
}

func mkCounter(max int) Counter {
	i := max
	var mutex sync.RWMutex
	return Counter{
		incr: func() { mutex.Lock(); defer mutex.Unlock(); i--  },
		done: func() bool { mutex.RLock(); defer mutex.RUnlock(); return i <= 0 },
	}
}

const NumOfCores=12

func (client *Client) RetrieveStories(numStories int, ids []int) []*ItemWithUrl {
	var stories []*ItemWithUrl
	itemWithUrls := make(chan *ItemWithUrl, 2*numStories)
	nextStoryId := mkNextItem(ids)
  storyCount := mkCounter(numStories)
	coreCount := mkCounter(NumOfCores)

	for core := 1; core <= NumOfCores; core++ {
		go client.worker(core, storyCount, coreCount, nextStoryId, itemWithUrls)
	}

	for item, ok := <- itemWithUrls; ok; item, ok = <- itemWithUrls{
		stories = append(stories, item)
	}

	sort.Slice(stories, func(i, j int) bool {
		return stories[i].Item.ID < stories[j].Item.ID
	})
	return stories[:numStories]
}

type Item struct {
	By          string `json:"by"`
	Descendants int    `json:"descendants"`
	ID          int    `json:"id"`
	Kids        []int  `json:"kids"`
	Score       int    `json:"score"`
	Time        int    `json:"time"`
	Title       string `json:"title"`
	Type        string `json:"type"`

	Text string `json:"text"`
	URL  string `json:"url"`
}

func isStoryLink(item *ItemWithUrl) bool {
	return item.Item.Type == "story" && item.Item.URL != ""
}

func addHost(item *Item) *ItemWithUrl {
	ret := ItemWithUrl{Item: item}
	if parsedUrl, err := url.Parse(ret.Item.URL); err == nil {
		ret.Host = strings.TrimPrefix(parsedUrl.Hostname(), "www.")
	}
	return &ret
}

type ItemWithUrl struct {
	Item *Item
	Host string
}
