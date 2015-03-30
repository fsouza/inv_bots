// Copyright 2014 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This bot collects all material facts about companies listed in the Bovespa
// stock exchange and send them via email, using Gmail's SMTP server.
//
// Users can customize at runtime the interval of the queries, and information
// about the sender and recipient of the email.
package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"launchpad.net/xmlpath"
	"log"
	"net/http"
	"net/smtp"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

const (
	dbName      = "cvm_fatos_relevantes"
	collName    = "records"
	listURL     = "http://siteempresas.bovespa.com.br/consbov/ExibeFatosRelevantesCvm.asp?pagina="
	protocolURL = "http://siteempresas.bovespa.com.br/consbov/ArquivosExibe.asp?protocolo="
)

var emailTemplate = template.Must(template.New("fatorelevante").Parse(`Subject: [FATO RELEVANTE] {{.company}}
To: {{.recipient}}
From: {{.sender}}

{{.subject}}

Data de Envio: {{.sendDate}}
Data de Referência: {{.referenceDate}}

{{.link}}`))

var (
	pathTR        = xmlpath.MustCompile("//table/tr")
	pathSend      = xmlpath.MustCompile("./td[1]")
	pathReference = xmlpath.MustCompile("./td[2]")
	pathSubject   = xmlpath.MustCompile("./td[3]")
	pathLink      = xmlpath.MustCompile("./td[3]/a")
	pathLinkHref  = xmlpath.MustCompile("./td[3]/a/@href")
	regexpLink    = regexp.MustCompile(`Javascript:AbreArquivo\('(\d+)'\)`)
	sender        string
	password      string
	recipient     string
	tickerTime    time.Duration
)

func init() {
	flag.StringVar(&sender, "s", "", "Email address of the sender, for authentication in Gmail")
	flag.StringVar(&password, "p", "", "Email password of the sender, for authentication in Gmail")
	flag.StringVar(&recipient, "r", "", "Email address of the recipient")
	flag.DurationVar(&tickerTime, "t", 600e9, "Ticker interval")
}

type Record struct {
	SendDate      string
	ReferenceDate string
	Company       string
	Subject       string
	Protocol      string
}

func connect() (*mgo.Session, error) {
	return mgo.Dial("localhost:27017")
}

func toUTF8(input []byte) string {
	result := make([]rune, len(input))
	for i, b := range input {
		result[i] = rune(b)
	}
	return string(result)
}

func pageRecords(page int) []Record {
	today := time.Now().Format("02/01/2006")
	url := listURL + strconv.Itoa(page)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("ERROR: %s", err)
		return nil
	}
	defer resp.Body.Close()
	records := make([]Record, 0, 6)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: %s", err)
		return nil
	}
	strBody := strings.Replace(toUTF8(body), "HTML", "html", -1)
	strBody = strings.Replace(strBody, "<<", "", -1)
	strBody = strings.Replace(strBody, ">>", "", -1)
	reader := strings.NewReader(strBody)
	root, err := xmlpath.ParseHTML(reader)
	if err != nil {
		log.Printf("ERROR: %s", err)
		return nil
	}
	trs := pathTR.Iter(root)
	trs.Next()
	for trs.Next() {
		tr := trs.Node()
		sendDate, ok := pathSend.String(tr)
		if !ok {
			continue
		}
		referenceDate, ok := pathReference.String(tr)
		if !ok {
			continue
		}
		company, ok := pathLink.String(tr)
		if !ok {
			continue
		}
		subject, ok := pathSubject.String(tr)
		if !ok {
			continue
		}
		subject = strings.TrimSpace(subject)
		subject = strings.TrimSpace(strings.Split(subject, "\n")[1])
		href, ok := pathLinkHref.String(tr)
		if !ok {
			continue
		}
		parts := regexpLink.FindStringSubmatch(href)
		if len(parts) < 2 {
			continue
		}
		protocol := parts[1]
		record := Record{
			SendDate: sendDate, ReferenceDate: referenceDate,
			Company: company, Subject: subject,
			Protocol: protocol,
		}
		records = append(records, record)
	}
	if length := len(records); length > 0 && records[length-1].ReferenceDate == today {
		nextPageRecords := pageRecords(page + 1)
		for _, record := range nextPageRecords {
			records = append(records, record)
		}
	}
	return records
}

func getRecords() []Record {
	pageRecords := pageRecords(1)
	if len(pageRecords) < 1 {
		return nil
	}
	session, err := connect()
	if err != nil {
		log.Printf("ERROR: %s", err)
		return nil
	}
	defer session.Close()
	query := buildQuery(pageRecords)
	collection := session.DB(dbName).C(collName)
	count, err := collection.Find(query).Count()
	if err != nil {
		log.Printf("ERROR: %s", err)
		return nil
	}
	records := pageRecords[:len(pageRecords)-count]
	for _, record := range records {
		collection.Insert(record)
	}
	return records
}

func buildQuery(records []Record) bson.M {
	sendDates := make([]string, len(records))
	for i, record := range records {
		sendDates[i] = record.SendDate
	}
	return bson.M{"senddate": bson.M{"$in": sendDates}}
}

func poolPage(ticker <-chan time.Time) {
	for _ = range ticker {
		records := getRecords()
		log.Printf("INFO: %d new record(s)", len(records))
		if len(records) > 0 {
			go sendRecords(records)
		}
	}
}

func sendRecords(records []Record) {
	var wg sync.WaitGroup
	for _, record := range records {
		wg.Add(1)
		go func(record Record) {
			defer wg.Done()
			var body bytes.Buffer
			emailTemplate.Execute(&body, map[string]string{
				"company":       record.Company,
				"subject":       record.Subject,
				"sendDate":      record.SendDate,
				"referenceDate": record.ReferenceDate,
				"recipient":     recipient,
				"sender":        sender,
				"link":          protocolURL + record.Protocol,
			})
			auth := smtp.PlainAuth("", sender, password, "smtp.gmail.com")
			err := smtp.SendMail("smtp.gmail.com:587", auth, sender, []string{recipient}, body.Bytes())
			if err != nil {
				log.Printf("ERROR: %s", err)
			}
		}(record)
	}
	wg.Wait()
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
	if failures == 0 {
		poolPage(time.Tick(tickerTime))
	}
}
