package private

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"strconv"
	"strings"
)

const (
	HITBTC_BASE_URL = "https://api.hitbtc.com"
)

func NewHitbtcApi(apikey string, apisecret string) (*HitbtcApi, error) {
	hitbtcPublic, err := public.NewHitbtcPublicApi()
	if err != nil {
		return nil, err
	}
	pairs, err := hitbtcPublic.CurrencyPairs()
	if err != nil {
		return nil, err
	}
	var settlements []string
	for _, v := range pairs {
		settlements = append(settlements, v.Settlement)
	}
	m := make(map[string]bool)
	uniq := []string{}
	for _, ele := range settlements {
		if !m[ele] {
			m[ele] = true
			uniq = append(uniq, ele)
		}
	}

	return &HitbtcApi{
		BaseURL:           HITBTC_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		ApiKey:            apikey,
		SecretKey:         apisecret,
		settlements:       uniq,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),

		m: new(sync.Mutex),
	}, nil
}

type HitbtcApi struct {
	ApiKey            string
	SecretKey         string
	BaseURL           string
	RateCacheDuration time.Duration
	HttpClient        http.Client
	settlements       []string

	volumeMap       map[string]map[string]float64
	rateMap         map[string]map[string]float64
	rateLastUpdated time.Time

	m *sync.Mutex
}

func (h *HitbtcApi) privateApiUrl() string {
	return h.BaseURL
}

func (h *HitbtcApi) privateApi(method string, path string, args map[string]string) ([]byte, error) {

	val := url.Values{}
	if args != nil {
		for k, v := range args {
			val.Add(k, v)
		}
	}

	reader := bytes.NewReader([]byte(val.Encode()))
	req, err := http.NewRequest(method, h.privateApiUrl()+path, reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	req.SetBasicAuth(h.ApiKey, h.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := h.HttpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to request command %s", path)
	}
	defer res.Body.Close()

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch result of command %s", path)
	}
	return resBody, nil
}

func (h *HitbtcApi) TradeFeeRates() (map[string]map[string]TradeFee, error) {
	purchaseFeeurl := "/api/2/public/symbol"
	method := "GET"
	resBody, err := h.privateApi(method, purchaseFeeurl, map[string]string{})
	if err != nil {
		return nil, err
	}
	json, err := gabs.ParseJSON(resBody)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	symbolMap, err := json.Children()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	traderFeeMap := make(map[string]map[string]TradeFee)
	for _, v := range symbolMap {
		takeLiquidityRateStr, ok := v.Path("takeLiquidityRate").Data().(string)
		if !ok {
			continue
		}
		takeLiquidityRate, err := strconv.ParseFloat(takeLiquidityRateStr, 10)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse json")
		}
		provideLiquidityRateStr, ok := v.Path("provideLiquidityRate").Data().(string)
		if !ok {
			continue
		}
		provideLiquidityRate, err := strconv.ParseFloat(provideLiquidityRateStr, 10)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse json")
		}
		baseCurrency, ok := v.Path("baseCurrency").Data().(string)
		if !ok {
			continue
		}
		quoteCurrency, ok := v.Path("quoteCurrency").Data().(string)
		if !ok {
			continue
		}
		n := make(map[string]TradeFee)
		n[quoteCurrency] = TradeFee{
			TakerFee: takeLiquidityRate,
			MakerFee: provideLiquidityRate,
		}
		traderFeeMap[baseCurrency] = n
	}
	return traderFeeMap, nil
}

func (b *HitbtcApi) TradeFeeRate(trading string, settlement string) (TradeFee, error) {
	feeMap, err := b.TradeFeeRates()
	if err != nil {
		return TradeFee{}, err
	}
	return feeMap[trading][settlement], nil
}

func (h *HitbtcApi) TransferFee() (map[string]float64, error) {
	purchaseFeeurl := "/api/2/public/currency"
	method := "GET"
	resBody, err := h.privateApi(method, purchaseFeeurl, map[string]string{})
	if err != nil {
		return nil, err
	}
	json, err := gabs.ParseJSON(resBody)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json: %v", string(resBody))
	}
	fmt.Println(json)
	currencyMap, err := json.Children()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json: %v", string(resBody))
	}
	transferFeeMap := make(map[string]float64)
	for _, v := range currencyMap {
		payoutFeeStr, ok := v.Path("payoutFee").Data().(string)
		if !ok {
			continue
		}
		payoutFee, err := strconv.ParseFloat(payoutFeeStr, 10)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse json: %v", string(resBody))
		}
		currency, ok := v.Path("id").Data().(string)
		if !ok {
			continue
		}
		transferFeeMap[currency] = payoutFee
	}
	return transferFeeMap, nil
}

