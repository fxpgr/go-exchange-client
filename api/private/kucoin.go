package private

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"bytes"
	"fmt"
	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"strconv"
	"strings"
	"encoding/base64"
	"github.com/fxpgr/go-exchange-client/logger"
)

const (
	KUCOIN_BASE_URL = "https://api.kucoin.com"
)

func NewKucoinApi(apikey string, apisecret string) (*KucoinApi, error) {
	hitbtcPublic, err := public.NewKucoinPublicApi()
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize public client")
	}
	pairs, err := hitbtcPublic.CurrencyPairs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pairs")
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

	return &KucoinApi{
		BaseURL:           KUCOIN_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		ApiKey:            apikey,
		SecretKey:         apisecret,
		settlements:       uniq,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		rt:                &http.Transport{},

		m: new(sync.Mutex),
	}, nil
}

type KucoinApi struct {
	ApiKey            string
	SecretKey         string
	BaseURL           string
	RateCacheDuration time.Duration
	HttpClient        http.Client
	rt                *http.Transport
	settlements       []string

	volumeMap       map[string]map[string]float64
	rateMap         map[string]map[string]float64
	rateLastUpdated time.Time

	m *sync.Mutex
}

func (h *KucoinApi) privateApiUrl() string {
	return h.BaseURL
}

func (h *KucoinApi) privateApi(method string, path string, params *url.Values) ([]byte, error) {

	var urlStr string
	urlStr = h.BaseURL + path
	if strings.ToUpper(method) == "GET" {
		urlStr = urlStr + "?" + params.Encode()
	}

	reader := bytes.NewReader([]byte(params.Encode()))
	req, err := http.NewRequest(method, urlStr, reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	req.Header.Set("Accept", "application/json")

	nonce := time.Now().UnixNano() / int64(time.Millisecond)

	strForSign := fmt.Sprintf("%s/%v/%s", path, nonce, params.Encode())
	signatureStr := base64.StdEncoding.EncodeToString([]byte(strForSign))
	signature := computeHmac256(signatureStr, h.SecretKey)
	req.Header.Add("KC-API-KEY", h.ApiKey)
	req.Header.Add("KC-API-NONCE", fmt.Sprintf("%v", nonce))
	req.Header.Add(
		"KC-API-SIGNATURE", signature,
	)

	res, err := h.HttpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to request command %s", path)
	}
	defer res.Body.Close()

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch result of command %s", path)
	}
	return resBody, err
}

func (h *KucoinApi) TradeFeeRates() (map[string]map[string]TradeFee, error) {
	url := public.KUCOIN_BASE_URL+"/v1/open/tick"
	resp, err := h.HttpClient.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObjectArray("data")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	traderFeeMap := make(map[string]map[string]TradeFee)
	for _, v := range data {
		trading, err:=v.GetString("coinType")
		if err != nil {
			return  nil,errors.Wrapf(err, "failed to parse object")
		}
		settlement,err:=v.GetString("coinTypePair")
		if err != nil {
			return  nil,errors.Wrapf(err, "failed to parse object")
		}
		feeRate,err := v.GetFloat64("feeRate")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse isTrading")
		}
		n := make(map[string]TradeFee)
		n[settlement] = TradeFee{feeRate,feeRate}
		traderFeeMap[trading] = n
	}
	return traderFeeMap, nil
}

func (b *KucoinApi) TradeFeeRate(trading string, settlement string) (TradeFee, error) {
	feeMap, err := b.TradeFeeRates()
	if err != nil {
		return TradeFee{}, err
	}
	return feeMap[trading][settlement], nil
}

type KucoinTransferFeeResponse struct {
	response []byte
	Currency string
	err      error
}

type kucoinTransferFeeMap map[string]float64
type kucoinTransferFeeSyncMap struct {
	kucoinTransferFeeMap
	m *sync.Mutex
}

func (sm *kucoinTransferFeeSyncMap) Set(currency string, fee float64) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.kucoinTransferFeeMap[currency] = fee
}
func (sm *kucoinTransferFeeSyncMap) GetAll() map[string]float64 {
	sm.m.Lock()
	defer sm.m.Unlock()
	return sm.kucoinTransferFeeMap
}

