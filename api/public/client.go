package public

import (
	"strings"
	"github.com/fxpgr/go-ccex-api-client/models"
	"errors"
)

//go:generate mockery -name=ExchangePublicRepository
type ExchangePublicRepository interface {
	Volume(trading string, settlement string) (float64, error)
	CurrencyPairs() ([]*models.CurrencyPair, error)
	Rate(trading string, settlement string) (float64, error)
}

func NewExchangePublicRepository(exchangeName string) (ExchangePublicRepository, error) {
	switch strings.ToLower(exchangeName) {
	case "bitflyer":
		return NewBitflyerPublicApi()
	case "poloniex":
		return NewPoloniexPublicApi()
	case "hitbtc":
		return NewHitbtcPublicApi()
	}
	return nil, errors.New("failed to init exchange api")
}
