// Copyright 2015 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This services serves two feeds using news from "Plantão Empresas". One of
// the feeds is exclusive for FIIs and the other for all other news, excluding
// news related to FIIs.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/feeds"
	"github.com/tsuru/tsuru/db/storage"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	NewsURL = "http://www.bmfbovespa.com.br/agencia/corpo.asp?origem=exibir&id=%s"
	Limit   = 100
)

var (
	listenHTTP string
	regexpNews = regexp.MustCompile(`^/bovespa/(\d+)$`)
)

func init() {
	flag.StringVar(&listenHTTP, "listen", "127.0.0.1:7676", "address to listen to connections")
	flag.Parse()
}

type News struct {
	ID    string `bson:"_id"`
	Title string
	Date  time.Time
}

func (n *News) RedirectURL() string {
	return fmt.Sprintf(NewsURL, n.ID)
}

func (n *News) Path() string {
	return "/bovespa/" + n.ID
}

func collection() (*storage.Collection, error) {
	storage, err := storage.Open("localhost:27017", "bovespa_plantao_empresas")
	if err != nil {
		return nil, err
	}
	coll := storage.Collection("news")
	coll.EnsureIndex(mgo.Index{Key: []string{"title"}, Background: true, Sparse: true})
	coll.EnsureIndex(mgo.Index{Key: []string{"-date"}, Background: true, Sparse: true})
	coll.EnsureIndex(mgo.Index{Key: []string{"title", "-date"}, Background: true, Sparse: true})
	return coll, nil
}

func getFeed(query bson.M, id string, baseURL string) (*feeds.Feed, error) {
	if strings.HasSuffix(baseURL, "/") {
		baseURL = baseURL[:len(baseURL)-1]
	}
	coll, err := collection()
	if err != nil {
		return nil, err
	}
	defer coll.Close()
	var newsList []News
	err = coll.Find(query).Sort("-date").Limit(Limit).All(&newsList)
	if err != nil {
		return nil, err
	}
	location, _ := time.LoadLocation("America/Sao_Paulo")
	updated := time.Now()
	if len(newsList) > 0 {
		updated = newsList[0].Date.In(location)
	}
	feed := &feeds.Feed{
		Title:       "Bovespa - Plantão Empresas - " + id,
		Link:        &feeds.Link{Href: baseURL + "?w=" + id},
		Description: "Notícias sobre empresas listadas na Bovespa",
		Author:      &feeds.Author{Name: "Francisco Souza", Email: "f@souza.cc"},
		Created:     time.Date(2014, 3, 20, 10, 0, 0, 0, location),
		Updated:     updated,
	}
	for _, news := range newsList {
		item := feeds.Item{
			Id:          baseURL + news.Path(),
			Title:       news.Title,
			Link:        &feeds.Link{Href: baseURL + news.Path()},
			Description: news.Title,
			Author:      &feeds.Author{Name: "Bovespa", Email: "bovespa@bmfbovespa.com.br"},
			Created:     news.Date,
			Updated:     news.Date,
		}
		feed.Items = append(feed.Items, &item)
	}
	return feed, nil
}

func feedAll(w http.ResponseWriter, r *http.Request) {
	baseURL := "http://" + r.Host
	feed, err := getFeed(bson.M{"title": bson.M{"$regex": "^((?!fii))", "$options": "i"}}, "all", baseURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	atom, err := feed.ToAtom()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/xml")
	fmt.Fprint(w, atom)
}

func feedFIIs(w http.ResponseWriter, r *http.Request) {
	baseURL := "http://" + r.Host
	feed, err := getFeed(bson.M{"title": bson.M{"$regex": "^fii", "$options": "i"}}, "fii", baseURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	atom, err := feed.ToAtom()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/xml")
	fmt.Fprint(w, atom)
}

func redirectNews(w http.ResponseWriter, r *http.Request) {
	var newsID string
	var news News
	parts := regexpNews.FindStringSubmatch(r.URL.Path)
	if len(parts) > 1 {
		newsID = parts[1]
	} else {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}
	coll, err := collection()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer coll.Close()
	err = coll.FindId(newsID).One(&news)
	if err == mgo.ErrNotFound {
		http.Error(w, "News not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Location", news.RedirectURL())
	w.WriteHeader(http.StatusMovedPermanently)
}

func main() {
	http.Handle("/all.atom", http.HandlerFunc(feedAll))
	http.Handle("/fii.atom", http.HandlerFunc(feedFIIs))
	http.Handle("/", http.HandlerFunc(redirectNews))
	http.ListenAndServe(listenHTTP, nil)
}
