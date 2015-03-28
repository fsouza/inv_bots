package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
)

const URL = "http://www.bmfbovespa.com.br/Pregao-Online/ExecutaAcaoAjax.asp?CodigoPapel="

type ComportamentoPapeis struct {
	XMLName xml.Name
	Papeis  []Papel `xml:"Papel"`
}

type Papel struct {
	XMLName xml.Name
	Codigo  string `xml:",attr"`
	Ultimo  string `xml:",attr"`
}

func main() {
	flag.Parse()
	simbolo := flag.Arg(0)
	resp, err := http.Get(URL + simbolo)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	var papeis ComportamentoPapeis
	xml.Unmarshal(content, &papeis)
	fmt.Println(papeis.Papeis[0].Ultimo)
}
