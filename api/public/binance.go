package public

import (
	"net/http"
	"sync"
	"time"

	"io/ioutil"
	url2 "net/url"
	"strings"

	"github.com/fxpgr/go-exchange-client/models"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	BINANCE_BASE_URL = "https://api.binance.com"
)

func NewBinancePublicApi() (*BinanceApi, error) {
	cli := http.DefaultClient
	cli.Timeout = 5 * time.Second
	api := &BinanceApi{
		BaseURL:           BINANCE_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		boardCache:        cache.New(3*time.Second, 1*time.Second),
		HttpClient:        cli,

		m:         new(sync.Mutex),
		rateM:     new(sync.Mutex),
		currencyM: new(sync.Mutex),
	}
	api.fetchSettlements()
	return api, nil
}

type BinanceApi struct {
	BaseURL           string
	RateCacheDuration time.Duration
	rateLastUpdated   time.Time
	volumeMap         map[string]map[string]float64
	rateMap           map[string]map[string]float64
	precisionMap      map[string]map[string]models.Precisions
	boardCache        *cache.Cache
	currencyPairs     []models.CurrencyPair

	HttpClient *http.Client

	settlements []string

	m         *sync.Mutex
	rateM     *sync.Mutex
	currencyM *sync.Mutex
}

func (h *BinanceApi) publicApiUrl(command string) string {
	return h.BaseURL + command
}

func (h *BinanceApi) fetchSettlements() error {
	h.settlements = []string{"BTC", "ETH", "NEO", "USDT", "KCS"}
	return nil
}

type BinanceTickResponse struct {
	response   []byte
	Trading    string
	Settlement string
	err        error
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

func (h *BinanceApi) fetchRate() error {
	url := h.publicApiUrl("/api/v1/ticker/24hr")
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
	currencyPairs, err := h.CurrencyPairs()
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}

	rateMap := make(map[string]map[string]float64)
	volumeMap := make(map[string]map[string]float64)
	for _, v := range value.Array() {
		var trading, settlement string
		symbol := v.Get("symbol").Str
		for _, c := range currencyPairs {
			if symbol == c.Trading+c.Settlement {
				trading = c.Trading
				settlement = c.Settlement
				break
			}
		}

		lastf := v.Get("lastPrice").Float()
		volumef := v.Get("volume").Float()
		h.rateM.Lock()
		n, ok := volumeMap[trading]
		if !ok {
			n = make(map[string]float64)
			volumeMap[trading] = n
		}
		n[settlement] = volumef
		m, ok := rateMap[trading]
		if !ok {
			m = make(map[string]float64)
			rateMap[trading] = m
		}
		m[settlement] = lastf
		h.rateM.Unlock()
	}
	h.rateMap = rateMap
	h.volumeMap = volumeMap
	return nil
}

func (h *BinanceApi) RateMap() (map[string]map[string]float64, error) {
	h.m.Lock()
	defer h.m.Unlock()
	now := time.Now()
	if now.Sub(h.rateLastUpdated) >= h.RateCacheDuration {
		err := h.fetchRate()
		if err != nil {
			return nil, err
		}
		h.rateLastUpdated = now
	}
	return h.rateMap, nil
}

func (h *BinanceApi) Precise(trading string, settlement string) (*models.Precisions, error) {
	if trading == settlement {
		return &models.Precisions{}, nil
	}

	h.fetchPrecision()
	if m, ok := h.precisionMap[trading]; !ok {
		return &models.Precisions{}, errors.Errorf("%s/%s", trading, settlement)
	} else if precisions, ok := m[settlement]; !ok {
		return &models.Precisions{}, errors.Errorf("%s/%s", trading, settlement)
	} else {
		return &precisions, nil
	}
}

