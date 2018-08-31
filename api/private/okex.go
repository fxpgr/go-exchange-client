package private

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"fmt"
	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"strconv"
	"strings"
)

const (
	OKEX_BASE_URL = "https://www.okex.com"
)

func NewOkexApi(apikey string, apisecret string) (*OkexApi, error) {
	hitbtcPublic, err := public.NewOkexPublicApi()
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

	return &OkexApi{
		BaseURL:           OKEX_BASE_URL,
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

type OkexApi struct {
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

func (o *OkexApi) privateApiUrl() string {
	return o.BaseURL
}

func (o *OkexApi) privateApi(method string, path string, params *url.Values) ([]byte, error) {
	params.Set("AccessKeyId", o.ApiKey)
	params.Set("SignatureMethod", "HmacSHA256")
	params.Set("SignatureVersion", "2")
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05"))
	domain := strings.Replace(o.BaseURL, "https://", "", len(o.BaseURL))
	payload := fmt.Sprintf("%s\n%s\n%s\n%s", method, domain, path, params.Encode())
	sign, _ := GetParamHmacSHA256Base64Sign(o.SecretKey, payload)
	params.Set("Signature", sign)
	urlStr := o.BaseURL + path + "?" + params.Encode()
	resBody, err := NewHttpRequest(&http.Client{}, method, urlStr, "", nil)
	return resBody, err
}

func (o *OkexApi) TradeFeeRates() (map[string]map[string]TradeFee, error) {
	cli, err := public.NewClient("okex")
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
		n[p.Settlement] = TradeFee{-0.001, 0.001}
		traderFeeMap[p.Trading] = n
	}
	return traderFeeMap, nil
}

func (o *OkexApi) TradeFeeRate(trading string, settlement string) (TradeFee, error) {
	feeMap, err := o.TradeFeeRates()
	if err != nil {
		return TradeFee{}, err
	}
	return feeMap[trading][settlement], nil
}

type OkexTransferFeeResponse struct {
	response []byte
	Currency string
	err      error
}

type okexTransferFeeMap map[string]float64
type okexTransferFeeSyncMap struct {
	okexTransferFeeMap
	m *sync.Mutex
}

func (sm *okexTransferFeeSyncMap) Set(currency string, fee float64) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.okexTransferFeeMap[currency] = fee
}
func (sm *okexTransferFeeSyncMap) GetAll() map[string]float64 {
	sm.m.Lock()
	defer sm.m.Unlock()
	return sm.okexTransferFeeMap
}

func (o *OkexApi) TransferFee() (map[string]float64, error) {
	cli, err := public.NewClient("okex")
	if err != nil {
		return nil, err
	}
	currencies, err := cli.FrozenCurrency()
	if err != nil {
		return nil, err
	}
	ch := make(chan *OkexTransferFeeResponse, len(currencies))
	workers := make(chan int, 10)
	wg := &sync.WaitGroup{}
	for _, c := range currencies {
		wg.Add(1)
		workers <- 1
		go func(currency string) {
			defer wg.Done()
			args := url.Values{}
			args.Add("currency", strings.ToLower(currency))
			url := o.BaseURL + "/v1/dw/withdraw-virtual/fee-range?" + args.Encode()
			cli := &http.Client{Transport: o.rt}
			resp, err := cli.Get(url)
			if err != nil {
				ch <- &OkexTransferFeeResponse{nil, currency, err}
				return
			}
			defer resp.Body.Close()
			byteArray, err := ioutil.ReadAll(resp.Body)
			ch <- &OkexTransferFeeResponse{byteArray, currency, err}
			<-workers
		}(c)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	transferMap := okexTransferFeeSyncMap{make(okexTransferFeeMap), new(sync.Mutex)}
	for r := range ch {
		if r.err != nil {
			continue
		}
		json, err := jason.NewObjectFromBytes(r.response)
		if err != nil {
			continue
		}
		data, err := json.GetObject("data")
		if err != nil {
			continue
		}
		amount, err := data.GetFloat64("default-amount")
		if err != nil {
			continue
		}
		transferMap.Set(r.Currency, amount)
	}
	return transferMap.GetAll(), nil
}

func (o *OkexApi) Balances() (map[string]float64, error) {
	accountId, err := o.getAccountId()
	if err != nil {
		return nil, err
	}
	params := &url.Values{}
	params.Set("account-id", accountId)
	byteArray, err := o.privateApi("GET", "/v1/account/accounts/"+accountId+"/balance", params)
	if err != nil {
		return nil, err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObject("data")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key data")
	}
	balances, err := data.GetObjectArray("list")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key list")
	}
	m := make(map[string]float64)
	for _, v := range balances {
		currency, err := v.GetString("currency")
		if err != nil {
			continue
		}
		t, err := v.GetString("type")
		if err != nil {
			continue
		}
		if t == "frozen" {
			continue
		}
		availableStr, err := v.GetString("balance")
		if err != nil {
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

type OkexBalance struct {
	T       string
	Balance float64
}

func (o *OkexApi) getAccountId() (string, error) {
	byteArray, err := o.privateApi("GET", "/v1/account/accounts", &url.Values{})
	if err != nil {
		return "", err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json: raw data")
	}
	data, err := json.GetObjectArray("data")
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json: key data")
	}
	if len(data) == 0 {
		return "", errors.New("there is no data")
	}
	accountIdInt, err := data[0].GetInt64("id")
	if err != nil {
		return "", errors.New("there is no available account")
	}
	accountId := strconv.Itoa(int(accountIdInt))
	return accountId, nil
}

func (o *OkexApi) CompleteBalances() (map[string]*models.Balance, error) {
	accountId, err := o.getAccountId()
	if err != nil {
		return nil, err
	}
	params := &url.Values{}
	params.Set("account-id", accountId)
	byteArray, err := o.privateApi("GET", "/v1/account/accounts/"+accountId+"/balance", params)
	if err != nil {
		return nil, err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObject("data")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key data")
	}
	balances, err := data.GetObjectArray("list")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	m := make(map[string]*models.Balance)
	var previousCurrency string
	previousBalance := &models.Balance{}
	var available float64
	for _, v := range balances {
		currency, err := v.GetString("currency")
		if err != nil {
			continue
		}
		t, err := v.GetString("type")
		if err != nil {
			continue
		}
		availableStr, err := v.GetString("balance")
		if err != nil {
			continue
		}
		available, err = strconv.ParseFloat(availableStr, 10)
		if err != nil {
			return nil, err
		}
		if previousCurrency != "" && previousCurrency != currency {
			m[previousCurrency] = previousBalance
			previousCurrency = currency
			previousBalance = &models.Balance{}
		}
		if t == "trade" {
			previousBalance.Available = available
		} else {
			previousBalance.OnOrders = available
		}
		previousCurrency = currency
	}
	return m, nil
}

type OkexActiveOrderResponse struct {
	response   []byte
	Trading    string
	Settlement string
	err        error
}

func (o *OkexApi) ActiveOrders() ([]*models.Order, error) {
	return nil, errors.New("not implemented")
}

func (o *OkexApi) Order(trading string, settlement string, ordertype models.OrderType, price float64, amount float64) (string, error) {
	accountId, err := o.getAccountId()
	if err != nil {
		return "", err
	}
	params := &url.Values{}
	if ordertype == models.Ask {
		params.Set("type", "buy-limit")
	} else if ordertype == models.Bid {
		params.Set("type", "sell-limit")
	} else {
		return "", errors.Errorf("unknown order type %d", ordertype)
	}
	params.Set("symbol", strings.ToLower(fmt.Sprintf("%s%s", trading, settlement)))
	params.Set("account-id", accountId)
	amountStr := strconv.FormatFloat(amount, 'f', 4, 64)
	priceStr := strconv.FormatFloat(price, 'f', 4, 64)
	params.Set("amount", amountStr)
	params.Set("price", priceStr)
	byteArray, err := o.privateApi("GET", "/v1/order/orders/place", params)
	if err != nil {
		return "", err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json")
	}
	orderId, err := json.GetString("data")
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json")
	}
	return orderId, nil
}

func (o *OkexApi) Transfer(typ string, addr string, amount float64, additionalFee float64) error {
	params := &url.Values{}
	amountStr := strconv.FormatFloat(amount, 'f', 4, 64)
	additionalFeeStr := strconv.FormatFloat(additionalFee, 'f', 4, 64)
	params.Set("address", addr)
	params.Set("amount", amountStr)
	params.Set("currency", typ)
	params.Set("fee", additionalFeeStr)
	_, err := o.privateApi("GET", "/v1/dw/withdraw/api/create", params)
	return err
}

func (o *OkexApi) CancelOrder(orderNumber string, _ string) error {
	params := &url.Values{}
	params.Set("order-id", orderNumber)
	_, err := o.privateApi("POST", "/v1/order/orders/"+orderNumber+"/submitcancel", params)
	if err != nil {
		return errors.Wrapf(err, "failed to cancel order")
	}
	return nil
}

func (o *OkexApi) IsOrderFilled(orderNumber string, _ string) (bool, error) {
	params := &url.Values{}
	params.Set("order-id", orderNumber)
	bs, err := o.privateApi("POST", "/v1/order/orders/"+orderNumber, params)
	if err != nil {
		return false, errors.Wrapf(err, "failed to cancel order")
	}
	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse json")
	}
	data, err := json.GetObject("data")
	if err != nil {
		return false, err
	}
	status, err := data.GetString("state")
	if err != nil {
		return false, err
	}
	if status == "filled" {
		return true, nil
	}
	return false, nil
}

func (o *OkexApi) Address(c string) (string, error) {
	params := &url.Values{}
	params.Set("currency", strings.ToLower(c))
	params.Set("type", "deposit")

	bs, err := o.privateApi("GET", "/v1/dw/deposit-virtual/addresses", params)
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch deposit address")
	}
	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse json")
	}
	fmt.Println(json)
	address, err := json.GetString("data")
	if err != nil {
		return "", errors.Wrapf(err, "failed to take address of %s", c)
	}
	return address, nil
}
