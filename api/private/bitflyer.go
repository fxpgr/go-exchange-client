package private

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/antonholmquist/jason"
	"github.com/davecgh/go-spew/spew"
	"github.com/fxpgr/go-ccex-api-client/logger"
	"github.com/fxpgr/go-ccex-api-client/models"
	"github.com/pkg/errors"
)

const (
	BITFLYER_BASE_URL = "https://api.bitflyer.jp"
)

type BitflyerApiConfig struct {
	Apikey    string
	ApiSecret string
	BaseURL   string

	RateCacheDuration time.Duration
}

type BitflyerApi struct {
	volumeMap       map[string]map[string]float64
	rateMap         map[string]map[string]float64
	rateLastUpdated time.Time

	m *sync.Mutex
	c *BitflyerApiConfig
}

func NewBitflyerApiUsingConfigFunc(f func(*BitflyerApiConfig)) (*BitflyerApi, error) {
	conf := &BitflyerApiConfig{
		BaseURL:           BITFLYER_BASE_URL,
		RateCacheDuration: 30 * time.Second,
	}
	f(conf)

	api := &BitflyerApi{
		rateMap:         nil,
		volumeMap:       nil,
		rateLastUpdated: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),

		m: new(sync.Mutex),
		c: conf,
	}
	return api, nil
}

func NewBitflyerApi(apikey string, apisecret string) (*BitflyerApi, error) {
	return NewBitflyerApiUsingConfigFunc(func(c *BitflyerApiConfig) {
		c.Apikey = apikey
		c.ApiSecret = apisecret
	})
}

func (b *BitflyerApi) privateApiUrl() string {
	return b.c.BaseURL
}

func (b *BitflyerApi) privateApi(method string, path string, args map[string]string) ([]byte, error) {
	cli := &http.Client{}
	var err error

	val := url.Values{}
	if args != nil {
		for k, v := range args {
			val.Add(k, v)
		}
	}
	nonce := strconv.FormatInt(time.Now().Unix(), 10)
	jsonString, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	text := nonce + method + path + string(jsonString)

	reader := bytes.NewReader([]byte(val.Encode()))
	req, err := http.NewRequest(method, b.privateApiUrl()+path, reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}

	mac := hmac.New(sha256.New, []byte(b.c.ApiSecret))
	_, err = mac.Write([]byte(text))
	if err != nil {
		return nil, errors.Wrap(err, "failed to encrypt request")
	}
	sign := mac.Sum(nil)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("ACCESS-TIMESTAMP", nonce)
	req.Header.Add("ACCESS-KEY", b.c.Apikey)
	req.Header.Add("ACCESS-SIGN", hex.EncodeToString(sign))

	res, err := cli.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to request command %s", path)
	}
	defer res.Body.Close()

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch result of command %s", path)
	}

	logger.Get().Infof("[bitflyer] private api called: cmd=%s req=%s, res=%.60s", path, spew.Sdump(args), string(resBody))

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

func (b *BitflyerApi) PurchaseFeeRate() (float64, error) {
	purchaseFeeurl := "/v1/me/gettradingcommission?product_code=BTC_JPY"
	method := "GET"
	resBody, err := b.privateApi(purchaseFeeurl, method, map[string]string{})
	if err != nil {
		return 1, err
	}
	purchaseFeeObject, err := jason.NewObjectFromBytes(resBody)
	if err != nil {
		return 1, err
	}
	purchaseFeeMap := purchaseFeeObject.Map()
	purchaseFee, err := purchaseFeeMap["commission_rate"].Float64()
	if err != nil {
		return 1, err
	}
	return purchaseFee, nil
}

func (b *BitflyerApi) SellFeeRate() (float64, error) {
	return b.PurchaseFeeRate()
}

func (b *BitflyerApi) TransferFee() (map[string]float64, error) {
	return nil, nil
}

func (b *BitflyerApi) Balances() (map[string]float64, error) {
	balancepath := "/v1/me/getbalance"

	method := "GET"

	resBody, err := b.privateApi(balancepath, method, map[string]string{})
	if err != nil {
		return nil, err
	}

	balances, err := jason.NewValueFromBytes(resBody)
	if err != nil {
		return nil, err
	}

	balanceArray, err := balances.Array()
	if err != nil {
		return nil, err
	}
	balancemap := make(map[string]float64)

	for i := range balanceArray {
		balanceObject, err := balanceArray[i].Object()
		if err != nil {
			return nil, err
		}

		balancetmpmap := balanceObject.Map()
		cur, err := balancetmpmap["currency_code"].String()
		if err != nil {
			return nil, err
		}
		avi, err := balancetmpmap["available"].Float64()
		if err != nil {
			return nil, err
		}
		balancemap[cur] = avi
	}
	return balancemap, nil
}