func (h *BinanceApi) VolumeMap() (map[string]map[string]float64, error) {
	h.m.Lock()
	defer h.m.Unlock()
	now := time.Now()
	if now.Sub(h.rateLastUpdated) >= h.RateCacheDuration {
		err := h.fetchRate()
		if err != nil {
			return nil, err
		}
		h.rateLastUpdated = now
	}
	return h.volumeMap, nil
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

func (h *BinanceApi) Volume(trading string, settlement string) (float64, error) {
	h.m.Lock()
	defer h.m.Unlock()

	now := time.Now()
	if now.Sub(h.rateLastUpdated) >= h.RateCacheDuration {
		err := h.fetchRate()
		if err != nil {
			return 0, err
		}
		h.rateLastUpdated = now
	}
	if m, ok := h.volumeMap[trading]; !ok {
		return 0, errors.Errorf("%s/%s", trading, settlement)
	} else if volume, ok := m[settlement]; !ok {
		return 0, errors.Errorf("%s/%s", trading, settlement)
	} else {
		return volume, nil
	}
}

func (h *BinanceApi) Rate(trading string, settlement string) (float64, error) {
	h.m.Lock()
	defer h.m.Unlock()

	if trading == settlement {
		return 1, nil
	}

	now := time.Now()
	if now.Sub(h.rateLastUpdated) >= h.RateCacheDuration {
		err := h.fetchRate()
		if err != nil {
			return 0, err
		}
		h.rateLastUpdated = now
	}
	if m, ok := h.rateMap[trading]; !ok {
		return 0, errors.Errorf("%s/%s", trading, settlement)
	} else if rate, ok := m[settlement]; !ok {
		return 0, errors.Errorf("%s/%s", trading, settlement)
	} else {
		return rate, nil
	}
}

func (h *BinanceApi) FrozenCurrency() ([]string, error) {
	if len(h.currencyPairs) != 0 {
		return []string{}, nil
	}
	url := h.publicApiUrl("/api/v1/exchangeInfo")
	req, err := requestGetAsChrome(url)
	if err != nil {
		return []string{}, errors.Wrapf(err, "failed to fetch %s", url)
	}
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return []string{}, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()
	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []string{}, errors.Wrapf(err, "failed to fetch %s", url)
	}
	value := gjson.ParseBytes(byteArray)
	symbols := value.Get("symbols").Array()

	var frozenCurrencies []string
	for _, v := range symbols {
		if v.Get("status").Str != "TRADING" {
			trading := v.Get("baseAsset").Str
			settlement := v.Get("quoteAsset").Str
			frozenCurrencies = append(frozenCurrencies, trading, settlement)
		}
	}
	m := make(map[string]bool)
	uniq := []string{}
	for _, ele := range frozenCurrencies {
		if !m[ele] {
			m[ele] = true
			uniq = append(uniq, ele)
		}
	}
	return uniq, nil
}

func (h *BinanceApi) Board(trading string, settlement string) (board *models.Board, err error) {
	c, found := h.boardCache.Get(trading + "_" + settlement)
	if found {
		return c.(*models.Board), nil
	}
	args := url2.Values{}
	args.Add("symbol", strings.ToUpper(trading)+strings.ToUpper(settlement))
	args.Add("limit", "1000")
	url := h.publicApiUrl("/api/v1/depth?") + args.Encode()
	req, err := requestGetAsChrome(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	value := gjson.ParseBytes(byteArray)
	bidsArray := value.Get("bids").Array()
	asksArray := value.Get("asks").Array()

	bids := make([]models.BoardOrder, 0)
	asks := make([]models.BoardOrder, 0)
	for _, v := range bidsArray {
		price := v.Array()[0].Float()
		amount := v.Array()[1].Float()
		bids = append(bids, models.BoardOrder{
			Price:  price,
			Amount: amount,
			Type:   models.Bid,
		})
	}
	for _, v := range asksArray {
		price := v.Array()[0].Float()
		amount := v.Array()[1].Float()
		asks = append(asks, models.BoardOrder{
			Price:  price,
			Amount: amount,
			Type:   models.Ask,
		})
	}
	board = &models.Board{
		Bids: bids,
		Asks: asks,
	}
	h.boardCache.Set(trading+"_"+settlement, board, cache.DefaultExpiration)
	return board, nil
}
