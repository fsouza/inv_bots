// Copyright 2014 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This bot collects information from "Plantão Empresas", at Bovespa, scrapping
// the HTML.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/tsuru/tsuru/db/storage"
	"io/ioutil"
	"labix.org/v2/mgo"
	"launchpad.net/xmlpath"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const BaseURL = "http://www.bmfbovespa.com.br/Agencia-Noticias/ListarNoticias.aspx?idioma=pt-br&q=&tipoFiltro=%d&pg=%d"

var (
	pathLink              = xmlpath.MustCompile(`//ul[@id="linksNoticias"]/li/a`)
	pathHrefLink          = xmlpath.MustCompile("./@href")
	idRegexp              = regexp.MustCompile(`^ListarNoticias.aspx\?idioma=pt-br\&idNoticia=(\d+)\&.*$`)
	replaceLessThan       = []byte{' ', '<', ' '}
	replaceGreaterThan    = []byte{' ', '>', ' '}
	replaceLessOrEqual    = []byte{' ', '<', '=', ' '}
	replaceGreaterOrEqual = []byte{' ', '>', '=', ' '}
	tickerTimer           time.Duration
	filter                int
)

func init() {
	flag.DurationVar(&tickerTimer, "interval", 10*time.Minute, "Ticker interval")
	flag.IntVar(&filter, "filter", 0, "News filter (0 for daily, 1 for weekly)")
	flag.Parse()
}

type News struct {
	ID    string `bson:"_id"`
	Title string
	Date  time.Time
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

func downloadContent(page int) (*xmlpath.Node, error) {
	url := fmt.Sprintf(BaseURL, filter, page)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	content = bytes.Replace(content, replaceLessThan, nil, -1)
	content = bytes.Replace(content, replaceGreaterThan, nil, -1)
	content = bytes.Replace(content, replaceLessOrEqual, nil, -1)
	content = bytes.Replace(content, replaceGreaterOrEqual, nil, -1)
	node, err := xmlpath.ParseHTML(bytes.NewBuffer(content))
	if err != nil {
		return nil, err
	}
	return node, err
}

func saveNews(news []News) {
	coll, err := collection()
	if err != nil {
		log.Printf("[ERROR] Failed to save news: %s", err)
		return
	}
	defer coll.Close()
	for _, n := range news {
		_, err = coll.UpsertId(n.ID, n)
		if err != nil {
			log.Printf("[ERROR] Failed to save news: %s", err)
		}
	}
}

func collectNews(node *xmlpath.Node) []News {
	location, _ := time.LoadLocation("America/Sao_Paulo")
	var err error
	var newsList []News
	iter := pathLink.Iter(node)
	for iter.Next() {
		var news News
		target, ok := pathHrefLink.String(iter.Node())
		if !ok {
			continue
		}
		parts := idRegexp.FindStringSubmatch(target)
		if len(parts) > 1 {
			news.ID = parts[1]
		}
		content := iter.Node().String()
		content = strings.TrimSpace(content)
		parts = strings.SplitN(content, " - ", 2)
		if len(parts) < 2 {
			continue
		}
		news.Title = parts[1]
		news.Date, err = time.ParseInLocation("02/01/2006 15:04", parts[0], location)
		if err != nil {
			log.Printf("[WARNING] Wrong date for news: %s", err)
			continue
		}
		newsList = append(newsList, news)
	}
	return newsList
}

func run() {
	for i := 0; ; i++ {
		node, err := downloadContent(i + 1)
		if err != nil {
			log.Print(err)
		}
		if !pathLink.Exists(node) {
			break
		}
		newsList := collectNews(node)
		if len(newsList) > 0 {
			saveNews(newsList)
		}
	}
}

func main() {
	for _ = range time.Tick(tickerTimer) {
		run()
	}
}
