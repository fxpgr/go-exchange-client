package private

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonholmquist/jason"
	"github.com/davecgh/go-spew/spew"
	"github.com/fxpgr/go-ccex-api-client/logger"
	"github.com/fxpgr/go-ccex-api-client/models"
	"github.com/pkg/errors"
)

const (
	POLONIEX_BASE_URL = "https://poloniex.com"
)

type PoloniexApiConfig struct {
	Apikey    string
	ApiSecret string
	BaseURL   string

	RateCacheDuration time.Duration
}

func NewPoloniexApiUsingConfigFunc(f func(*PoloniexApiConfig)) (*PoloniexApi, error) {
	conf := &PoloniexApiConfig{
		BaseURL:           POLONIEX_BASE_URL,
		RateCacheDuration: 30 * time.Second,
	}
	f(conf)

	api := &PoloniexApi{
		rateMap:         nil,
		volumeMap:       nil,
		rateLastUpdated: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),

		m: new(sync.Mutex),
		c: conf,
	}
	return api, nil
}

func NewPoloniexApi(apikey string, apisecret string) (*PoloniexApi, error) {
	return NewPoloniexApiUsingConfigFunc(func(c *PoloniexApiConfig) {
		c.Apikey = apikey
		c.ApiSecret = apisecret
	})
}

func parseCurrencyPair(s string) (string, string, error) {
	xs := strings.Split(s, "_")

	if len(xs) != 2 {
		return "", "", errors.New("invalid ticker title")
	}
	return xs[0], xs[1], nil
}

type PoloniexApi struct {
	volumeMap       map[string]map[string]float64
	rateMap         map[string]map[string]float64
	rateLastUpdated time.Time

	m *sync.Mutex
	c *PoloniexApiConfig
}

func (p *PoloniexApi) privateApiUrl() string {
	return p.c.BaseURL
}

type errorResponse struct {
	Error *string `json:"error"`
}

func (p *PoloniexApi) privateApi(command string, args map[string]string) ([]byte, error) {
	cli := &http.Client{}

	val := url.Values{}
	val.Add("command", command)
	val.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	if args != nil {
		for k, v := range args {
			val.Add(k, v)
		}
	}

	reader := bytes.NewReader([]byte(val.Encode()))
	req, err := http.NewRequest("POST", p.privateApiUrl(), reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", command)
	}

	mac := hmac.New(sha512.New, []byte(p.c.ApiSecret))
	_, err = mac.Write([]byte(val.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to encrypt request")
	}
	sign := mac.Sum(nil)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Key", p.c.Apikey)
	req.Header.Add("Sign", hex.EncodeToString(sign))

	res, err := cli.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to request command %s", command)
	}
	defer res.Body.Close()

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch result of command %s", command)
	}

	logger.Get().Infof("[poloniex] private api called: cmd=%s req=%s, res=%.60s", command, spew.Sdump(args), string(resBody))

	var errres errorResponse
	err = json.Unmarshal(resBody, &errres)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse response")
	}
	if errres.Error != nil {
		return nil, errors.Errorf("server returns error '%s'", *errres.Error)
	}

	return resBody, nil
}

type poloniexFeeRate struct {
	MakerFee float64 `json:"makerFee"`
	TakerFee float64 `json:"takerFee"`
}

func (p *PoloniexApi) fetchFeeRate() (float64, error) {
	var fee poloniexFeeRate

	bs, err := p.privateApi("returnFeeInfo", nil)
	if err != nil {
		return 0, err
	}

	err = json.Unmarshal(bs, &fee)
	if err != nil {
		return 0, err
	}

	return fee.TakerFee, nil
}

func (p *PoloniexApi) PurchaseFeeRate() (float64, error) {
	return p.fetchFeeRate()
}

func (p *PoloniexApi) SellFeeRate() (float64, error) {
	return p.fetchFeeRate()
}

func (p *PoloniexApi) TransferFee() (map[string]float64, error) {
	return nil, nil
}

func (p *PoloniexApi) Balances() (map[string]float64, error) {
	bs, err := p.privateApi("returnBalances", nil)
	if err != nil {
		return nil, err
	}

	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse json")
	}

	m := make(map[string]float64)
	jsonMap := json.Map()
	for k, v := range jsonMap {
		if err != nil {
			continue
		}
		balanceStr, err := v.String()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse %v as string", v)
		}
		balance, err := strconv.ParseFloat(balanceStr, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s as float", balanceStr)
		}
		m[k] = balance
	}

	return m, nil
}

func (p *PoloniexApi) CompleteBalances() (map[string]*models.Balance, error) {
	bs, err := p.privateApi("returnCompleteBalances", nil)
	if err != nil {
		return nil, err
	}

	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse json")
	}

	m := make(map[string]*models.Balance)
	jsonMap := json.Map()
	for k, v := range jsonMap {
		balanceObj, err := v.Object()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse %v as object", v)
		}
		balanceMap := balanceObj.Map()

		availableStr, err := balanceMap["available"].String()
		if err != nil {
			return nil, errors.Wrap(err, "couldn't get available as string")
		}
		available, err := strconv.ParseFloat(availableStr, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't parse available(%s) as float64", availableStr)
		}

		onOrdersStr, err := balanceMap["onOrders"].String()
		if err != nil {
			return nil, errors.Wrap(err, "couldn't get onOrders as string")
		}
		onOrders, err := strconv.ParseFloat(onOrdersStr, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't parse onOrders(%s) as float64", onOrdersStr)
		}

		b := models.NewBalance(available, onOrders)
		m[k] = b
	}

	return m, nil
}

