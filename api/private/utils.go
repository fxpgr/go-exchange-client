package private

import (
	"github.com/pkg/errors"
	"strings"
)

func parseCurrencyPair(s string) (string, string, error) {
	xs := strings.Split(s, "_")

	if len(xs) != 2 {
		return "", "", errors.New("invalid ticker title")
	}
	return xs[0], xs[1], nil
}

type errorResponse struct {
	Error *string `json:"error"`
}

type openOrder struct {
	OrderNumber string `json:"orderNumber"`
	Type        string `json:"type"`
	Rate        string `json:"rate"`
	Amount      string `json:"amount"`
	Total       string `json:"total"`
}

type orderRespnose struct {
	OrderNumber string `json:"orderNumber,string"`
}

type transferResponse struct {
	Response string `json:"response"`
}
type cancelOrderResponse struct {
	Success int `json:"success"`
}
