package private

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/helpers"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	BINANCE_BASE_URL = "https://api.binance.com"
)

func NewBinanceApi(apikey func() (string, error), apisecret func() (string, error)) (*BinanceApi, error) {
	hitbtcPublic, err := public.NewBinancePublicApi()
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
	b := &BinanceApi{
		BaseURL:           BINANCE_BASE_URL,
		apiV1:             BINANCE_BASE_URL + "/api/v1/",
		apiV3:             BINANCE_BASE_URL + "/api/v3/",
		RateCacheDuration: 30 * time.Second,
		ApiKeyFunc:        apikey,
		SecretKeyFunc:     apisecret,
		settlements:       uniq,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		rt:                &http.Transport{},

		m:         new(sync.Mutex),
		currencyM: new(sync.Mutex),
	}
	b.setTimeOffset()
	return b, nil
}

type BinanceApi struct {
	ApiKeyFunc        func() (string, error)
	SecretKeyFunc     func() (string, error)
	BaseURL           string
	RateCacheDuration time.Duration
	HttpClient        http.Client
	rt                *http.Transport
	settlements       []string
	apiV1             string
	apiV3             string
	timeoffset        int64

	volumeMap       map[string]map[string]float64
	rateMap         map[string]map[string]float64
	precisionMap    map[string]map[string]models.Precisions
	currencyPairs   []models.CurrencyPair
	rateLastUpdated time.Time

	m         *sync.Mutex
	currencyM *sync.Mutex
}

func (h *BinanceApi) privateApiUrl() string {
	return h.BaseURL
}

func (h *BinanceApi) publicApiUrl(command string) string {
	return h.BaseURL + command
}

func (h *BinanceApi) fetchPrecision() error {
	if h.precisionMap != nil {
		return nil
	}

	h.precisionMap = make(map[string]map[string]models.Precisions)
	url := h.publicApiUrl("/api/v1/exchangeInfo")
	req, err := requestGetAsChrome(url)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()
	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	value := gjson.ParseBytes(byteArray)
	for _, v := range value.Get("symbols").Array() {
		trading := v.Get("baseAsset").Str
		settlement := v.Get("quoteAsset").Str
		m, ok := h.precisionMap[trading]
		if !ok {
			m = make(map[string]models.Precisions)
			h.precisionMap[trading] = m
		}
		m[settlement] = models.Precisions{
			PricePrecision:  int(v.Get("baseAssetPrecision").Int()),
			AmountPrecision: int(v.Get("quotePrecision").Int()),
		}
	}

	return errors.Wrapf(err, "failed to fetch %s", url)
}

func (h *BinanceApi) precise(trading string, settlement string) (*models.Precisions, error) {
	if trading == settlement {
		return &models.Precisions{}, nil
	}

	h.fetchPrecision()
	if m, ok := h.precisionMap[trading]; !ok {
		return &models.Precisions{}, errors.Errorf("%s/%s missing trading", trading, settlement)
	} else if precisions, ok := m[settlement]; !ok {
		return &models.Precisions{}, errors.Errorf("%s/%s missing settlement", trading, settlement)
	} else {
		return &precisions, nil
	}
}
func (h *BinanceApi) privateApi(method string, path string, params *url.Values) ([]byte, error) {
	urlStr := h.BaseURL + path
	if strings.ToUpper(method) == "GET" {
		urlStr = urlStr + "?" + params.Encode()
	}
	nonce := time.Now().Unix() * 1000
	params.Set("timestamp", fmt.Sprintf("%d", nonce))

	reader := bytes.NewReader([]byte(params.Encode()))
	req, err := http.NewRequest(method, urlStr, reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	apiKey, err := h.ApiKeyFunc()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	secKey, err := h.SecretKeyFunc()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	req.Header.Set("Accept", "application/json")

	req.Header.Set("X-MBX-APIKEY", apiKey)
	mac := hmac.New(sha256.New, []byte(secKey))
	_, err = mac.Write([]byte(params.Encode()))
	if err != nil {
		return nil, err
	}
	signature := hex.EncodeToString(mac.Sum(nil))
	req.URL.RawQuery = params.Encode() + "&signature=" + signature

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

func (h *BinanceApi) CurrencyPairs() ([]models.CurrencyPair, error) {
	h.currencyM.Lock()
	defer h.currencyM.Unlock()
	if len(h.currencyPairs) != 0 {
		return h.currencyPairs, nil
	}
	url := h.publicApiUrl("/api/v1/exchangeInfo")
	req, err := requestGetAsChrome(url)
	if err != nil {
		return h.currencyPairs, errors.Wrapf(err, "failed to fetch %s", url)
	}
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return h.currencyPairs, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()
	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return h.currencyPairs, errors.Wrapf(err, "failed to fetch %s", url)
	}
	currencyPairs := make([]models.CurrencyPair, 0)
	value := gjson.ParseBytes(byteArray)
	for _, v := range value.Get("symbols").Array() {
		trading := v.Get("baseAsset").Str
		settlement := v.Get("quoteAsset").Str
		currencyPairs = append(currencyPairs, models.CurrencyPair{
			Trading:    trading,
			Settlement: settlement,
		})
	}
	h.currencyPairs = currencyPairs
	return currencyPairs, nil
}

func (h *BinanceApi) TradeFeeRates() (map[string]map[string]TradeFee, error) {
	url := public.BINANCE_BASE_URL + "/api/v3/account"
	resp, err := h.HttpClient.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	value := gjson.ParseBytes(byteArray)
	makerFee := value.Get("makerCommission").Num / 10000
	takerFee := value.Get("takerCommission").Num / 10000
	traderFeeMap := make(map[string]map[string]TradeFee)
	for _, pair := range h.currencyPairs {
		m, ok := traderFeeMap[pair.Trading]
		if !ok {
			m = make(map[string]TradeFee)
			traderFeeMap[pair.Trading] = m
		}
		m[pair.Settlement] = TradeFee{makerFee, takerFee}

	}
	return traderFeeMap, nil
}

func (b *BinanceApi) TradeFeeRate(trading string, settlement string) (TradeFee, error) {
	feeMap, err := b.TradeFeeRates()
	if err != nil {
		return TradeFee{}, err
	}
	return feeMap[trading][settlement], nil
}

type BinanceTransferFeeResponse struct {
	response []byte
	Currency string
	err      error
}

type binanceTransferFeeMap map[string]float64
type binanceTransferFeeSyncMap struct {
	binanceTransferFeeMap
	m *sync.Mutex
}

func (sm *binanceTransferFeeSyncMap) Set(currency string, fee float64) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.binanceTransferFeeMap[currency] = fee
}
func (sm *binanceTransferFeeSyncMap) GetAll() map[string]float64 {
	sm.m.Lock()
	defer sm.m.Unlock()
	return sm.binanceTransferFeeMap
}

