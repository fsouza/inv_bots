// Copyright 2014 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a function that you can use on Google Spreadsheets to get the
// current price of the given bond.
//
// For example, if the cell A1 contains the bond "NTN-B Principal", and you
// want to display its current price in the cell B1, you can use the following
// formula in the cell B1:
//
//     =TESOURODIRETO(A1)
//
// It assumes you're using the server at http://td.souza.cc. If you're
// hosting it somewhere else, keep in mind that you will need to change the
// URL.
//
// The code for the server is available in the file tesouro_direto.go.
function tesourodireto(nome) {
	var response = UrlFetchApp.fetch("http://td.souza.cc");
	var data = JSON.parse(response.getContentText());
	for(var i = 0; i < data.length; i++) {
		if(data[i].Titulo == nome) {
			return data[i].PrecoVenda > 0 ? data[i].PrecoVenda : data[i].PrecoCompra;
		}
	}
	return 0;
}
