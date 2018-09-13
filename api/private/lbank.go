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
)

const (
	LBANK_BASE_URL = "https://api.lbkex.com"
)

func NewLbankApi(apikey string, apisecret string) (*LbankApi, error) {
	hitbtcPublic, err := public.NewLbankPublicApi()
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

	return &LbankApi{
		BaseURL:           LBANK_BASE_URL,
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

type LbankApi struct {
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

func (h *LbankApi) privateApiUrl() string {
	return h.BaseURL
}

func (h *LbankApi) privateApi(method string, path string, params *url.Values) ([]byte, error) {
	params.Set("api_key", h.ApiKey)
	queryString := params.Encode() + "&secret_key=" + h.SecretKey
	sign, _ := GetMd5HashSign(queryString)
	params.Set("sign", strings.ToUpper(sign))

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
	return resBody, err
}

func (h *LbankApi) TradeFeeRates() (map[string]map[string]TradeFee, error) {
	cli, err := public.NewClient("lbank")
	if err != nil {
		return nil, err
	}
	pairs, err := cli.CurrencyPairs()
	if err != nil {
		return nil, err
	}
	traderFeeMap := make(map[string]map[string]TradeFee)
	for _, p := range pairs {
		n := make(map[string]TradeFee)
		n[p.Settlement] = TradeFee{0.001, 0.001}
		traderFeeMap[p.Trading] = n
	}
	return traderFeeMap, nil
}

func (b *LbankApi) TradeFeeRate(trading string, settlement string) (TradeFee, error) {
	feeMap, err := b.TradeFeeRates()
	if err != nil {
		return TradeFee{}, err
	}
	return feeMap[trading][settlement], nil
}

type LbankTransferFeeResponse struct {
	response []byte
	Currency string
	err      error
}

type lbankTransferFeeMap map[string]float64
type lbankTransferFeeSyncMap struct {
	lbankTransferFeeMap
	m *sync.Mutex
}

func (sm *lbankTransferFeeSyncMap) Set(currency string, fee float64) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.lbankTransferFeeMap[currency] = fee
}
func (sm *lbankTransferFeeSyncMap) GetAll() map[string]float64 {
	sm.m.Lock()
	defer sm.m.Unlock()
	return sm.lbankTransferFeeMap
}

func (h *LbankApi) TransferFee() (map[string]float64, error) {
	url := LBANK_BASE_URL + "/v1/withdrawConfigs.do"
	resp, err := h.HttpClient.Get(url)
	transferFeeMap := lbankTransferFeeSyncMap{make(lbankTransferFeeMap), new(sync.Mutex)}
	if err != nil {
		return transferFeeMap.GetAll(), errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return transferFeeMap.GetAll(), errors.Wrapf(err, "failed to fetch %s", url)
	}
	json, err := jason.NewValueFromBytes(byteArray)
	if err != nil {
		return transferFeeMap.GetAll(), errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.Array()
	if err != nil {
		return transferFeeMap.GetAll(), errors.Wrapf(err, "failed to parse json")
	}
	for _, v := range data {
		vo, err := v.Object()
		if err != nil {
			continue
		}
		currency, err := vo.GetString("assetCode")
		if err != nil {
			continue
		}
		feeString, err := vo.GetString("fee")
		if err != nil {
			continue
		}
		feef, err := strconv.ParseFloat(feeString, 64)
		if err != nil {
			continue
		}
		transferFeeMap.Set(strings.ToUpper(currency), feef)
	}
	return transferFeeMap.GetAll(), nil
}

func (h *LbankApi) Balances() (map[string]float64, error) {
	params := &url.Values{}
	byteArray, err := h.privateApi("POST", "/v1/user_info.do", params)
	if err != nil {
		return nil, err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObject("info")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key data")
	}
	balances, err := data.GetObject("free")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key list0")
	}
	m := make(map[string]float64)
	for currency, v := range balances.Map() {
		available, err := v.Float64()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse json key list1")
		}
		currency = strings.ToUpper(currency)
		m[currency] = available
	}
	return m, nil
}

type LbankBalance struct {
	T       string
	Balance float64
}

func (h *LbankApi) CompleteBalances() (map[string]*models.Balance, error) {
	params := &url.Values{}
	byteArray, err := h.privateApi("POST", "/v1/user_info.do", params)
	if err != nil {
		return nil, err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObject("info")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key data")
	}
	frees, err := data.GetObject("free")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key list")
	}
	frozens, err := data.GetObject("freeze")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key list")
	}
	m := make(map[string]*models.Balance)
	for currency, v := range frees.Map() {
		available, err := v.Float64()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse json key list 1")
		}

		currency = strings.ToUpper(currency)
		m[currency] = &models.Balance{Available: available}
	}
	for currency, v := range frozens.Map() {
		frozen, err := v.Float64()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse json key list 3")
		}
		currency = strings.ToUpper(currency)
		_, ok := m[currency]
		if ok {
			m[currency].OnOrders = frozen
		} else {
			m[currency] = &models.Balance{OnOrders: frozen}
		}
	}
	return m, nil
}

