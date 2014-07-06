// Copyright 2014 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This bot collects current interest rate for CDI, from Cetip's home page, and
// provides an HTTP server for serving the current annual and daily rate.
package main

import (
	"code.google.com/p/cascadia"
	"code.google.com/p/go.net/html"
	"encoding/json"
	"flag"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AnnualInterest struct {
	Year float64
	Day  float64
}

func (i *AnnualInterest) CalculateDay() {
	i.Day = (math.Pow(1+i.Year/100, 1.0/252.0) - 1) * 100
}

func (i *AnnualInterest) CalculateYear() {
	i.Year = (math.Pow(1+i.Day/100, 252.0) - 1) * 100
}

func (i *AnnualInterest) Equiv(value float64) AnnualInterest {
	var equiv AnnualInterest
	equiv.Day = value * i.Day
	equiv.CalculateYear()
	return equiv
}

var cdi AnnualInterest

var (
	interval time.Duration
	bind     string
)

func init() {
	flag.DurationVar(&interval, "interval", time.Hour, "Interval between updates in the value")
	flag.StringVar(&bind, "bind", "0.0.0.0:5555", "Address to bind")
}

func collectCDI() {
	resp, err := http.Get("http://www.cetip.com.br/Home")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	selector := cascadia.MustCompile("#ctl00_Banner_lblTaxDI")
	root, err := html.Parse(resp.Body)
	if err != nil {
		panic(err)
	}
	node := selector.MatchFirst(root)
	interest := strings.TrimRight(node.FirstChild.Data, "%")
	interest = strings.Replace(interest, ",", ".", 1)
	if value, err := strconv.ParseFloat(interest, 64); err == nil {
		cdi.Year = value
		cdi.CalculateDay()
	}
}

func main() {
	flag.Parse()
	collectCDI()
	go func() {
		for _ = range time.Tick(interval) {
			collectCDI()
		}
	}()
	err := http.ListenAndServe(bind, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cdi)
	}))
	if err != nil {
		panic(err)
	}
}
