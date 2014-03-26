// Copyright 2014 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a bot and a service. The bot runs in the specified intervals and
// collects data about the list of government bonds available at Bovespa.
//
// There's also a webserver that serves it directly from the memory, in JSON
// format.
package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"launchpad.net/xmlpath"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const URL = "http://www.bmfbovespa.com.br/pt-br/mercados/outros-titulos/tesouro-direto/tesouro-direto.aspx?idioma=pt-br"

type Titulo struct {
	Titulo      string
	Vencimento  time.Time
	PrecoCompra float64
	PrecoVenda  float64
	TaxaCompra  string
	TaxaVenda   string
}

var (
	titulos      []Titulo
	listen       string
	interval     time.Duration
	pathTD       = xmlpath.MustCompile(`//table[@summary="Taxas"]/tbody/tr/td[1]`)
	pathSiblings = xmlpath.MustCompile(`./following-sibling::*`)
)

func init() {
	flag.DurationVar(&interval, "interval", time.Minute*60, "Interval")
	flag.StringVar(&listen, "http", ":7575", "Address to listen")
}

func collectTitulos() {
	resp, err := http.Get(URL)
	if err != nil {
		log.Printf("ERROR: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: %s", err)
		return
	}
	strBody := strings.Replace(string(body), " < ", " &lt; ", -1)
	strBody = strings.Replace(strBody, " > ", " &gt; ", -1)
	reader := strings.NewReader(strBody)
	root, err := xmlpath.ParseHTML(reader)
	if err != nil {
		log.Printf("ERROR: %s", err)
		return
	}
	titulos = make([]Titulo, 0, 10)
	tds := pathTD.Iter(root)
	for tds.Next() {
		var titulo Titulo
		titulo.Titulo = tds.Node().String()
		siblings := pathSiblings.Iter(tds.Node())
		for i := 0; siblings.Next(); i++ {
			node := siblings.Node()
			switch i {
			case 0:
				original := node.String()
				vencimento, err := time.Parse("02/01/2006", original)
				if err != nil {
					log.Printf("[ERROR] Failed to parse vencimento: %s", err)
					break
				}
				titulo.Vencimento = vencimento
				titulo.Titulo += " " + vencimento.Format("020106")
			case 2:
				titulo.TaxaCompra = node.String()
			case 3:
				titulo.TaxaVenda = node.String()
			case 4:
				original := strings.Replace(node.String(), ",", ".", -1)
				if original == "-" {
					break
				}
				precoCompra, err := strconv.ParseFloat(original, 64)
				if err != nil {
					log.Printf("[ERROR] Failed to parse precoCompra: %s", err)
					break
				}
				titulo.PrecoCompra = precoCompra
			case 5:
				original := strings.Replace(node.String(), ",", ".", -1)
				if original == "-" {
					break
				}
				precoVenda, err := strconv.ParseFloat(original, 64)
				if err != nil {
					log.Printf("[ERROR] Failed to parse precoVenda: %s", err)
					break
				}
				titulo.PrecoVenda = precoVenda
			}
		}
		titulos = append(titulos, titulo)
	}
}

func tesouroDireto(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(titulos)

}

func collectLoop() {
	for _ = range time.Tick(interval) {
		collectTitulos()
	}
}

func main() {
	flag.Parse()
	collectTitulos()
	go collectLoop()
	http.Handle("/", http.HandlerFunc(tesouroDireto))
	log.Printf("Starting server at %s...\n", listen)
	err := http.ListenAndServe(listen, nil)
	if err != nil {
		log.Printf("ERROR: %s\n", err)
	}
}