func (h *HitbtcApi) Balances() (map[string]float64, error) {
	bs, err := h.privateApi("GET", "/api/2/trading/balance", nil)
	if err != nil {
		return nil, err
	}
	json, err := gabs.ParseJSON(bs)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	rateMap, err := json.Children()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	m := make(map[string]float64)
	for _, v := range rateMap {
		currency, ok := v.Path("currency").Data().(string)
		if !ok {
			continue
		}
		availableStr, ok := v.Path("available").Data().(string)
		if !ok {
			continue
		}
		available, err := strconv.ParseFloat(availableStr, 10)
		if err != nil {
			return nil, err
		}
		m[currency] = available
	}
	return m, nil
}

func (h *HitbtcApi) CompleteBalances() (map[string]*models.Balance, error) {
	bs, err := h.privateApi("GET", "/api/2/trading/balance", nil)
	if err != nil {
		return nil, err
	}
	json, err := gabs.ParseJSON(bs)

	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}

	rateMap, err := json.Children()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	m := make(map[string]*models.Balance)
	for _, v := range rateMap {
		availableStr, ok := v.Path("available").Data().(string)
		if !ok {
			continue
		}
		available, err := strconv.ParseFloat(availableStr, 10)
		if err != nil {
			return nil, err
		}
		reservedStr, ok := v.Path("reserved").Data().(string)
		if !ok {
			continue
		}
		reserved, err := strconv.ParseFloat(reservedStr, 10)
		if err != nil {
			return nil, err
		}
		currency, ok := v.Path("currency").Data().(string)
		if !ok {
			continue
		}
		balance := models.NewBalance(available, reserved)
		m[currency] = balance
	}
	return m, nil
}

func (h *HitbtcApi) CompleteBalance(coin string) (*models.Balance, error) {
	bs, err := h.privateApi("GET", "/api/2/trading/balance", nil)
	if err != nil {
		return nil, err
	}
	json, err := gabs.ParseJSON(bs)

	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}

	rateMap, err := json.Children()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	m := make(map[string]*models.Balance)
	for _, v := range rateMap {
		currency, ok := v.Path("currency").Data().(string)
		if !ok {
			continue
		}
		if currency != coin {
			continue
		}
		availableStr, ok := v.Path("available").Data().(string)
		if !ok {
			continue
		}
		available, err := strconv.ParseFloat(availableStr, 10)
		if err != nil {
			return nil, err
		}
		reservedStr, ok := v.Path("reserved").Data().(string)
		if !ok {
			continue
		}
		reserved, err := strconv.ParseFloat(reservedStr, 10)
		if err != nil {
			return nil, err
		}
		balance := models.NewBalance(available, reserved)
		m[currency] = balance
	}
	return m[coin], nil
}

func (h *HitbtcApi) IsOrderFilled(trading string, settlement string, orderNumber string) (bool, error) {
	orders, err := h.ActiveOrders()
	if err != nil {
		return false, errors.Wrapf(err, "failed to get active orders")
	}
	for _, v := range orders {
		if orderNumber == v.ExchangeOrderID {
			return false, nil
		}
	}
	return true, nil
}