func (h *KucoinApi) TransferFee() (map[string]float64, error) {
	url := KUCOIN_BASE_URL + "/v1/market/open/coins"
	resp, err := h.HttpClient.Get(url)
	transferFeeMap := kucoinTransferFeeSyncMap{make(kucoinTransferFeeMap), new(sync.Mutex)}
	if err != nil {
		return transferFeeMap.GetAll(), errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return transferFeeMap.GetAll(), errors.Wrapf(err, "failed to fetch %s", url)
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return transferFeeMap.GetAll(), errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObjectArray("data")
	if err != nil {
		return transferFeeMap.GetAll(), errors.Wrapf(err, "failed to parse json")
	}
	for _, v := range data {
		feef,err := v.GetFloat64("withdrawMinFee")
		if err != nil {
			continue
		}
		coin,err := v.GetString("coin")
		if err != nil {
			continue
		}
		transferFeeMap.Set(strings.ToUpper(coin), feef)
	}
	return transferFeeMap.GetAll(), nil
}

func (h *KucoinApi) Balances() (map[string]float64, error) {
	params := &url.Values{}
	params.Set("limit", "20")
	byteArray, err := h.privateApi("GET", "/v1/account/balances", params)
	if err != nil {
		return nil, err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObjectArray("data")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key data")
	}
	m := make(map[string]float64)
	for _, v := range data {
		logger.Get().Errorf("%v", v)
		var currency string
		var balance float64
		var freeze float64
		for k,s:= range v.Map() {
			if k =="coinType" {
				currency, err= s.String()
				if err != nil {
					return nil, errors.Wrapf(err, "failed to parse json key data")
				}
			} else if k == "balance" {
				balance, err= s.Float64()
				if err != nil {
					return nil, errors.Wrapf(err, "failed to parse json key data")
				}
			} else if k =="freezeBalance" {
				freeze, err= s.Float64()
				if err != nil {
					return nil, errors.Wrapf(err, "failed to parse json key data")
				}
			}
		}
		currency = strings.ToUpper(currency)
		m[currency] = balance-freeze
	}
	return m, nil
}

type KucoinBalance struct {
	T       string
	Balance float64
}

func (h *KucoinApi) CompleteBalances() (map[string]*models.Balance, error) {
	params := &url.Values{}
	params.Set("limit", "20")
	byteArray, err := h.privateApi("GET", "/v1/account/balances", params)
	if err != nil {
		return nil, err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObjectArray("data")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key data")
	}
	m := make(map[string]*models.Balance)
	for _, v := range data {
		currency, err:= v.GetString("coinType")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse json key list1")
		}
		balance, err:= v.GetFloat64("balance")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse json key list1")
		}
		freeze, err:= v.GetFloat64("freezeBalance")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse json key list1")
		}
		currency = strings.ToUpper(currency)
		m[currency] = &models.Balance{
			Available: balance-freeze,
			OnOrders:freeze,
		}
	}
	return m, nil
}

type KucoinActiveOrderResponse struct {
	response   []byte
	Trading    string
	Settlement string
	err        error
}

func (h *KucoinApi) ActiveOrders() ([]*models.Order, error) {
	return nil, errors.New("not implemented")
}

func (h *KucoinApi) Order(trading string, settlement string, ordertype models.OrderType, price float64, amount float64) (string, error) {
	params := &url.Values{}
	if ordertype == models.Ask {
		params.Set("type", "1")
	} else if ordertype == models.Bid {
		params.Set("type", "0")
	} else {
		return "", errors.Errorf("unknown order type %d", ordertype)
	}
	amountStr := strconv.FormatFloat(amount, 'f', 4, 64)
	priceStr := strconv.FormatFloat(price, 'f', 4, 64)
	params.Set("amount", amountStr)
	params.Set("price", priceStr)
	byteArray, err := h.privateApi("POST", "/v1/order?symbol="+strings.ToUpper(fmt.Sprintf("%s-%s", trading, settlement)), params)
	if err != nil {
		return "", err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json object")
	}
	data, err := json.GetObject("data")
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json data")
	}
	orderId, err := data.GetString("orderOid")
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json orderId")
	}
	return orderId, nil
}

func (h *KucoinApi) Transfer(typ string, addr string, amount float64, additionalFee float64) error {
	params := &url.Values{}
	amountStr := strconv.FormatFloat(amount, 'f', 4, 64)
	params.Set("address", addr)
	params.Set("coin", typ)
	params.Set("amount", amountStr)
	_, err := h.privateApi("POST", fmt.Sprintf("/v1/account/%s/withdraw/apply",typ), params)
	return err
}

func (h *KucoinApi) CancelOrder(orderNumber string, currencyPair string) error {
	params := &url.Values{}
	params.Set("order_id", orderNumber)
	params.Set("symbol", currencyPair)
	_, err := h.privateApi("POST", "/v1/cancel-order?symbol="+currencyPair, params)
	if err != nil {
		return errors.Wrapf(err, "failed to cancel order")
	}
	return nil
}

func (h *KucoinApi) IsOrderFilled(orderNumber string, symbol string) (bool, error) {
	params := &url.Values{}
	params.Set("symbol", symbol)
	bs, err := h.privateApi("POST", "/v1/order/active", params)
	if err != nil {
		return false, errors.Wrapf(err, "failed to cancel order")
	}
	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse json")
	}
	data, err := json.GetObject("data")
	if err != nil {
		return false, errors.Wrap(err, "failed to parse json")
	}
	buys, err := data.GetValueArray("BUY")
	if err != nil {
		return false, errors.Wrap(err, "failed to parse json")
	}
	sells, err := data.GetValueArray("SELL")
	if err != nil {
		return false, errors.Wrap(err, "failed to parse json")
	}
	for _,s := range sells {
		sary,err := s.Array()
		if err != nil {
			return false, errors.Wrap(err, "failed to parse json")
		}
		orderId,err := sary[5].String()
		if err != nil {
			return false, errors.Wrap(err, "failed to parse json")
		}
		if orderId == orderNumber {
			return true,nil
		}
	}
	for _,s := range buys {
		sary,err := s.Array()
		if err != nil {
			return false, errors.Wrap(err, "failed to parse json")
		}
		orderId,err := sary[5].String()
		if err != nil {
			return false, errors.Wrap(err, "failed to parse json")
		}
		if orderId == orderNumber {
			return true,nil
		}
	}
	return false, nil
}

func (h *KucoinApi) Address(c string) (string, error) {
	params := &url.Values{}
	bs, err := h.privateApi("GET", fmt.Sprintf("/v1/account/%s/wallet/address",c), params)
	if err != nil {
		return "", errors.Wrapf(err, "failed to cancel order")
	}
	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse json")
	}
	data, err := json.GetObject("data")
	if err != nil {
		return "", errors.Wrap(err, "failed to parse json")
	}
	address, err := data.GetString("address")
	if err != nil {
		return "", errors.Wrap(err, "failed to parse json")
	}
	return address, errors.New("not implemented")
}
