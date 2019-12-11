package models

type CurrencyPair struct {
	Trading    string `json:"trading"`
	Settlement string `json:"settlement"`
}

type Asset struct {
	Name   string // "Bitcoin"
	Symbol string // "BTC"
}