func (h *HitbtcApi) ActiveOrders() ([]*models.Order, error) {
	bs, err := h.privateApi("GET", "/api/2/order", map[string]string{})
	if err != nil {
		return nil, err
	}
	json, err := gabs.ParseJSON(bs)
	m, err := json.Children()
	if err != nil {
		return nil, err
	}
	var orders []*models.Order
	for _, v := range m {
		orderId := v.Path("clientOrderId").Data().(string)
		if err != nil {
			continue
		}
		symbol := v.Path("symbol").Data().(string)
		if err != nil {
			continue
		}
		quantityStr := v.Path("quantity").Data().(string)
		if err != nil {
			continue
		}
		quantity, err := strconv.ParseFloat(quantityStr, 10)
		if err != nil {
			return nil, err
		}
		priceStr := v.Path("price").Data().(string)
		if err != nil {
			continue
		}
		price, err := strconv.ParseFloat(priceStr, 10)
		if err != nil {
			return nil, err
		}
		side := v.Path("side").Data().(string)
		if err != nil {
			continue
		}
		orderType := models.Bid
		if side == "buy" {
			orderType = models.Ask
		}
		var settlement string
		var trading string
		for _, s := range h.settlements {
			index := strings.LastIndex(symbol, s)
			if index != 0 && index == len(symbol)-len(s) {
				settlement = s
				trading = symbol[0:index]
			}
		}
		if settlement == "" || trading == "" {
			continue
		}
		c := &models.Order{
			ExchangeOrderID: orderId,
			Type:            orderType,
			Trading:         trading,
			Settlement:      settlement,
			Price:           price,
			Amount:          quantity,
		}
		orders = append(orders, c)
	}
	return orders, nil
}

func (h *HitbtcApi) Order(trading string, settlement string, ordertype models.OrderType, price float64, amount float64) (string, error) {
	var cmd string
	if ordertype == models.Ask {
		cmd = "buy"
	} else if ordertype == models.Bid {
		cmd = "sell"
	} else {
		return "", errors.Errorf("unknown order type %d", ordertype)
	}
	pair := strings.ToUpper(fmt.Sprintf("%s%s", trading, settlement))
	args := make(map[string]string)
	args["side"] = cmd
	args["symbol"] = pair
	args["price"] = strconv.FormatFloat(price, 'g', -1, 64)
	args["quantity"] = strconv.FormatFloat(amount, 'g', -1, 64)
	bs, err := h.privateApi("POST", "/api/2/order", args)
	if err != nil {
		return "", errors.Wrap(err, "failed to request order")
	}

	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse response json %s", string(bs))
	}

	orderNumber, err := json.GetString("clientOrderId")
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse order number int %s", string(bs))
	}
	return orderNumber, nil
}

func (h *HitbtcApi) Transfer(typ string, addr string, amount float64, additionalFee float64) error {
	args := make(map[string]string)
	args["currency"] = typ
	args["amount"] = strconv.FormatFloat(amount, 'g', -1, 64)
	args["type"] = "exchangeToBank"
	bs, err := h.privateApi("POST", "/api/2/account/transfer", args)
	if err != nil {
		return errors.Wrap(err, "failed to transfer deposit")
	}
	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return errors.Wrapf(err, "failed to parse response json %s", string(bs))
	}
	_, err = json.GetString("id")
	if err != nil {
		return errors.Wrapf(err, "failed to parse response json %s", string(bs))
	}

	args = make(map[string]string)
	args["address"] = addr
	args["currency"] = typ
	args["amount"] = strconv.FormatFloat(amount, 'g', -1, 64)
	args["networkFee"] = strconv.FormatFloat(additionalFee, 'g', -1, 64)

	bs, err = h.privateApi("POST", "/api/2/account/crypto/withdraw", args)
	if err != nil {
		return errors.Wrap(err, "failed to transfer deposit")
	}
	json, err = jason.NewObjectFromBytes(bs)
	if err != nil {
		return errors.Wrapf(err, "failed to parse response json %s", string(bs))
	}
	_, err = json.GetString("id")
	if err != nil {
		return errors.Wrapf(err, "failed to parse response json %s", string(bs))
	}
	return err
}

func (h *HitbtcApi) CancelOrder(trading string, settlement string,
	ordertype models.OrderType, orderNumber string) error {
	args := make(map[string]string)
	_, err := h.privateApi("DELETE", "/api/2/order/"+orderNumber, args)
	if err != nil {
		return errors.Wrapf(err, "failed to cancel order")
	}
	return nil
}

func (h *HitbtcApi) Address(c string) (string, error) {
	bs, err := h.privateApi("GET", "/api/2/account/crypto/address/"+c, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch deposit address")
	}
	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse json")
	}
	address, err := json.GetString("address")
	if err != nil {
		return "", errors.Wrapf(err, "failed to take address of %s", c)
	}
	return address, nil
}