type openOrder struct {
	OrderNumber string `json:"orderNumber"`
	Type        string `json:"type"`
	Rate        string `json:"rate"`
	Amount      string `json:"amount"`
	Total       string `json:"total"`
}

func (p *PoloniexApi) ActiveOrders() ([]*models.Order, error) {
	bs, err := p.privateApi("returnOpenOrders", map[string]string{
		"currencyPair": "all",
	})
	if err != nil {
		return nil, err
	}

	var m map[string][]openOrder
	if err := json.Unmarshal(bs, &m); err != nil {
		return nil, err
	}

	var orders []*models.Order
	for pair, os := range m {
		if len(os) == 0 {
			continue
		}

		trading, settlement, err := parseCurrencyPair(pair)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse currency pair: %s", pair)
		}

		for _, o := range os {
			_, err := strconv.ParseInt(o.OrderNumber, 10, 64)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot parse ordernumber: %s", o.OrderNumber)
			}

			rate, err := strconv.ParseFloat(o.Rate, 64)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot parse rate: %s", o.Rate)
			}

			amount, err := strconv.ParseFloat(o.Amount, 64)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot parse amount: %s", o.Amount)
			}

			var orderType models.OrderType
			switch o.Type {
			case "sell":
				orderType = models.Ask
			case "buy":
				orderType = models.Bid
			default:
				return nil, errors.Errorf("unknown order type: %s", o.Type)
			}

			c := &models.Order{
				ExchangeOrderID: o.OrderNumber,
				Type:            orderType,
				Trading:         trading,
				Settlement:      settlement,
				Price:           rate,
				Amount:          amount,
			}
			orders = append(orders, c)
		}
	}
	return orders, nil
}

type orderRespnose struct {
	OrderNumber string `json:"orderNumber,string"`
}

func (p *PoloniexApi) Order(trading string, settlement string, ordertype models.OrderType, price float64, amount float64) (string, error) {
	var cmd string
	if ordertype == models.Ask {
		cmd = "buy"
	} else if ordertype == models.Bid {
		cmd = "sell"
	} else {
		return "", errors.Errorf("unknown order type %d", ordertype)
	}

	pair := fmt.Sprintf("%s_%s", settlement, trading)

	args := make(map[string]string)
	args["currencyPair"] = pair
	args["rate"] = strconv.FormatFloat(price, 'g', -1, 64)
	args["amount"] = strconv.FormatFloat(amount, 'g', -1, 64)

	bs, err := p.privateApi(cmd, args)
	if err != nil {
		return "", errors.Wrap(err, "failed to request order")
	}
	var res orderRespnose
	err = json.Unmarshal(bs, &res)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse response json %s", string(bs))
	}
	orderNumberInt, err := strconv.Atoi(res.OrderNumber)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse response json %s", string(bs))
	}
	if orderNumberInt <= 0 {
		return "", errors.Errorf("invalid order number %d", res.OrderNumber)
	}

	return res.OrderNumber, nil
}

type transferResponse struct {
	Response string `json:"response"`
}

func (p *PoloniexApi) Transfer(typ string, addr string, amount float64, additionalFee float64) error {
	args := make(map[string]string)
	args["address"] = addr
	args["currency"] = typ
	args["amount"] = strconv.FormatFloat(amount, 'g', -1, 64)

	bs, err := p.privateApi("withdraw", args)
	if err != nil {
		return errors.Wrap(err, "failed to transfer deposit")
	}
	var res transferResponse
	err = json.Unmarshal(bs, res)
	if err != nil {
		return errors.Wrapf(err, "failed to parse response json %s", string(bs))
	}
	if res.Response == "" {
		return errors.Errorf("invalid response %s", string(bs))
	}

	return nil
}

type cancelOrderResponse struct {
	Success int `json:"success"`
}

func (p *PoloniexApi) CancelOrder(orderNumber string, _ string) error {
	args := make(map[string]string)
	args["orderNumber"] = orderNumber

	bs, err := p.privateApi("cancelOrder", args)
	if err != nil {
		return errors.Wrapf(err, "failed to cancel order")
	}

	var res cancelOrderResponse
	if err := json.Unmarshal(bs, &res); err != nil {
		return errors.Wrapf(err, "failed to parse response json")
	}

	if res.Success != 1 {
		return errors.New("cancel order failed")
	}

	return nil
}

func (p *PoloniexApi) Address(c string) (string, error) {
	bs, err := p.privateApi("returnDepositAddresses", nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch deposit address")
	}

	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse json")
	}

	jsonMap := json.Map()
	addr, err := jsonMap[c].String()
	if err != nil {
		return "", errors.Wrapf(err, "failed to take address of %s", c)
	}

	return addr, nil
}
