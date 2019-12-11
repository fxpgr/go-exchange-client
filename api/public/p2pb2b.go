package public

import (
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	P2PB2B_BASE_URL = "https://api.p2pb2b.io/api/v1"
)

type P2pb2bApiConfig struct {
}

func NewP2pb2bPublicApi() (*P2pb2bApi, error) {
	api := &P2pb2bApi{
		BaseURL:           P2PB2B_BASE_URL,
		RateCacheDuration: 3 * time.Second,
		rateMap:           nil,
		volumeMap:         nil,
		orderBookTickMap:  nil,
		precisionMap:      nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		boardCache:        cache.New(3*time.Second, 1*time.Second),
		HttpClient:        &http.Client{},

		m: new(sync.Mutex),
	}
	api.fetchSettlements()
	return api, nil
}

type P2pb2bApi struct {
	BaseURL           string
	RateCacheDuration time.Duration
	volumeMap         map[string]map[string]float64
	rateMap           map[string]map[string]float64
	orderBookTickMap  map[string]map[string]models.OrderBookTick
	precisionMap      map[string]map[string]models.Precisions
	rateLastUpdated   time.Time
	boardCache        *cache.Cache
	HttpClient        *http.Client

	settlements []string

	m *sync.Mutex
	c *P2pb2bApiConfig
}

func (h *P2pb2bApi) SetTransport(transport http.RoundTripper) error {
	h.HttpClient.Transport = transport
	return nil
}

func (h *P2pb2bApi) publicApiUrl(command string) string {
	return h.BaseURL + "/" + command
}

func (h *P2pb2bApi) fetchSettlements() error {
	settlements := make([]string, 0)
	url := h.publicApiUrl("public/products")
	resp, err := h.HttpClient.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	json, err := gabs.ParseJSON(byteArray)

	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}
	pairs, err := json.Path("result").Children()
	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}
	for _, v := range pairs {
		settlement := v.Path("toSymbol").String()
		settlements = append(settlements, settlement)
	}
	m := make(map[string]bool)
	uniq := []string{}
	for _, ele := range settlements {
		if !m[ele] {
			m[ele] = true
			uniq = append(uniq, ele)
		}
	}
	h.settlements = uniq
	return nil
}

func (h *P2pb2bApi) fetchPrecision() error {
	if h.precisionMap != nil {
		return nil
	}
	h.precisionMap = make(map[string]map[string]models.Precisions)

	url := h.publicApiUrl("public/tickers")
	resp, err := h.HttpClient.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	json, err := gabs.ParseJSON(byteArray)
	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}
	rateMap, err := json.Path("result").ChildrenMap()
	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}
	for k, v := range rateMap {
		coins := strings.Split(k, "_")
		if len(coins) != 2 {
			continue
		}
		trading, settlement := coins[0], coins[1]
		if settlement == "" || trading == "" {
			continue
		}
		last, ok := v.Path("ticker").Path("last").Data().(string)
		if !ok {
			continue
		}
		// update rate
		_, err = strconv.ParseFloat(last, 64)
		if err != nil {
			return err
		}

		// update volume
		volume, ok := v.Path("ticker").Path("vol").Data().(string)
		if !ok {
			continue
		}
		_, err = strconv.ParseFloat(volume, 64)
		if err != nil {
			return err
		}

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

func (h *P2pb2bApi) fetchRate() error {
	h.rateMap = make(map[string]map[string]float64)
	h.volumeMap = make(map[string]map[string]float64)
	h.orderBookTickMap = make(map[string]map[string]models.OrderBookTick)
	url := h.publicApiUrl("public/tickers")
	resp, err := h.HttpClient.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	json, err := gabs.ParseJSON(byteArray)
	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}
	rateMap, err := json.Path("result").ChildrenMap()
	if err != nil {
		return errors.Wrapf(err, "failed to parse json children map")
	}
	for k, v := range rateMap {
		coins := strings.Split(k, "_")
		if len(coins) != 2 {
			continue
		}
		trading, settlement := coins[0], coins[1]
		if settlement == "" || trading == "" {
			continue
		}
		last, ok := v.Path("ticker").Path("last").Data().(string)
		if !ok {
			continue
		}
		// update rate
		lastf, err := strconv.ParseFloat(last, 64)
		if err != nil {
			return err
		}

		m, ok := h.rateMap[trading]
		if !ok {
			m = make(map[string]float64)
			h.rateMap[trading] = m
		}
		m[settlement] = lastf

		// update volume
		volume, ok := v.Path("ticker").Path("vol").Data().(string)
		if !ok {
			continue
		}
		volumef, err := strconv.ParseFloat(volume, 64)
		if err != nil {
			return err
		}

		m, ok = h.volumeMap[trading]
		if !ok {
			m = make(map[string]float64)
			h.volumeMap[trading] = m
		}
		m[settlement] = volumef

		// update orderBookTick
		askPrice, ok := v.Path("ticker").Path("ask").Data().(string)
		if !ok {
			continue
		}
		askPricef, err := strconv.ParseFloat(askPrice, 64)
		if err != nil {
			return err
		}
		bidPrice, ok := v.Path("ticker").Path("bid").Data().(string)
		if !ok {
			continue
		}
		bidPricef, err := strconv.ParseFloat(bidPrice, 64)
		if err != nil {
			return err
		}

		n, ok := h.orderBookTickMap[trading]
		if !ok {
			n = make(map[string]models.OrderBookTick)
			h.orderBookTickMap[trading] = n
		}
		n[settlement] = models.OrderBookTick{
			BestAskPrice: askPricef,
			BestBidPrice: bidPricef,
		}
	}

	return nil
}

