package models

type CurrencyPair struct {
	Trading    string `json:"trading,string"`
	Settlement string `json:"settlement,string"`
}