func (b *BitflyerApi) CompleteBalances() (map[string]*models.Balance, error) {
	balancepath := "/v1/me/getbalance"
	method := "GET"

	resBody, err := b.privateApi(balancepath, method, map[string]string{})
	if err != nil {
		return nil, err
	}

	balances, err := jason.NewValueFromBytes(resBody)
	if err != nil {
		return nil, err
	}

	balanceArray, err := balances.Array()
	if err != nil {
		return nil, err
	}
	completebalancemap := make(map[string]*models.Balance)

	for i := range balanceArray {
		balanceObject, err := balanceArray[i].Object()
		if err != nil {
			return nil, err
		}

		balancetmpmap := balanceObject.Map()
		cur, err := balancetmpmap["currency_code"].String()
		if err != nil {
			return nil, err
		}
		avi, err := balancetmpmap["available"].Float64()
		if err != nil {
			return nil, err
		}
		amount, err := balancetmpmap["amount"].Float64()
		if err != nil {
			return nil, err
		}
		completeBalance := models.NewBalance(avi, amount-avi)
		completebalancemap[cur] = completeBalance
	}
	return completebalancemap, nil
}

func (b *BitflyerApi) ActiveOrders() ([]*models.Order, error) {
	activeOrderurl := "/v1/me/getchildorders?child_order_state=ACTIVE&product_code=BTC_JPY"
	method := "GET"
	params := make(map[string]string)
	params["child_order_state"] = "ACTIVE"
	resBody, err := b.privateApi(activeOrderurl, method, params)
	if err != nil {
		return nil, err
	}
	activeOrderValue, err := jason.NewValueFromBytes(resBody)
	if err != nil {
		return nil, err
	}
	activeOrderArray, err := activeOrderValue.ObjectArray()
	if err != nil {
		return nil, err
	}
	var activeOrders []*models.Order

	for _, v := range activeOrderArray {
		exchangeOrderId, err := v.GetString("id")
		if err != nil {
			continue
		}
		var orderType models.OrderType
		orderTypeStr, err := v.GetString("side")
		if err != nil {
			continue
		}
		if orderTypeStr == "BUY" {
			orderType = models.Ask
		} else if orderTypeStr == "SELL" {
			orderType = models.Bid
		}
		productCodeStr, err := v.GetString("product_code")
		if err != nil {
			continue
		}
		trading, settlement, err := parseCurrencyPair(productCodeStr)
		amount, err := v.GetFloat64("size")
		if err != nil {
			continue
		}
		price, err := v.GetFloat64("price")
		if err != nil {
			continue
		}
		activeOrders = append(activeOrders, &models.Order{
			ExchangeOrderID: exchangeOrderId,
			Type:            orderType,
			Trading:         trading,
			Settlement:      settlement,
			Price:           price,
			Amount:          amount,
		})

	}
	return activeOrders, nil
}

type orderBitflyerRespnose struct {
	OrderNumber string `json:"child_order_acceptance_id,string"`
}

func (b *BitflyerApi) Order(trading string, settlement string, ordertype models.OrderType, price float64, amount float64) (string, error) {
	orderpath := "/v1/me/sendchildorder"
	method := "POST"

	param := make(map[string]string)
	param["product_code"] = trading + "_" + settlement
	param["child_order_type"] = "LIMIT"

	var cmd string
	if ordertype == models.Ask {
		cmd = "BUY"
	} else if ordertype == models.Bid {
		cmd = "SELL"
	} else {
		return "", errors.Errorf("unknown order type %d", ordertype)
	}
	param["side"] = cmd

	param["price"] = strconv.FormatFloat(price, 'f', 8, 64)
	param["size"] = strconv.FormatFloat(amount, 'f', 8, 64)

	bs, err := b.privateApi(orderpath, method, param)
	if err != nil {
		return "", errors.Wrap(err, "failed to request order")
	}
	var res orderBitflyerRespnose
	err = json.Unmarshal(bs, &res)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse response json %s", string(bs))
	}
	return res.OrderNumber, nil
}

func (b *BitflyerApi) Transfer(typ string, addr string,
	amount float64, additionalFee float64) error {
	return errors.New("bitflyer transfer api not implemented")
}

func (b *BitflyerApi) CancelOrder(orderNumber string, productCode string) error {
	args := make(map[string]string)
	args["child_order_id"] = orderNumber
	args["product_code"] = productCode

	_, err := b.privateApi("POST", "/v1/me/sendchildorder", args)
	if err != nil {
		return errors.Wrapf(err, "failed to cancel order")
	}

	return nil
}

func (b *BitflyerApi) Address(c string) (string, error) {
	return "", errors.New("bitflyer address api not implemented")
}
