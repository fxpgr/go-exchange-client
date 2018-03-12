package main

import (
	"fmt"
	"github.com/fxpgr/go-ccex-api-client/api/private"
)

func main() {
	cli, err := private.NewClient("huobi","0f6df47e-f968f864-481be14e-b98ac","d59d90fd-067711c2-9e65ea61-fe710")
	if err != nil {
		panic(err)
	}
	fmt.Println(cli.Balances())
}
