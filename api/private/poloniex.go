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
	"sync"
	"time"

	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-exchange-client/logger"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"strings"
)

const (
	POLONIEX_BASE_URL = "https://poloniex.com"
)

func NewPoloniexApi(apikey string, apisecret string) (*PoloniexApi, error) {
	return &PoloniexApi{
		BaseURL:           POLONIEX_BASE_URL,
		RateCacheDuration: 7 * 24 * time.Hour,
		ApiKey:            apikey,
		SecretKey:         apisecret,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),

		m: new(sync.Mutex),
	}, nil
}

type PoloniexApi struct {
	ApiKey            string
	SecretKey         string
	BaseURL           string
	RateCacheDuration time.Duration
	HttpClient        http.Client

	volumeMap       map[string]map[string]float64
	rateMap         map[string]map[string]float64
	rateLastUpdated time.Time

	m *sync.Mutex
}

func parsePoloCurrencyPair(s string) (string, string, error) {
	xs := strings.Split(s, "_")

	if len(xs) != 2 {
		return "", "", errors.New("invalid ticker title")
	}

	return xs[0], xs[1], nil
}

func (p *PoloniexApi) baseUrl() string {
	return p.BaseURL
}

func (p *PoloniexApi) privateApiUrl() string {
	return p.BaseURL
}

func (p *PoloniexApi) privateApi(command string, args map[string]string) ([]byte, error) {
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

	mac := hmac.New(sha512.New, []byte(p.SecretKey))
	_, err = mac.Write([]byte(val.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to encrypt request")
	}
	sign := mac.Sum(nil)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Key", p.ApiKey)
	req.Header.Add("Sign", hex.EncodeToString(sign))

	res, err := p.HttpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to request command %s", command)
	}
	defer res.Body.Close()

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch result of command %s", command)
	}
	return resBody, nil
}

type poloniexFeeRate struct {
	MakerFee float64 `json:"makerFee"`
	TakerFee float64 `json:"takerFee"`
}

func (p *PoloniexApi) fetchRate() error {
	p.rateMap = make(map[string]map[string]float64)
	p.volumeMap = make(map[string]map[string]float64)
	url := p.baseUrl() + "/public?command=returnTicker"
	resp, err := p.HttpClient.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()
	json, err := jason.NewObjectFromReader(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to parse json from reader")
	}

	rateMap := json.Map()
	for k, v := range rateMap {
		settlement, trading, err := parsePoloCurrencyPair(k)
		if err != nil {
			logger.Get().Warn("couldn't parse currency pair", err)
			continue
		}

		obj, err := v.Object()
		if err != nil {
			return err
		}

		// update rate
		last, err := obj.GetString("last")
		if err != nil {
			return err
		}

		lastf, err := strconv.ParseFloat(last, 64)
		if err != nil {
			return err
		}

		m, ok := p.rateMap[trading]
		if !ok {
			m = make(map[string]float64)
			p.rateMap[trading] = m
		}
		m[settlement] = lastf

		// update volume
		volume, err := obj.GetString("baseVolume")
		if err != nil {
			return err
		}

		volumef, err := strconv.ParseFloat(volume, 64)
		if err != nil {
			return err
		}

		m, ok = p.volumeMap[trading]
		if !ok {
			m = make(map[string]float64)
			p.volumeMap[trading] = m
		}
		m[settlement] = volumef
	}
	return nil
}

func (p *PoloniexApi) TradeFeeRates() (map[string]map[string]TradeFee, error) {
	p.m.Lock()
	defer p.m.Unlock()

	now := time.Now()
	if now.Sub(p.rateLastUpdated) >= p.RateCacheDuration {
		err := p.fetchRate()
		if err != nil {
			return nil, errors.Wrap(err,"aa")
		}
		p.rateLastUpdated = now
	}
	bs, err := p.privateApi("returnFeeInfo", nil)
	if err != nil {
		return nil, errors.Wrap(err,"bb")
	}
	fmt.Println(fmt.Sprintf("%s", bs))

	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return nil, errors.Wrap(err,"cc")
	}
	makerFeeString, err := json.GetString("makerFee")
	if err != nil {
		return nil, errors.Wrap(err,"dd")
	}
	makerFee, err := strconv.ParseFloat(makerFeeString, 10)
	if err != nil {
		return nil, err
	}
	takerFeeString, err := json.GetString("takerFee")
	if err != nil {
		return nil, err
	}
	takerFee, err := strconv.ParseFloat(takerFeeString, 10)
	if err != nil {
		return nil, err
	}

	traderFeeMap := make(map[string]map[string]TradeFee)
	for trading, v := range p.rateMap {
		m := make(map[string]TradeFee)
		for settlement, _ := range v {
			m[settlement] = TradeFee{TakerFee: takerFee, MakerFee: makerFee}
		}
		traderFeeMap[trading] = m
	}

	return traderFeeMap, nil
}

func (b *PoloniexApi) TradeFeeRate(trading string, settlement string) (TradeFee, error) {
	feeMap, err := b.TradeFeeRates()
	if err != nil {
		return TradeFee{}, err
	}
	return feeMap[trading][settlement], nil
}

type Currency struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	TxFee          float64 `json:"txFee,string"`
	MinConf        int     `json:"minConf"`
	DepositAddress string  `json:"depositAddress"`
	Disabled       int     `json:"disabled"`
	Delisted       int     `json:"delisted"`
	Frozen         int     `json:"frozen"`
}

func (p *PoloniexApi) TransferFee() (map[string]float64, error) {
	url := p.baseUrl() + "/public?command=returnCurrencies"
	resp, err := p.HttpClient.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()
	transferFeeMap := make(map[string]float64)
	m := make(map[string]Currency)
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, errors.Wrap(err, "failed to parse response")
	}
	for k, v := range m {
		transferFeeMap[k] = v.TxFee
	}
	return transferFeeMap, nil
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

func (p *PoloniexApi) IsOrderFilled(orderNumber string, _ string) (bool, error) {
	orders, err := p.ActiveOrders()
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

		settlement, trading, err := parseCurrencyPair(pair)
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

			orderType := models.Bid
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
	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse response json %s", string(bs))
	}

	orderNumberInt, err := json.GetInt64("orderNumber")
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse order number int %s", string(bs))
	}
	if orderNumberInt <= 0 {
		return "", errors.Errorf("invalid order number %v", orderNumberInt)
	}
	return strconv.Itoa(int(orderNumberInt)), nil
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
	err = json.Unmarshal(bs, &res)
	if err != nil {
		return errors.Wrapf(err, "failed to parse response json %s", string(bs))
	}
	if res.Response == "" {
		return errors.Errorf("invalid response %s", string(bs))
	}

	return nil
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
