package private

import (
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"strings"
)

type TradeFee struct {
	MakerFee float64
	TakerFee float64
}

//go:generate mockery -name=PrivateClient
type PrivateClient interface {
	TransferFee() (map[string]float64, error)
	TradeFeeRate() (map[string]map[string]TradeFee, error)
	Balances() (map[string]float64, error)
	CompleteBalances() (map[string]*models.Balance, error)
	ActiveOrders() ([]*models.Order, error)
	IsOrderFilled(string, string) (bool, error)
	Order(trading string, settlement string,
		ordertype models.OrderType, price float64, amount float64) (string, error)
	CancelOrder(orderNumber string, productCode string) error
	//FilledOrderInfo(orderNumber string) (models.FilledOrderInfo,error)
	Transfer(typ string, addr string,
		amount float64, additionalFee float64) error
	Address(c string) (string, error)
}

func NewClient(mode ClientMode, exchangeName string, apikey string, seckey string) (PrivateClient, error) {
	switch strings.ToLower(exchangeName) {
	case "bitflyer":
		return NewBitflyerPrivateApi( apikey, seckey)
	case "poloniex":
		return NewPoloniexApi( apikey, seckey)
	case "hitbtc":
		return NewHitbtcApi( apikey, seckey)
	case "huobi":
		return NewHuobiApi( apikey, seckey)
	}
	return nil, errors.New("failed to init exchange api")
}
