// Copyright 2014 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This bot extract informations about FIIs (REITs) and write it to a JSON
// file. Users may configure a web server to serve the JSON file.
package main

import (
	"encoding/json"
	"flag"
	"github.com/PuerkitoBio/goquery"
	"log"
	"os"
	"strconv"
	"strings"
)

const url = "http://cvmweb.cvm.gov.br/SWB/Sistemas/SCW/CPublica/ListaPLFII/CPublicaListaPLFII.aspx"

var outputFile string

func init() {
	flag.StringVar(&outputFile, "o", "fii.json", "Output file")
}

type FII struct {
	Name   string  `json:"name"`
	Shares int     `json:"shares"`
	Value  float64 `json:"value"`
}

func collectFIIs() []FII {
	document, err := goquery.NewDocument(url)
	if err != nil {
		log.Printf("Failed to get information from CVM: %s", err)
		return nil
	}
	var fiis []FII
	document.Find("table tr").Not(":last-child").Not(":first-child").Each(func(i int, line *goquery.Selection) {
		tds := line.Find("td")
		name := tds.Eq(1).Text()
		sharesStr := tds.Eq(5).Text()
		sharesStr = strings.Replace(sharesStr, ".", "", -1)
		shares, err := strconv.Atoi(strings.TrimSpace(sharesStr))
		if err != nil {
			log.Printf("Failed to get shares for %q: %s", name, err)
			return
		}
		valueStr := tds.Eq(4).Text()
		valueStr = strings.Replace(valueStr, ".", "", -1)
		valueStr = strings.Replace(valueStr, ",", ".", 1)
		value, err := strconv.ParseFloat(strings.TrimSpace(valueStr), 64)
		if err != nil {
			log.Printf("Failed to get value for %q: %s", name, err)
			return
		}
		fii := FII{
			Name:   strings.TrimSpace(name),
			Shares: shares,
			Value:  value,
		}
		fiis = append(fiis, fii)
	})
	return fiis
}

func main() {
	flag.Parse()
	fiis := collectFIIs()
	file, err := os.Create(outputFile)
	if err != nil {
		log.Printf("Failed to open %q: %s", outputFile, err)
		os.Exit(1)
	}
	defer file.Close()
	err = json.NewEncoder(file).Encode(fiis)
	if err != nil {
		log.Printf("Failed to write the file: %s", err)
		os.Exit(1)
	}
}
