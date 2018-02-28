package private

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/antonholmquist/jason"
	"github.com/davecgh/go-spew/spew"
	"github.com/fxpgr/go-ccex-api-client/logger"
	"github.com/pkg/errors"
)

const (
	HITBTC_BASE_URL = "https://api.hitbtc.com"
)

func NewHitbtcApi(apikey string, apisecret string) (*HitbtcApi, error) {
	return &HitbtcApi{
		BaseURL:           HITBTC_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		ApiKey:            apikey,
		SecretKey:         apisecret,
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
	req.SetBasicAuth(h.ApiKey,h.SecretKey)
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

	logger.Get().Infof("[hitbtc] private api called: cmd=%s req=%s, res=%.60s", path, spew.Sdump(args), string(resBody))

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

func (h *HitbtcApi) fetchFeeRate() (float64, error) {
	purchaseFeeurl := "/api/2/trading/fee/ETHBTC"
	method := "GET"
	resBody, err := h.privateApi(method, purchaseFeeurl, map[string]string{})
	if err != nil {
		return 1, err
	}
	purchaseFeeObject, err := jason.NewObjectFromBytes(resBody)
	if err != nil {
		return 1, err
	}
	purchaseFeeMap := purchaseFeeObject.Map()
	purchaseFee, err := purchaseFeeMap["takeLiquidityRate"].Float64()
	if err != nil {
		return 1, err
	}
	return purchaseFee, nil
}

func (h *HitbtcApi) PurchaseFeeRate() (float64, error) {
	return h.fetchFeeRate()
}

func (h *HitbtcApi) SellFeeRate() (float64, error) {
	return h.fetchFeeRate()
}

func (h *HitbtcApi) TransferFee() (map[string]float64, error) {
	return nil, nil
}
/*
func (h *HitbtcApi) Balances() (map[string]float64, error) {
	bs, err := h.privateApi("GET","/api/2/trading/balance", nil)
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

func (h *HitbtcApi) CompleteBalances() (map[string]*models.Balance, error) {
	bs, err := h.privateApi("returnCompleteBalances", nil)
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

func (h *HitbtcApi) ActiveOrders() ([]*models.Order, error) {
	bs, err := h.privateApi("returnOpenOrders", map[string]string{
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

func (h *HitbtcApi) Order(trading string, settlement string, ordertype models.OrderType, price float64, amount float64) (string, error) {
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

	bs, err := h.privateApi(cmd, args)
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
		return "", errors.Errorf("invalid order number %v", res.OrderNumber)
	}

	return res.OrderNumber, nil
}

func (h *HitbtcApi) Transfer(typ string, addr string, amount float64, additionalFee float64) error {
	args := make(map[string]string)
	args["address"] = addr
	args["currency"] = typ
	args["amount"] = strconv.FormatFloat(amount, 'g', -1, 64)

	bs, err := h.privateApi("withdraw", args)
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

func (h *HitbtcApi) CancelOrder(orderNumber string, _ string) error {
	args := make(map[string]string)
	args["orderNumber"] = orderNumber

	bs, err := h.privateApi("cancelOrder", args)
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

func (h *HitbtcApi) Address(c string) (string, error) {
	bs, err := h.privateApi("returnDepositAddresses", nil)
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
*/