func (h *BinanceApi) TransferFee() (map[string]float64, error) {
	params := url.Values{}
	h.buildParamsSigned(&params)
	apiKey, err := h.ApiKeyFunc()
	if err != nil {
		return nil, err
	}
	path := h.BaseURL + "/wapi/v3/assetDetail.html" + params.Encode()
	respmap, err := helpers.HttpGet2(&h.HttpClient, path, map[string]string{"X-MBX-APIKEY": apiKey})
	fmt.Println(respmap)
	transferFeeMap := binanceTransferFeeSyncMap{make(binanceTransferFeeMap), new(sync.Mutex)}
	ad := respmap["assetDetail"]
	if ad == nil {
		return nil, errors.New("nothing trade balance")
	}
	adv := ad.(map[string]interface{})
	for k, v := range adv {
		vv := v.(map[string]interface{})
		fee := vv["withdrawFee"].(float64)
		transferFeeMap.Set(strings.ToUpper(k), fee)
	}
	return transferFeeMap.GetAll(), nil
}

const SERVER_TIME_URL = "time"

func (bn *BinanceApi) setTimeOffset() error {
	respmap, err := helpers.HttpGet(&bn.HttpClient, bn.apiV1+SERVER_TIME_URL)
	if err != nil {
		return err
	}

	stime := int64(helpers.ToInt(respmap["serverTime"]))
	st := time.Unix(stime/1000, 1000000*(stime%1000))
	lt := time.Now()
	offset := st.Sub(lt).Nanoseconds()
	bn.timeoffset = int64(offset)
	return nil
}

func (bn *BinanceApi) buildParamsSigned(postForm *url.Values) error {
	secretKey, err := bn.SecretKeyFunc()
	if err != nil {
		return err
	}
	postForm.Set("recvWindow", "60000")
	tonce := strconv.FormatInt(time.Now().UnixNano()+bn.timeoffset, 10)[0:13]
	postForm.Set("timestamp", tonce)
	payload := postForm.Encode()
	sign, _ := helpers.GetParamHmacSHA256Sign(secretKey, payload)
	postForm.Set("signature", sign)
	return nil
}

const ACCOUNT_URI = "account?"

func (h *BinanceApi) Balances() (map[string]float64, error) {
	params := url.Values{}
	h.buildParamsSigned(&params)
	apiKey, err := h.ApiKeyFunc()
	if err != nil {
		return nil, err
	}
	path := h.apiV3 + ACCOUNT_URI + params.Encode()
	respmap, err := helpers.HttpGet2(&h.HttpClient, path, map[string]string{"X-MBX-APIKEY": apiKey})

	m := make(map[string]float64)

	balances := respmap["balances"].([]interface{})
	for _, v := range balances {
		vv := v.(map[string]interface{})
		currency := vv["asset"].(string)
		currency = strings.ToUpper(currency)
		m[currency] = helpers.ToFloat64(vv["free"])
	}
	return m, nil
}

