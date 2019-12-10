package private

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
)

const (
	HUOBI_BASE_URL = "https://api.huobi.pro"
)

func NewHuobiApi(apikey func() (string, error), apisecret func() (string, error)) (*HuobiApi, error) {
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
		ApiKeyFunc:        apikey,
		SecretKeyFunc:     apisecret,
		settlements:       uniq,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		rt:                &http.Transport{},

		m: new(sync.Mutex),
	}, nil
}

type HuobiApi struct {
	ApiKeyFunc        func() (string, error)
	SecretKeyFunc     func() (string, error)
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

	apiKey, err := h.ApiKeyFunc()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	secretKey, err := h.SecretKeyFunc()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	params.Set("AccessKeyId", apiKey)
	params.Set("SignatureMethod", "HmacSHA256")
	params.Set("SignatureVersion", "2")
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05"))
	domain := strings.Replace(h.BaseURL, "https://", "", len(h.BaseURL))
	payload := fmt.Sprintf("%s\n%s\n%s\n%s", method, domain, path, params.Encode())
	sign, _ := GetParamHmacSHA256Base64Sign(secretKey, payload)
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
	transferFeeMap := huobiTransferFeeSyncMap{make(huobiTransferFeeMap), new(sync.Mutex)}
	transferFeeMap.Set("ZRX", 10)
	transferFeeMap.Set("ACT", 0.01)
	transferFeeMap.Set("ADX", 0.002)
	transferFeeMap.Set("ELF", 0.002)
	transferFeeMap.Set("AIDOC", 143)
	transferFeeMap.Set("AST", 0.01)
	transferFeeMap.Set("SOC", 0.01)
	transferFeeMap.Set("APPC", 0.002)
	transferFeeMap.Set("ABT", 0.002)
	transferFeeMap.Set("BAT", 0.02)
	transferFeeMap.Set("BFT", 0.01)
	transferFeeMap.Set("BIX", 0.01)
	transferFeeMap.Set("BTC", 0.01)
	transferFeeMap.Set("BCH", 92)
	transferFeeMap.Set("BCD", 0.002)
	transferFeeMap.Set("BTG", 5)
	transferFeeMap.Set("BT2", 0.002)
	transferFeeMap.Set("BIFI", 8.5)
	transferFeeMap.Set("BCX", 0.002)
	transferFeeMap.Set("BT1", 0.05)
	transferFeeMap.Set("BTS", 0.002)
	transferFeeMap.Set("BLZ", 0.002)
	transferFeeMap.Set("BTM", 0.002)
	transferFeeMap.Set("ADA", 0.001)
	transferFeeMap.Set("LINK", 296.3)
	transferFeeMap.Set("CHAT", 0.01)
	transferFeeMap.Set("CVC", 0.002)
	transferFeeMap.Set("CMT", 0.001)
	transferFeeMap.Set("DASH", 1.5)
	transferFeeMap.Set("DTA", 0.01)
	transferFeeMap.Set("DAT", 0.002)
	transferFeeMap.Set("MANA", 1)
	transferFeeMap.Set("DCR", 1082)
	transferFeeMap.Set("DBC", 0.002)
	transferFeeMap.Set("DGB", 0.002)
	transferFeeMap.Set("DGD", 2)
	transferFeeMap.Set("EKO", 0.01)
	transferFeeMap.Set("ELA", 4)
	transferFeeMap.Set("ENG", 0.01)
	transferFeeMap.Set("EOS", 8.6)
	transferFeeMap.Set("ETH", 0.002)
	transferFeeMap.Set("ETC", 0.002)
	transferFeeMap.Set("ETF", 0.002)
	transferFeeMap.Set("EVX", 0.2)
	transferFeeMap.Set("GAS", 4)
	transferFeeMap.Set("GNX", 1294)
	transferFeeMap.Set("GNT", 0.01)
	transferFeeMap.Set("GXS", 0.5)
	transferFeeMap.Set("HSR", 30)
	transferFeeMap.Set("HT", 0.01)
	transferFeeMap.Set("ICX", 0.002)
	transferFeeMap.Set("IOST", 1)
	transferFeeMap.Set("ITC", 7)
	transferFeeMap.Set("MIOTA", 0.01)
	transferFeeMap.Set("KNC", 0.04)
	transferFeeMap.Set("LET", 0.002)
	transferFeeMap.Set("LSK", 3)
	transferFeeMap.Set("LTC", 20)
	transferFeeMap.Set("LUN", 0.1)
	transferFeeMap.Set("MTX", 0.002)
	transferFeeMap.Set("MTN", 0.1)
	transferFeeMap.Set("MDS", 0.0001)
	transferFeeMap.Set("MTL", 0.01)
	transferFeeMap.Set("MCO", 0.01)
	transferFeeMap.Set("XMR", 2)
	transferFeeMap.Set("NAS", 0.05)
	transferFeeMap.Set("XEM", 0.01)
	transferFeeMap.Set("NEO", 1)
	transferFeeMap.Set("OCN", 0.0001)
	transferFeeMap.Set("OMG", 0.01)
	transferFeeMap.Set("ONT", 0.01)
	transferFeeMap.Set("POLY", 0.01)
	transferFeeMap.Set("POWR", 0.01)
	transferFeeMap.Set("PRO", 0.002)
	transferFeeMap.Set("QASH", 1)
	transferFeeMap.Set("QTUM", 10)
	transferFeeMap.Set("QSP", 0.001)
	transferFeeMap.Set("QUN", 7)
	transferFeeMap.Set("RDN", 0.1)
	transferFeeMap.Set("RPX", 0.002)
	transferFeeMap.Set("REQ", 0.001)
	transferFeeMap.Set("RCN", 0.01)
	transferFeeMap.Set("XRP", 0.01)
	transferFeeMap.Set("RUFF", 10)
	transferFeeMap.Set("SALT", 0.5)
	transferFeeMap.Set("OST", 0.01)
	transferFeeMap.Set("SRN", 0.01)
	transferFeeMap.Set("SMT", 0.1)
	transferFeeMap.Set("SNT", 61)
	transferFeeMap.Set("STEEM", 0.01)
	transferFeeMap.Set("XLM", 3.6)
	transferFeeMap.Set("STK", 1)
	transferFeeMap.Set("STORJ", 0.2)
	transferFeeMap.Set("SNC", 0.5)
	transferFeeMap.Set("SBTC", 0.002)
	transferFeeMap.Set("SWFTC", 1.7)
	transferFeeMap.Set("PAY", 0.002)
	transferFeeMap.Set("USDT", 0.01)
	transferFeeMap.Set("THETA", 0.01)
	transferFeeMap.Set("TNT", 0.01)
	transferFeeMap.Set("TNB", 88)
	transferFeeMap.Set("TRX", 0.0005)
	transferFeeMap.Set("UTK", 0.001)
	transferFeeMap.Set("VEN", 0.01)
	transferFeeMap.Set("XVG", 0.1)
	transferFeeMap.Set("WTC", 1)
	transferFeeMap.Set("WAN", 15)
	transferFeeMap.Set("WAVES", 10)
	transferFeeMap.Set("WAX", 0.01)
	transferFeeMap.Set("WIC", 0.002)
	transferFeeMap.Set("WPR", 10)
	transferFeeMap.Set("YEE", 0.002)
	transferFeeMap.Set("ZEC", 0.6)
	transferFeeMap.Set("ZLA", 0.01)
	transferFeeMap.Set("ZIL", 0.01)
	return transferFeeMap.GetAll(), nil
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

func (h *HuobiApi) CompleteBalance(coin string) (*models.Balance, error) {
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
		if currency == coin {
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
	return m[coin], nil
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

func (h *HuobiApi) CancelOrder(trading string, settlement string,
	ordertype models.OrderType, orderNumber string) error {
	params := &url.Values{}
	params.Set("order-id", orderNumber)
	_, err := h.privateApi("POST", "/v1/order/orders/"+orderNumber+"/submitcancel", params)
	if err != nil {
		return errors.Wrapf(err, "failed to cancel order")
	}
	return nil
}

func (h *HuobiApi) IsOrderFilled(trading string, settlement string, orderNumber string) (bool, error) {
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
