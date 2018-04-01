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

	"github.com/Jeffail/gabs"
	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
)

const (
	BITFLYER_BASE_URL = "https://api.bitflyer.jp"
)

type BitflyerApiConfig struct {
}

type BitflyerApi struct {
	Apikey            string
	ApiSecret         string
	BaseURL           string
	RateCacheDuration time.Duration
	HttpClient        http.Client
	Mode              ClientMode

	volumeMap       map[string]map[string]float64
	rateMap         map[string]map[string]float64
	rateLastUpdated time.Time

	m *sync.Mutex
}

func NewBitflyerPrivateApi(mode ClientMode, apikey string, apisecret string) (*BitflyerApi, error) {
	api := &BitflyerApi{
		Apikey:            apikey,
		ApiSecret:         apisecret,
		BaseURL:           BITFLYER_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		Mode:              mode,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),

		m: new(sync.Mutex),
	}
	return api, nil
}

func (b *BitflyerApi) privateApiUrl() string {
	return b.BaseURL
}

func (b *BitflyerApi) privateApi(method string, path string, args map[string]string) ([]byte, error) {
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

	mac := hmac.New(sha256.New, []byte(b.ApiSecret))
	_, err = mac.Write([]byte(text))
	if err != nil {
		return nil, errors.Wrap(err, "failed to encrypt request")
	}
	sign := mac.Sum(nil)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("ACCESS-TIMESTAMP", nonce)
	req.Header.Add("ACCESS-KEY", b.Apikey)
	req.Header.Add("ACCESS-SIGN", hex.EncodeToString(sign))

	resp, err := b.HttpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to request command %s", path)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, errors.Wrapf(err, "failed to fetch %s", path)
	}
	return byteArray, nil
}

func (b *BitflyerApi) PurchaseFeeRate() (float64, error) {
	purchaseFeeurl := "/v1/me/gettradingcommission?product_code=BTC_JPY"
	method := "GET"
	resBody, err := b.privateApi(method, purchaseFeeurl, map[string]string{})
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

func (b *BitflyerApi) TradeFeeRate() (map[string]map[string]TradeFee, error) {
	purchaseFeeurl := "/v1/me/gettradingcommission?product_code=BTC_JPY"
	method := "GET"
	resBody, err := b.privateApi(method, purchaseFeeurl, map[string]string{})
	if err != nil {
		return nil, err
	}
	purchaseFeeObject, err := jason.NewObjectFromBytes(resBody)
	if err != nil {
		return nil, err
	}
	purchaseFeeMap := purchaseFeeObject.Map()
	purchaseFee, err := purchaseFeeMap["commission_rate"].Float64()
	if err != nil {
		return nil, err
	}

	traderFeeMap := make(map[string]map[string]TradeFee)
	for trading, v := range b.rateMap {
		m := make(map[string]TradeFee)
		for settlement, _ := range v {
			m[settlement] = TradeFee{TakerFee: purchaseFee, MakerFee: purchaseFee}
		}
		traderFeeMap[trading] = m
	}
	return traderFeeMap, nil
}

func (b *BitflyerApi) TransferFee() (map[string]float64, error) {
	return nil, nil
}

func (b *BitflyerApi) Balances() (map[string]float64, error) {
	balancepath := "/v1/me/getbalance"

	method := "GET"

	resBody, err := b.privateApi(method, balancepath, map[string]string{})
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

	resBody, err := b.privateApi(method, balancepath, map[string]string{})
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

func (b *BitflyerApi) IsOrderFilled(orderNumber string, _ string) (bool, error) {
	orders, err := b.ActiveOrders()
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

func (b *BitflyerApi) ActiveOrders() ([]*models.Order, error) {
	activeOrderurl := "/v1/me/getchildorders?child_order_state=ACTIVE&product_code=BTC_JPY"
	method := "GET"
	params := make(map[string]string)
	params["child_order_state"] = "ACTIVE"
	resBody, err := b.privateApi(method, activeOrderurl, params)
	if err != nil {
		return nil, err
	}
	json, err := gabs.ParseJSON(resBody)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	activeOrderArray, err := json.Children()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	var activeOrders []*models.Order

	for _, v := range activeOrderArray {
		exchangeOrderId, ok := v.Path("child_order_acceptance_id").Data().(string)
		if !ok {
			continue
		}
		var orderType models.OrderType
		orderTypeStr, ok := v.Path("side").Data().(string)
		if !ok {
			continue
		}
		if orderTypeStr == "BUY" {
			orderType = models.Ask
		} else if orderTypeStr == "SELL" {
			orderType = models.Bid
		}
		productCodeStr, ok := v.Path("product_code").Data().(string)
		if !ok {
			continue
		}
		trading, settlement, err := parseCurrencyPair(productCodeStr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse currency pair")
		}
		amount, ok := v.Path("size").Data().(float64)
		if !ok {
			continue
		}
		price, ok := v.Path("price").Data().(float64)
		if !ok {
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
	OrderNumber string `json:"child_order_acceptance_id"`
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

	bs, err := b.privateApi(method, orderpath, param)
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
