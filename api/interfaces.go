package api

import (
	"github.com/fxpgr/go-ccex-api-client/api/private"
	"github.com/fxpgr/go-ccex-api-client/models"
	"github.com/pkg/errors"
	"strings"
)

//go:generate mockery -name=ExchangePrivateRepository
type ExchangePrivateRepository interface {
	PurchaseFeeRate() (float64, error)
	SellFeeRate() (float64, error)
	TransferFee() (map[string]float64, error)
	Balances() (map[string]float64, error)
	CompleteBalances() (map[string]*models.Balance, error)
	ActiveOrders() ([]*models.Order, error)
	Order(trading string, settlement string,
		ordertype models.OrderType, price float64, amount float64) (string, error)
	Transfer(typ string, addr string,
		amount float64, additionalFee float64) error
	CancelOrder(orderNumber string, productCode string) error
	Address(c string) (string, error)
}

func NewExchangePrivateRepository(exchangeName string, apikey string, seckey string) (ExchangePrivateRepository, error) {
	switch strings.ToLower(exchangeName) {
	case "bitflyer":
		return private.NewBitflyerPrivateApi(apikey, seckey)
	case "poloniex":
		return private.NewPoloniexApi(apikey, seckey)
	}
	return nil, errors.New("failed to init exchange api")
}
