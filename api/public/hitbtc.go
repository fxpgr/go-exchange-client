package public

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-ccex-api-client/models"
	"github.com/pkg/errors"
	"io/ioutil"
	"strings"
)

const (
	HITBTC_BASE_URL = "https://api.hitbtc.com/api/2"
)

type HitbtcApiConfig struct {
}

func NewHitbtcPublicApi() (*HitbtcApi, error) {
	api := &HitbtcApi{
		BaseURL:           HITBTC_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),

		m: new(sync.Mutex),
	}
	api.fetchSettlements()
	return api, nil
}

type HitbtcApi struct {
	BaseURL           string
	RateCacheDuration time.Duration
	volumeMap         map[string]map[string]float64
	rateMap           map[string]map[string]float64
	rateLastUpdated   time.Time
	HttpClient        http.Client

	settlements []string

	m *sync.Mutex
	c *HitbtcApiConfig
}

func (h *HitbtcApi) publicApiUrl(command string) string {
	return h.BaseURL + "/public/" + command
}

func (h *HitbtcApi) fetchSettlements() error {
	settlements := make([]string, 0)
	url := h.publicApiUrl("symbol")
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

	pairMap, err := json.Children()
	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}
	for _, v := range pairMap {
		settlement, ok := v.Path("quoteCurrency").Data().(string)
		if !ok {
			continue
		}
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

func (h *HitbtcApi) fetchRate() error {
	h.rateMap = make(map[string]map[string]float64)
	h.volumeMap = make(map[string]map[string]float64)
	url := h.publicApiUrl("ticker")
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

	rateMap, err := json.Children()
	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}
	for _, v := range rateMap {
		pair, ok := v.Path("symbol").Data().(string)
		if !ok {
			continue
		}

		var settlement string
		var trading string
		for _, s := range h.settlements {
			index := strings.LastIndex(pair, s)
			if index != 0 && index == len(pair)-len(s) {
				settlement = s
				trading = pair[0:index]
			}
		}
		if settlement == "" || trading == "" {
			continue
		}
		// update rate
		last, ok := v.Path("last").Data().(string)
		if !ok {
			continue
		}

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
		volume, ok := v.Path("volume").Data().(string)
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
	}

	return nil
}

func (h *HitbtcApi) RateMap() (map[string]map[string]float64, error) {
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

func (h *HitbtcApi) VolumeMap() (map[string]map[string]float64, error) {
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

func (h *HitbtcApi) CurrencyPairs() ([]models.CurrencyPair, error) {
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

func (h *HitbtcApi) Volume(trading string, settlement string) (float64, error) {
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

func (h *HitbtcApi) Rate(trading string, settlement string) (float64, error) {
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

func (h *HitbtcApi) FrozenCurrency() ([]string, error) {
	url := h.publicApiUrl("currency")
	resp, err := h.HttpClient.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	json, err := gabs.ParseJSON(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	currencyMap, err := json.Children()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	var frozens []string
	for _, v := range currencyMap {
		payoutEnable, ok := v.Path("payoutEnabled").Data().(bool)
		if !ok {
			continue
		}
		if !payoutEnable {
			currency, ok := v.Path("id").Data().(string)
			if !ok {
				continue
			}
			frozens = append(frozens, currency)
		}
	}
	return frozens, nil
}

func (h *HitbtcApi) Board(trading string, settlement string) (board *models.Board, err error) {
	url := h.publicApiUrl("orderbook/" + trading + settlement)
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
	jsonBids, err := json.GetObjectArray("bid")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	jsonAsks, err := json.GetObjectArray("ask")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	bids := make([]models.BoardOrder, 0)
	asks := make([]models.BoardOrder, 0)
	for _, v := range jsonBids {
		priceStr, err := v.GetString("price")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		price, err := strconv.ParseFloat(priceStr, 10)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		sizeStr, err := v.GetString("size")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse size")
		}
		size, err := strconv.ParseFloat(sizeStr, 10)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse size")
		}
		bids = append(bids, models.BoardOrder{
			Price:  price,
			Amount: size,
			Type:   models.Bid,
		})
	}
	for _, v := range jsonAsks {
		priceStr, err := v.GetString("price")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		price, err := strconv.ParseFloat(priceStr, 10)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		sizeStr, err := v.GetString("size")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse size")
		}
		size, err := strconv.ParseFloat(sizeStr, 10)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse size")
		}
		asks = append(asks, models.BoardOrder{
			Price:  price,
			Amount: size,
			Type:   models.Ask,
		})
	}
	board = &models.Board{
		Bids: bids,
		Asks: asks,
	}
	return board, nil
}
