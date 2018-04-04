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
	HUOBI_BASE_URL = "https://api.huobi.pro"
)

func NewHuobiApi(apikey string, apisecret string) (*HuobiApi, error) {
	hitbtcPublic, err := public.NewHuobiPublicApi()
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

	return &HuobiApi{
		BaseURL:           HUOBI_BASE_URL,
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

type HuobiApi struct {
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

func (h *HuobiApi) privateApiUrl() string {
	return h.BaseURL
}

func (h *HuobiApi) privateApi(method string, path string, params *url.Values) ([]byte, error) {
	params.Set("AccessKeyId", h.ApiKey)
	params.Set("SignatureMethod", "HmacSHA256")
	params.Set("SignatureVersion", "2")
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05"))
	domain := strings.Replace(h.BaseURL, "https://", "", len(h.BaseURL))
	payload := fmt.Sprintf("%s\n%s\n%s\n%s", method, domain, path, params.Encode())
	sign, _ := GetParamHmacSHA256Base64Sign(h.SecretKey, payload)
	params.Set("Signature", sign)
	urlStr := h.BaseURL + path + "?" + params.Encode()
	resBody, err := NewHttpRequest(&http.Client{}, method, urlStr, "", nil)
	return resBody, err
}

func (h *HuobiApi) TradeFeeRates() (map[string]map[string]TradeFee, error) {
	cli, err := public.NewClient("huobi")
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
		n[p.Settlement] = TradeFee{0.002, 0.002}
		traderFeeMap[p.Trading] = n
	}
	return traderFeeMap, nil
}

func (b *HuobiApi) TradeFeeRate(trading string, settlement string) (TradeFee, error) {
	feeMap, err := b.TradeFeeRates()
	if err != nil {
		return TradeFee{}, err
	}
	return feeMap[trading][settlement], nil
}

type HuobiTransferFeeResponse struct {
	response []byte
	Currency string
	err      error
}

type huobiTransferFeeMap map[string]float64
type huobiTransferFeeSyncMap struct {
	huobiTransferFeeMap
	m *sync.Mutex
}

func (sm *huobiTransferFeeSyncMap) Set(currency string, fee float64) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.huobiTransferFeeMap[currency] = fee
}
func (sm *huobiTransferFeeSyncMap) GetAll() map[string]float64 {
	sm.m.Lock()
	defer sm.m.Unlock()
	return sm.huobiTransferFeeMap
}

func (h *HuobiApi) TransferFee() (map[string]float64, error) {
	cli, err := public.NewClient("huobi")
	if err != nil {
		return nil, err
	}
	currencies, err := cli.FrozenCurrency()
	if err != nil {
		return nil, err
	}
	ch := make(chan *HuobiTransferFeeResponse, len(currencies))
	workers := make(chan int, 10)
	wg := &sync.WaitGroup{}
	for _, c := range currencies {
		wg.Add(1)
		workers <- 1
		go func(currency string) {
			defer wg.Done()
			args := url.Values{}
			args.Add("currency", strings.ToLower(currency))
			url := h.BaseURL + "/v1/dw/withdraw-virtual/fee-range?" + args.Encode()
			cli := &http.Client{Transport: h.rt}
			resp, err := cli.Get(url)
			if err != nil {
				ch <- &HuobiTransferFeeResponse{nil, currency, err}
				return
			}
			defer resp.Body.Close()
			byteArray, err := ioutil.ReadAll(resp.Body)
			ch <- &HuobiTransferFeeResponse{byteArray, currency, err}
			<-workers
		}(c)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	transferMap := huobiTransferFeeSyncMap{make(huobiTransferFeeMap), new(sync.Mutex)}
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

func (h *HuobiApi) Balances() (map[string]float64, error) {
	accountId, err := h.getAccountId()
	if err != nil {
		return nil, err
	}
	params := &url.Values{}
	params.Set("account-id", accountId)
	byteArray, err := h.privateApi("GET", "/v1/account/accounts/"+accountId+"/balance", params)
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

type HuobiBalance struct {
	T       string
	Balance float64
}

func (h *HuobiApi) getAccountId() (string, error) {
	byteArray, err := h.privateApi("GET", "/v1/account/accounts", &url.Values{})
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

func (h *HuobiApi) CompleteBalances() (map[string]*models.Balance, error) {
	accountId, err := h.getAccountId()
	if err != nil {
		return nil, err
	}
	params := &url.Values{}
	params.Set("account-id", accountId)
	byteArray, err := h.privateApi("GET", "/v1/account/accounts/"+accountId+"/balance", params)
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

type HuobiActiveOrderResponse struct {
	response   []byte
	Trading    string
	Settlement string
	err        error
}

func (h *HuobiApi) ActiveOrders() ([]*models.Order, error) {
	return nil, errors.New("not implemented")
}

func (h *HuobiApi) Order(trading string, settlement string, ordertype models.OrderType, price float64, amount float64) (string, error) {
	accountId, err := h.getAccountId()
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
	byteArray, err := h.privateApi("GET", "/v1/order/orders/place", params)
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

func (h *HuobiApi) Transfer(typ string, addr string, amount float64, additionalFee float64) error {
	params := &url.Values{}
	amountStr := strconv.FormatFloat(amount, 'f', 4, 64)
	additionalFeeStr := strconv.FormatFloat(additionalFee, 'f', 4, 64)
	params.Set("address", addr)
	params.Set("amount", amountStr)
	params.Set("currency", typ)
	params.Set("fee", additionalFeeStr)
	_, err := h.privateApi("GET", "/v1/dw/withdraw/api/create", params)
	return err
}

func (h *HuobiApi) CancelOrder(orderNumber string, _ string) error {
	params := &url.Values{}
	params.Set("order-id", orderNumber)
	_, err := h.privateApi("POST", "/v1/order/orders/"+orderNumber+"/submitcancel", params)
	if err != nil {
		return errors.Wrapf(err, "failed to cancel order")
	}
	return nil
}

func (h *HuobiApi) IsOrderFilled(orderNumber string, _ string) (bool, error) {
	params := &url.Values{}
	params.Set("order-id", orderNumber)
	bs, err := h.privateApi("POST", "/v1/order/orders/"+orderNumber, params)
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

func (h *HuobiApi) Address(c string) (string, error) {
	params := &url.Values{}
	params.Set("currency", strings.ToLower(c))
	params.Set("type", "deposit")

	bs, err := h.privateApi("GET", "/v1/dw/deposit-virtual/addresses", params)
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