func (h *P2pb2bApi) OrderBookTickMap() (map[string]map[string]models.OrderBookTick, error) {
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
	return h.orderBookTickMap, nil
}

func (h *P2pb2bApi) RateMap() (map[string]map[string]float64, error) {
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

func (h *P2pb2bApi) VolumeMap() (map[string]map[string]float64, error) {
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

func (h *P2pb2bApi) CurrencyPairs() ([]models.CurrencyPair, error) {
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

	var pairs []models.CurrencyPair
	for trading, m := range h.rateMap {
		for settlement := range m {
			pair := models.CurrencyPair{
				Trading:    trading,
				Settlement: settlement,
			}
			pairs = append(pairs, pair)
		}
	}

	return pairs, nil
}

func (h *P2pb2bApi) Precise(trading string, settlement string) (*models.Precisions, error) {
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

func (h *P2pb2bApi) Volume(trading string, settlement string) (float64, error) {
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

func (h *P2pb2bApi) Rate(trading string, settlement string) (float64, error) {
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

func (h *P2pb2bApi) FrozenCurrency() ([]string, error) {
	var frozens []string
	return frozens, nil
}

func (h *P2pb2bApi) Board(trading string, settlement string) (board *models.Board, err error) {
	c, found := h.boardCache.Get(trading + "_" + settlement)
	if found {
		return c.(*models.Board), nil
	}
	url := h.publicApiUrl("public/depth/result?market=" + trading + "_" + settlement + "&limit=100")
	resp, err := h.HttpClient.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	value := gjson.Parse(string(byteArray))
	if value.Get("code").String() == "-1003" {
		return nil, errors.Errorf("ip banned %s", url)
	}
	bidsJson := value.Get("bids").Array()
	asksJson := value.Get("asks").Array()

	asks := make([]models.BoardBar, 0)
	bids := make([]models.BoardBar, 0)
	for _, bidJson := range bidsJson {
		price := bidJson.Array()[0].Float()
		amount := bidJson.Array()[1].Float()
		bidBoardBar := models.BoardBar{
			Type:   models.Bid,
			Price:  price,
			Amount: amount,
		}
		bids = append(bids, bidBoardBar)
	}
	for _, askJson := range asksJson {
		price := askJson.Array()[0].Float()
		amount := askJson.Array()[1].Float()
		askBoardBar := models.BoardBar{
			Type:   models.Ask,
			Price:  price,
			Amount: amount,
		}
		asks = append(asks, askBoardBar)
	}
	board = &models.Board{
		Asks: asks,
		Bids: bids,
	}
	h.boardCache.Set(trading+"_"+settlement, board, cache.DefaultExpiration)
	return board, nil
}