type BinanceBalance struct {
	T       string
	Balance float64
}

func (h *BinanceApi) CompleteBalances() (map[string]*models.Balance, error) {
	params := url.Values{}
	h.buildParamsSigned(&params)
	apiKey, err := h.ApiKeyFunc()
	if err != nil {
		return nil, err
	}
	path := h.apiV3 + ACCOUNT_URI + params.Encode()
	respmap, err := helpers.HttpGet2(&h.HttpClient, path, map[string]string{"X-MBX-APIKEY": apiKey})

	m := make(map[string]*models.Balance)
	balances := respmap["balances"].([]interface{})
	for _, v := range balances {
		vv := v.(map[string]interface{})
		currency := vv["asset"].(string)
		currency = strings.ToUpper(currency)
		m[currency] = &models.Balance{
			Available: helpers.ToFloat64(vv["free"]),
			OnOrders:  helpers.ToFloat64(vv["locked"]),
		}
	}
	return m, nil
}

func (h *BinanceApi) CompleteBalance(coin string) (*models.Balance, error) {
	m, err := h.CompleteBalances()
	if err != nil {
		return nil, err
	}
	b, ok := m[coin]
	if !ok {
		return nil, errors.Errorf("no coin %s", coin)
	}
	return b, nil
}

type BinanceActiveOrderResponse struct {
	response   []byte
	Trading    string
	Settlement string
	err        error
}

func (h *BinanceApi) ActiveOrders() ([]*models.Order, error) {
	return nil, errors.New("not implemented")
}

func (h *BinanceApi) Order(trading string, settlement string, ordertype models.OrderType, price float64, amount float64) (string, error) {
	params := &url.Values{}

	symbol := strings.ToUpper(fmt.Sprintf("%s%s", trading, settlement))
	params.Set("symbol", symbol)

	if ordertype == models.Bid {
		params.Set("side", "SELL")
	} else if ordertype == models.Ask {
		params.Set("side", "BUY")
	} else {
		return "", errors.Errorf("unknown order type %d", ordertype)
	}
	params.Set("type", "LIMIT")
	params.Set("timeInForce", "GTC")
	precise, err := h.precise(trading, settlement)
	if err != nil {
		return "", err
	}
	params.Set("quantity", FloorFloat64ToStr(amount, precise.AmountPrecision))
	params.Set("price", FloorFloat64ToStr(price, precise.PricePrecision))

	byteArray, err := h.privateApi("POST", "/api/v3/order", params)
	if err != nil {
		return "", err
	}
	value := gjson.ParseBytes(byteArray)

	return value.Get("clientOrderId").Str, nil
}

func (h *BinanceApi) Transfer(typ string, addr string, amount float64, additionalFee float64) error {
	params := &url.Values{}
	amountStr := strconv.FormatFloat(amount, 'f', 4, 64)
	params.Set("asset", typ)
	params.Set("address", addr)
	params.Set("amount", amountStr)
	_, err := h.privateApi("POST", "/wapi/v3/withdraw.html", params)
	return err
}

func (h *BinanceApi) CancelOrder(trading string, settlement string,
	ordertype models.OrderType, orderNumber string) error {
	params := &url.Values{}
	params.Set("symbol", trading+settlement)
	params.Set("origClientOrderId", orderNumber)

	bs, err := h.privateApi("DELETE", "/api/v3/order", params)
	if err != nil {
		return errors.Wrapf(err, "failed to cancel order")
	}
	value := gjson.ParseBytes(bs)
	if orderNumber == value.Get("origClientOrderId").String() {
		return nil
	}
	return errors.Errorf("failed to cancel order %s", orderNumber)
}

func (h *BinanceApi) IsOrderFilled(trading string, settlement string, orderNumber string) (bool, error) {
	params := &url.Values{}
	params.Set("symbol", trading+settlement)
	params.Set("origClientOrderId", orderNumber)
	bs, err := h.privateApi("GET", "/api/v3/order", params)
	if err != nil {
		return false, errors.Wrapf(err, "failed to cancel order")
	}
	value := gjson.ParseBytes(bs)
	if value.Get("isWorking").Bool() {
		return true, nil
	}
	return false, nil
}

func (h *BinanceApi) Address(c string) (string, error) {
	params := &url.Values{}
	params.Set("asset", c)
	bs, err := h.privateApi("GET", "/wapi/v3/depositAddress.html", params)
	if err != nil {
		return "", errors.Wrapf(err, "failed to cancel order")
	}
	value := gjson.ParseBytes(bs)
	return value.Array()[0].Get("address").Str, nil
}