func (h *LbankApi) CompleteBalance(coin string) (*models.Balance, error) {
	params := &url.Values{}
	byteArray, err := h.privateApi("POST", "/v1/user_info.do", params)
	if err != nil {
		return nil, err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObject("info")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key data")
	}
	frees, err := data.GetObject("free")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key list")
	}
	frozens, err := data.GetObject("freeze")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key list")
	}
	m := make(map[string]*models.Balance)
	for currency, v := range frees.Map() {
		if strings.ToUpper(currency) != coin {
			continue
		}
		available, err := v.Float64()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse json key list 1")
		}
		currency = strings.ToUpper(currency)
		m[currency] = &models.Balance{Available: available}
	}
	for currency, v := range frozens.Map() {
		if strings.ToUpper(currency) != coin {
			continue
		}
		frozen, err := v.Float64()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse json key list 3")
		}
		currency = strings.ToUpper(currency)
		_, ok := m[currency]
		if ok {
			m[currency].OnOrders = frozen
		} else {
			m[currency] = &models.Balance{OnOrders: frozen}
		}
	}
	return m[coin], nil
}

type LbankActiveOrderResponse struct {
	response   []byte
	Trading    string
	Settlement string
	err        error
}

func (h *LbankApi) ActiveOrders() ([]*models.Order, error) {
	return nil, errors.New("not implemented")
}

func (h *LbankApi) Order(trading string, settlement string, ordertype models.OrderType, price float64, amount float64) (string, error) {
	params := &url.Values{}
	if ordertype == models.Ask {
		params.Set("type", "buy")
	} else if ordertype == models.Bid {
		params.Set("type", "sell")
	} else {
		return "", errors.Errorf("unknown order type %d", ordertype)
	}
	params.Set("symbol", strings.ToLower(fmt.Sprintf("%s_%s", trading, settlement)))
	amountStr := strconv.FormatFloat(amount, 'f', 4, 64)
	priceStr := strconv.FormatFloat(price, 'f', 4, 64)
	params.Set("amount", amountStr)
	params.Set("price", priceStr)
	byteArray, err := h.privateApi("POST", "/v1/create_order.do", params)
	if err != nil {
		return "", err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json")
	}
	orderId, err := json.GetString("order_id")
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json")
	}
	return orderId, nil
}

func (h *LbankApi) Transfer(typ string, addr string, amount float64, additionalFee float64) error {
	params := &url.Values{}
	amountStr := strconv.FormatFloat(amount, 'f', 4, 64)
	additionalFeeStr := strconv.FormatFloat(additionalFee, 'f', 4, 64)
	params.Set("account", addr)
	params.Set("assetCode", typ)
	params.Set("amount", amountStr)
	params.Set("fee", additionalFeeStr)
	_, err := h.privateApi("POST", "/v1/withdraw.do", params)
	return err
}

func (h *LbankApi) CancelOrder(trading string, settlement string,
	ordertype models.OrderType, orderNumber string) error {
	params := &url.Values{}
	params.Set("order_id", orderNumber)
	params.Set("symbol", trading+settlement)
	_, err := h.privateApi("POST", "/v1/cancel_order.do", params)
	if err != nil {
		return errors.Wrapf(err, "failed to cancel order")
	}
	return nil
}

func (h *LbankApi) IsOrderFilled(trading string, settlement string, orderNumber string) (bool, error) {
	params := &url.Values{}
	params.Set("order_id", orderNumber)
	params.Set("symbol", trading+settlement)
	bs, err := h.privateApi("POST", "/v1/orders_info.do", params)
	if err != nil {
		return false, errors.Wrapf(err, "failed to cancel order")
	}
	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse json")
	}
	status, err := json.GetString("result")
	if err != nil {
		return false, err
	}
	if status == "true" {
		return true, nil
	}
	return false, nil
}

func (h *LbankApi) Address(c string) (string, error) {
	return "", errors.New("not implemented")
}
