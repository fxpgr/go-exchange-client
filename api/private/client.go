package private

import (
	"github.com/fxpgr/go-ccex-api-client/models"
	"github.com/pkg/errors"
	"strings"
)

//go:generate mockery -name=PrivateClient
type PrivateClient interface {
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

func NewClient(exchangeName string, apikey string, seckey string) (PrivateClient, error) {
	switch strings.ToLower(exchangeName) {
	case "bitflyer":
		return NewBitflyerPrivateApi(apikey, seckey)
	case "poloniex":
		return NewPoloniexApi(apikey, seckey)
	}
	return nil, errors.New("failed to init exchange api")
}
