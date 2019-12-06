package main

import (
	"fmt"

	"github.com/fxpgr/go-exchange-client/api/private"
)

func main() {
	pcli, err := private.NewClient(private.PROJECT, "kucoin", func() (string, error) {
		return "test", nil
	}, func() (s string, err error) {
		return "????", nil
	})
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	fmt.Println(pcli.Balances())
}
