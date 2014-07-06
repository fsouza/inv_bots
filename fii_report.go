package main

import (
	"bytes"
	"flag"
	"github.com/fsouza/inv_bots/lib"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"log"
	"os"
	"text/template"
	"time"
)

const (
	dbName   = "bovespa_plantao_empresas"
	collName = "news"
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
	ID       string `bson:"_id"`
	Title    string
	Date     time.Time
	Notified bool
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

func poolRecords(ticker <-chan time.Time) {
	for _ = range ticker {
		records := getRecords()
		if len(records) > 0 {
			notifyRecords(records)
		}
	}
}

func getRecords() []News {
	session, err := connect()
	if err != nil {
		log.Printf("ERROR: %s", err)
		return nil
	}
	defer session.Close()
	collection := session.DB(dbName).C(collName)
	var newsList []News
	query := bson.M{
		"title": bson.M{"$regex": "^fii.*(relatorio|informe)", "$options": "i"},
		"notified": bson.M{"$ne": true},
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
		news.Notified = true
		session, err := connect()
		if err != nil {
			log.Printf("ERROR: %s", err)
			return
		}
		defer session.Close()
		collection := session.DB(dbName).C(collName)
		collection.UpdateId(news.ID, news)
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
