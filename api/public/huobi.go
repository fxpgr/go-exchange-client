package public

import (
	"net/http"
	"sync"
	"time"

	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-ccex-api-client/models"
	"github.com/pkg/errors"
	"io/ioutil"
	"strings"
)

const (
	HUOBI_BASE_URL = "https://api.huobi.pro"
)

type HuobiApiConfig struct {
}

func NewHuobiPublicApi() (*HuobiApi, error) {
	api := &HuobiApi{
		BaseURL:           HUOBI_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		HttpClient:        &http.Client{},

		m: new(sync.Mutex),
	}
	api.fetchSettlements()
	return api, nil
}

type HuobiApi struct {
	BaseURL           string
	RateCacheDuration time.Duration
	volumeMap         map[string]map[string]float64
	rateMap           map[string]map[string]float64
	currencyPairs     []models.CurrencyPair
	rateLastUpdated   time.Time
	HttpClient        *http.Client

	settlements []string

	m *sync.Mutex
	c *HuobiApiConfig
}

func (h *HuobiApi) publicApiUrl(command string) string {
	return h.BaseURL + command
}

func (h *HuobiApi) fetchSettlements() error {
	settlements := make([]string, 0)
	url := h.publicApiUrl("/v1/common/symbols")
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
		settlement, err := v.GetString("quote-currency")
		if err != nil {
			return errors.Wrapf(err, "failed to parse json")
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

type HuobiTickResponse struct {
	response   []byte
	Trading    string
	Settlement string
	err        error
}

func (h *HuobiApi) fetchRate() error {
	currencyPairs, err := h.CurrencyPairs()
	if err != nil {
		return err
	}
	ch := make(chan *HuobiTickResponse, len(currencyPairs))
	workers := make(chan int, 10)
	wg := &sync.WaitGroup{}
	for _, v := range currencyPairs {
		wg.Add(1)
		workers <- 1
		go func(trading string, settlement string) {
			defer wg.Done()
			url := h.publicApiUrl("/market/detail/merged?symbol=" + strings.ToLower(trading) + strings.ToLower(settlement))
			resp, err := h.HttpClient.Get(url)
			if err != nil {
				ch <- &HuobiTickResponse{nil, trading, settlement, err}
				return
			}
			defer resp.Body.Close()
			byteArray, err := ioutil.ReadAll(resp.Body)
			ch <- &HuobiTickResponse{byteArray, trading, settlement, err}
			<-workers
		}(v.Trading, v.Settlement)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	h.rateMap = make(map[string]map[string]float64)
	h.volumeMap = make(map[string]map[string]float64)
	for r := range ch {
		if r.err != nil {
			continue
		}
		data, err := jason.NewObjectFromBytes(r.response)
		if err != nil {
			continue
		}
		tick, err := data.GetObject("tick")
		if err != nil {
			continue
		}
		volume, err := tick.GetFloat64("vol")
		if err != nil {
			continue
		}
		m, ok := h.volumeMap[r.Trading]
		if !ok {
			m = make(map[string]float64)
			h.volumeMap[r.Trading] = m
		}
		m[r.Settlement] = volume
		close, err := tick.GetFloat64("close")
		if err != nil {
			continue
		}
		m, ok = h.rateMap[r.Trading]
		if !ok {
			m = make(map[string]float64)
			h.rateMap[r.Trading] = m
		}
		m[r.Settlement] = close
	}
	return nil
}

func (h *HuobiApi) RateMap() (map[string]map[string]float64, error) {
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

func (h *HuobiApi) CurrencyPairs() ([]models.CurrencyPair, error) {
	if h.currencyPairs != nil {
		return h.currencyPairs, nil
	}

	url := h.publicApiUrl("/v1/common/symbols")
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
		settlement, err := v.GetString("quote-currency")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse quote")
		}
		trading, err := v.GetString("base-currency")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse base")
		}
		pair := models.CurrencyPair{
			Trading:    strings.ToUpper(trading),
			Settlement: strings.ToUpper(settlement),
		}
		pairs = append(pairs, pair)
	}
	h.currencyPairs = pairs
	return pairs, nil
}

func (h *HuobiApi) Volume(trading string, settlement string) (float64, error) {
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

func (h *HuobiApi) Rate(trading string, settlement string) (float64, error) {
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

func (h *HuobiApi) FrozenCurrency() ([]string, error) {
	var frozens []string
	return frozens, nil
}
