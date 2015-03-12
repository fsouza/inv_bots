// Copyright 2014 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"text/template"
	"time"

	"github.com/fsouza/inv_bots/lib"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	dbName                = "bovespa_plantao_empresas"
	newsCollName          = "news"
	notificationsCollName = "notifications"
)

var emailTemplate = template.Must(template.New("report").Parse(`Subject: {{.subject}}
To: {{.recipient}}
From: {{.sender}}

{{.subject}}

{{.link}}`))

var (
	baseURL    string
	sender     string
	password   string
	recipient  string
	tickerTime time.Duration
)

type News struct {
	ID    string `bson:"_id"`
	Title string
	Date  time.Time
}

type Notification struct {
	NewsID    string
	Date      time.Time
	Recipient string
}

func init() {
	flag.StringVar(&sender, "s", "", "Email address of the sender, for authentication in Gmail")
	flag.StringVar(&password, "p", "", "Email password of the sender, for authentication in Gmail")
	flag.StringVar(&recipient, "r", "", "Email address of the recipient")
	flag.StringVar(&baseURL, "u", "", "Base URL")
	flag.DurationVar(&tickerTime, "t", time.Minute, "Ticker interval")
}

func connect() (*mgo.Session, error) {
	return mgo.Dial("localhost:27017")
}

func notificationsCollection(session *mgo.Session) *mgo.Collection {
	collection := session.DB(dbName).C(notificationsCollName)
	collection.EnsureIndex(mgo.Index{Key: []string{"newsid"}, Background: true})
	return collection
}

func poolRecords(ticker <-chan time.Time) {
	for _ = range ticker {
		records := getRecords()
		if len(records) > 0 {
			notifyRecords(records)
		}
	}
}

func getNotificatedNews() ([]string, error) {
	session, err := connect()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	collection := notificationsCollection(session)
	var notifications []Notification
	err = collection.Find(nil).Select(bson.M{"newsid": 1}).All(&notifications)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(notifications))
	for i, notification := range notifications {
		result[i] = notification.NewsID
	}
	return result, nil
}

func getRecords() []News {
	session, err := connect()
	if err != nil {
		log.Printf("ERROR: %s", err)
		return nil
	}
	defer session.Close()
	notificated, err := getNotificatedNews()
	if err != nil {
		log.Printf("ERROR: %s", err)
		return nil
	}
	collection := session.DB(dbName).C(newsCollName)
	var newsList []News
	query := bson.M{
		"_id":   bson.M{"$nin": notificated},
		"title": bson.M{"$regex": "^fii.*(relatorio|informe|carta)", "$options": "i"},
	}
	err = collection.Find(query).All(&newsList)
	if err != nil {
		log.Printf("ERROR: %s", err)
		return nil
	}
	return newsList
}

func notifyRecords(newsList []News) {
	mailSender, err := lib.NewGmailSender(sender, password)
	if err != nil {
		log.Printf("ERROR: %s", err)
		return
	}
	defer mailSender.Close()
	session, err := connect()
	if err != nil {
		log.Printf("ERROR: %s", err)
		return
	}
	defer session.Close()
	for _, news := range newsList {
		var body bytes.Buffer
		emailTemplate.Execute(&body, map[string]string{
			"subject":   news.Title,
			"recipient": recipient,
			"sender":    sender,
			"link":      baseURL + news.ID,
		})
		err := mailSender.SendMail(recipient, body.Bytes())
		if err != nil {
			log.Printf("ERROR: %s", err)
			return
		}
		collection := notificationsCollection(session)
		collection.Insert(Notification{NewsID: news.ID, Recipient: recipient, Date: time.Now()})
	}
}

func main() {
	var failures int
	flag.Parse()
	if sender == "" {
		log.Print("Please provide the sender")
		failures++
	}
	if recipient == "" {
		log.Print("Please provide the recipient")
		failures++
	}
	if password == "" {
		log.Print("Please provide the password")
		failures++
	}
	if baseURL == "" {
		log.Print("Please provide the base URL")
		failures++
	}
	if failures > 0 {
		os.Exit(2)
	}
	poolRecords(time.Tick(tickerTime))
}
