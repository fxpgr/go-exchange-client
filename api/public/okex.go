package public

import (
	"net/http"
	"sync"
	"time"

	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"io/ioutil"
	url2 "net/url"
	"strconv"
	"strings"
)

const (
	OKEX_BASE_URL = "https://www.okex.com"
)

func NewOkexPublicApi() (*OkexApi, error) {
	api := &OkexApi{
		BaseURL:                    OKEX_BASE_URL,
		RateCacheDuration:          30 * time.Second,
		rateMap:                    nil,
		volumeMap:                  nil,
		rateLastUpdated:            time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyPairsCacheDuration: 7 * 24 * time.Hour,
		currencyPairsLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),

		HttpClient: &http.Client{},
		rt:         &http.Transport{},

		m:         new(sync.Mutex),
		rateM:     new(sync.Mutex),
		currencyM: new(sync.Mutex),
	}
	api.fetchSettlements()
	return api, nil
}

type OkexApi struct {
	BaseURL                    string
	RateCacheDuration          time.Duration
	rateLastUpdated            time.Time
	volumeMap                  map[string]map[string]float64
	rateMap                    map[string]map[string]float64
	precisionMap               map[string]map[string]models.Precisions
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

func (h *OkexApi)SetTransport(transport http.RoundTripper) error {
	h.HttpClient.Transport = transport
	return nil
}
func (h *OkexApi) publicApiUrl(command string) string {
	return h.BaseURL + command
}

func (h *OkexApi) fetchSettlements() error {
	pairs, err := h.CurrencyPairs()
	if err != nil {
		return errors.Wrap(err, "failed to fetch settlements")
	}
	m := make(map[string]bool)
	uniq := []string{}
	for _, ele := range pairs {
		if !m[ele.Settlement] {
			m[ele.Settlement] = true
			uniq = append(uniq, ele.Settlement)
		}
	}
	h.settlements = uniq
	return nil
}

func (h *OkexApi) fetchPrecision() error {
	if h.precisionMap != nil {
		return nil
	}
	h.precisionMap = make(map[string]map[string]models.Precisions)

	url := h.publicApiUrl("/v2/spot/markets/tickers")
	resp, err := h.HttpClient.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	value := gjson.Parse(string(byteArray))

	for _, v := range value.Get("data").Array() {
		last := v.Get("last").Str
		volume := v.Get("volume").Str

		pairString := v.Get("symbol").Str
		currencies := strings.Split(pairString, "_")
		if len(currencies) != 2 {
			continue
		}
		trading := currencies[0]
		settlement := currencies[1]
		m, ok := h.precisionMap[trading]
		if !ok {
			m = make(map[string]models.Precisions)
			h.precisionMap[trading] = m
		}
		m[settlement] = models.Precisions{
			PricePrecision:  Precision(last),
			AmountPrecision: Precision(volume),
		}
	}
	return nil
}

type OkexTickResponse struct {
	response   []byte
	Trading    string
	Settlement string
	err        error
}

func (h *OkexApi) fetchRate() error {
	h.rateMap = make(map[string]map[string]float64)
	h.volumeMap = make(map[string]map[string]float64)
	url := h.publicApiUrl("/v2/spot/markets/tickers")
	resp, err := h.HttpClient.Get(url)
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
	for _, v := range data {
		lastString, err := v.GetString("last")
		if err != nil {
			return errors.Wrapf(err, "failed to parse quote")
		}
		lastf, err := strconv.ParseFloat(lastString, 64)
		if err != nil {
			return errors.Wrapf(err, "failed to parse quote")
		}

		volumeString, err := v.GetString("volume")
		if err != nil {
			return errors.Wrapf(err, "failed to parse quote")
		}
		volumef, err := strconv.ParseFloat(volumeString, 64)
		if err != nil {
			return errors.Wrapf(err, "failed to parse quote")
		}

		pairString, err := v.GetString("symbol")
		if err != nil {
			return errors.Wrapf(err, "failed to parse quote")
		}
		currencies := strings.Split(pairString, "_")
		if len(currencies) != 2 {
			continue
		}
		trading := currencies[0]
		settlement := currencies[1]
		m, ok := h.rateMap[trading]
		if !ok {
			m = make(map[string]float64)
			h.rateMap[trading] = m
		}
		m[settlement] = lastf
		m, ok = h.volumeMap[trading]
		if !ok {
			m = make(map[string]float64)
			h.volumeMap[trading] = m
		}
		m[settlement] = volumef
	}
	return nil
}

func (h *OkexApi) RateMap() (map[string]map[string]float64, error) {
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

func (h *OkexApi) VolumeMap() (map[string]map[string]float64, error) {
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

func (h *OkexApi) CurrencyPairs() ([]models.CurrencyPair, error) {
	h.currencyM.Lock()
	defer h.currencyM.Unlock()
	if len(h.currencyPairs) != 0 {
		return h.currencyPairs, nil
	}
	url := h.publicApiUrl("/v2/markets/products")
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
	var pairs []models.CurrencyPair
	for _, v := range data {
		pairString, err := v.GetString("symbol")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse quote")
		}
		currencies := strings.Split(pairString, "_")
		if len(currencies) != 2 {
			continue
		}
		pair := models.CurrencyPair{
			Trading:    strings.ToUpper(currencies[0]),
			Settlement: strings.ToUpper(currencies[1]),
		}
		pairs = append(pairs, pair)
	}
	h.currencyPairs = pairs
	return pairs, nil
}

func (h *OkexApi) Precise(trading string, settlement string) (*models.Precisions, error) {
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

func (h *OkexApi) Volume(trading string, settlement string) (float64, error) {
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

func (h *OkexApi) Rate(trading string, settlement string) (float64, error) {
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

func (h *OkexApi) FrozenCurrency() ([]string, error) {
	var frozens []string
	url := h.publicApiUrl("/v2/markets/currencies")
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
	for _, v := range data {
		withdrawEnabled, err := v.GetBoolean("withdrawable")
		if err != nil {
			continue
		}
		depositEnabled, err := v.GetBoolean("rechargeable")
		if err != nil {
			continue
		}
		if withdrawEnabled || depositEnabled {
			continue
		}
		currencyName, err := v.GetString("symbol")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse quote")
		}
		frozens = append(frozens, currencyName)
	}
	return frozens, nil
}

func (h *OkexApi) Board(trading string, settlement string) (board *models.Board, err error) {
	args := url2.Values{}
	args.Add("size", "200")
	method := "/v2/markets/" + strings.ToLower(trading) + "_" + strings.ToLower(settlement) + "/depth?" + args.Encode()
	url := h.publicApiUrl(method)
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
		return nil, errors.Wrapf(err, "failed to parse json from byte array")
	}
	data, err := json.GetObject("data")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json by key tick")
	}
	jsonBids, err := data.GetObjectArray("bids")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json bids")
	}
	jsonAsks, err := data.GetObjectArray("asks")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json asks")
	}
	bids := make([]models.BoardOrder, 0)
	asks := make([]models.BoardOrder, 0)
	for _, v := range jsonBids {
		priceString, err := v.GetString("price")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse array")
		}
		sizeString, err := v.GetString("totalSize")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse array")
		}
		pricef, err := strconv.ParseFloat(priceString, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		sizef, err := strconv.ParseFloat(sizeString, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse size")
		}
		bids = append(bids, models.BoardOrder{
			Price:  pricef,
			Amount: sizef,
			Type:   models.Bid,
		})
	}
	for _, v := range jsonAsks {

		priceString, err := v.GetString("price")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse array")
		}
		sizeString, err := v.GetString("totalSize")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse array")
		}
		pricef, err := strconv.ParseFloat(priceString, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		sizef, err := strconv.ParseFloat(sizeString, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse size")
		}
		asks = append(asks, models.BoardOrder{
			Price:  pricef,
			Amount: sizef,
			Type:   models.Ask,
		})
	}
	board = &models.Board{
		Bids: bids,
		Asks: asks,
	}
	return board, nil
}
