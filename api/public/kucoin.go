package public

import (
	"net/http"
	"sync"
	"time"

	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"io/ioutil"
	url2 "net/url"
	"strings"
)

const (
	KUCOIN_BASE_URL = "https://api.kucoin.com"
)

func NewKucoinPublicApi() (*KucoinApi, error) {
	api := &KucoinApi{
		BaseURL:                    KUCOIN_BASE_URL,
		RateCacheDuration:          30 * time.Second,
		rateMap:                    nil,
		volumeMap:                  nil,
		rateLastUpdated:            time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyPairsCacheDuration: 7 * 24 * time.Hour,
		currencyPairsLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),

		HttpClient: &http.Client{Timeout: time.Duration(5) * time.Second},
		rt:         &http.Transport{},

		m:         new(sync.Mutex),
		rateM:     new(sync.Mutex),
		currencyM: new(sync.Mutex),
	}
	api.fetchSettlements()
	return api, nil
}

type KucoinApi struct {
	BaseURL                    string
	RateCacheDuration          time.Duration
	rateLastUpdated            time.Time
	volumeMap                  map[string]map[string]float64
	rateMap                    map[string]map[string]float64
	currencyPairs              []models.CurrencyPair
	CurrencyPairsCacheDuration time.Duration
	currencyPairsLastUpdated   time.Time

	HttpClient *http.Client
	rt         http.RoundTripper

	settlements []string

	m         *sync.Mutex
	rateM     *sync.Mutex
	currencyM *sync.Mutex
}

func (h *KucoinApi) publicApiUrl(command string) string {
	return h.BaseURL + command
}

func (h *KucoinApi) fetchSettlements() error {
	h.settlements = []string{"BTC", "ETH", "NEO", "USDT", "KCS"}
	return nil
}

type KucoinTickResponse struct {
	response   []byte
	Trading    string
	Settlement string
	err        error
}

func requestGetAsChrome(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return req, err
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; MAFSJS; rv:11.0) like Gecko")
	return req, err
}

func (h *KucoinApi) fetchRate() error {
	url := h.publicApiUrl("/v1/open/tick")
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
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObjectArray("data")
	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}
	rateMap := make(map[string]map[string]float64)
	volumeMap := make(map[string]map[string]float64)
	for _, v := range data {
		trading, err := v.GetString("coinType")
		if err != nil {
			continue
		}
		settlement, err := v.GetString("coinTypePair")
		if err != nil {
			continue
		}
		lastf, err := v.GetFloat64("lastDealPrice")
		if err != nil {
			continue
		}
		volumef, err := v.GetFloat64("vol")
		if err != nil {
			continue
		}
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

func (h *KucoinApi) RateMap() (map[string]map[string]float64, error) {
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

func (h *KucoinApi) VolumeMap() (map[string]map[string]float64, error) {
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

func (h *KucoinApi) CurrencyPairs() ([]models.CurrencyPair, error) {
	h.currencyM.Lock()
	defer h.currencyM.Unlock()
	if len(h.currencyPairs) != 0 {
		return h.currencyPairs, nil
	}
	h.fetchSettlements()
	currecyPairs := make([]models.CurrencyPair, 0)
	url := h.publicApiUrl("/v1/open/tick")
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
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObjectArray("data")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	for _, v := range data {
		trading, err := v.GetString("coinType")
		if err != nil {
			continue
		}
		settlement, err := v.GetString("coinTypePair")
		if err != nil {
			continue
		}
		currecyPairs = append(currecyPairs, models.CurrencyPair{
			Trading:    trading,
			Settlement: settlement,
		})
	}
	h.currencyPairs = currecyPairs
	return currecyPairs, nil
}

func (h *KucoinApi) Volume(trading string, settlement string) (float64, error) {
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

func (h *KucoinApi) Rate(trading string, settlement string) (float64, error) {
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

func (h *KucoinApi) FrozenCurrency() ([]string, error) {
	url := h.publicApiUrl("/v1/market/open/coins")
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
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return []string{}, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObjectArray("data")
	if err != nil {
		return []string{}, errors.Wrapf(err, "failed to parse json")
	}
	var frozenCurrencies []string
	for _, v := range data {
		enableWithdraw, err := v.GetBoolean("enableWithdraw")
		if err != nil {
			return []string{}, errors.Wrapf(err, "failed to parse isTrading")
		}
		enableDeposit, err := v.GetBoolean("enableDeposit")
		if err != nil {
			return []string{}, errors.Wrapf(err, "failed to parse isTrading")
		}
		trading, err := v.GetString("coin")
		if err != nil {
			return []string{}, errors.Wrapf(err, "failed to parse object")
		}
		if !enableWithdraw || !enableDeposit {
			frozenCurrencies = append(frozenCurrencies, trading)
		}
	}
	return frozenCurrencies, nil
}

func (h *KucoinApi) Board(trading string, settlement string) (board *models.Board, err error) {
	args := url2.Values{}
	args.Add("symbol", strings.ToUpper(trading)+"-"+strings.ToUpper(settlement))
	args.Add("group", "1")
	url := h.publicApiUrl("/v1/open/orders?") + args.Encode()
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
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json from byte array")
	}
	data, err := json.GetObject("data")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json by key tick")
	}
	sells, err := data.GetValueArray("SELL")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json bids")
	}
	buys, err := data.GetValueArray("BUY")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json asks")
	}
	bids := make([]models.BoardOrder, 0)
	asks := make([]models.BoardOrder, 0)
	for _, v := range buys {
		s, err := v.Array()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse array")
		}
		price, err := s[0].Float64()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		amount, err := s[1].Float64()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse amount")
		}
		bids = append(bids, models.BoardOrder{
			Price:  price,
			Amount: amount,
			Type:   models.Bid,
		})
	}
	for _, v := range sells {
		s, err := v.Array()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse array")
		}
		price, err := s[0].Float64()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		amount, err := s[1].Float64()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse amount")
		}
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
	return board, nil
}
