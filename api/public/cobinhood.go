package public

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"io/ioutil"
	"strings"
	"net/url"
	"fmt"
)

const (
	COBINHOOD_BASE_URL = "https://api.cobinhood.com"
)

type CobinhoodApiConfig struct {
}

func NewCobinhoodPublicApi() (*CobinhoodApi, error) {
	api := &CobinhoodApi{
		BaseURL:           COBINHOOD_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyPairsCacheDuration: 7 * 24 * time.Hour,
		currencyPairsLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),

		m: new(sync.Mutex),
		currencyM: new(sync.Mutex),
	}
	api.fetchSettlements()
	return api, nil
}

type CobinhoodApi struct {
	BaseURL           string
	RateCacheDuration time.Duration
	volumeMap         map[string]map[string]float64
	rateMap           map[string]map[string]float64
	rateLastUpdated   time.Time
	currencyPairs              []models.CurrencyPair
	CurrencyPairsCacheDuration time.Duration
	currencyPairsLastUpdated   time.Time
	HttpClient        http.Client

	settlements []string

	m *sync.Mutex
	currencyM *sync.Mutex
	c *CobinhoodApiConfig
}

func (h *CobinhoodApi) publicApiUrl(command string) string {
	return h.BaseURL + command
}

func (h *CobinhoodApi) fetchSettlements() error {
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

func (h *CobinhoodApi) fetchRate() error {
	h.rateMap = make(map[string]map[string]float64)
	h.volumeMap = make(map[string]map[string]float64)
	url := h.publicApiUrl("/v1/market/tickers")
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
	result, err := json.GetObject("result")
	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}
	tickers, err := result.GetObjectArray("tickers")
	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}
	for _, v := range tickers {
		lastString, err := v.GetString("last_trade_price")
		if err != nil {
			return errors.Wrapf(err, "failed to parse quote")
		}
		lastf, err := strconv.ParseFloat(lastString, 64)
		if err != nil {
			return errors.Wrapf(err, "failed to parse quote")
		}

		volumeString, err := v.GetString("24h_volume")
		if err != nil {
			return errors.Wrapf(err, "failed to parse quote")
		}
		volumef, err := strconv.ParseFloat(volumeString, 64)
		if err != nil {
			return errors.Wrapf(err, "failed to parse quote")
		}

		pairString, err := v.GetString("trading_pair_id")
		if err != nil {
			return errors.Wrapf(err, "failed to parse quote")
		}
		currencies := strings.Split(pairString, "-")
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

func (h *CobinhoodApi) RateMap() (map[string]map[string]float64, error) {
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

func (h *CobinhoodApi) VolumeMap() (map[string]map[string]float64, error) {
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

func (h *CobinhoodApi) CurrencyPairs() ([]models.CurrencyPair, error) {
	h.currencyM.Lock()
	defer h.currencyM.Unlock()
	if len(h.currencyPairs) != 0 {
		return h.currencyPairs, nil
	}
	url := h.publicApiUrl("/v1/market/trading_pairs")
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
	result, err := json.GetObject("result")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	trading_pairs, err := result.GetObjectArray("trading_pairs")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	var pairs []models.CurrencyPair
	for _, v := range trading_pairs {
		trading, err := v.GetString("base_currency_id")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse quote")
		}
		settlement, err := v.GetString("quote_currency_id")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse quote")
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

func (h *CobinhoodApi) Volume(trading string, settlement string) (float64, error) {
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

func (h *CobinhoodApi) Rate(trading string, settlement string) (float64, error) {
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

func (h *CobinhoodApi) FrozenCurrency() ([]string, error) {
	var frozens []string
	url := h.publicApiUrl("/v1/market/currencies")
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
	result, err := json.GetObject("result")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	currencies, err := result.GetObjectArray("currencies")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	for _, v := range currencies {
		isFrozen, err := v.GetBoolean("funding_frozen")
		if err != nil {
			continue
		}
		isActive, err := v.GetBoolean("is_active")
		if err != nil {
			continue
		}
		if !isFrozen && isActive{
			continue
		}
		currencyName, err := v.GetString("currency")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse quote")
		}
		frozens = append(frozens, currencyName)
	}
	return frozens, nil
}

func (h *CobinhoodApi) Board(trading string, settlement string) (board *models.Board, err error) {
	args := url.Values{}
	args.Add("limit", "10000")
	path := h.publicApiUrl("/v1/market/orderbooks/" + trading +"-"+ settlement) +"?"+args.Encode()
	resp, err := h.HttpClient.Get(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", path)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", path)
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	result, err := json.GetObject("result")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json result")
	}
	orderbook, err := result.GetObject("orderbook")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json orderbook")
	}
	jsonBids, err := orderbook.GetValueArray("bids")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json bids")
	}
	jsonAsks, err := orderbook.GetValueArray("asks")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json asks")
	}
	bids := make([]models.BoardOrder, 0)
	asks := make([]models.BoardOrder, 0)
	for _, v := range jsonBids {
		s, err := v.Array()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse array")
		}
		priceStr, err := s[0].String()
		if err != nil {
			fmt.Println(s[0])
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		amountStr, err := s[2].String()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse amount")
		}
		price,err := strconv.ParseFloat(priceStr,64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse amount")
		}
		amount,err := strconv.ParseFloat(amountStr,64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse amount")
		}
		bids = append(bids, models.BoardOrder{
			Price:  price,
			Amount: amount,
			Type:   models.Bid,
		})
	}
	for _, v := range jsonAsks {
		s, err := v.Array()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse array")
		}
		priceStr, err := s[0].String()
		if err != nil {
			fmt.Println(s[0])
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		amountStr, err := s[2].String()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse amount")
		}
		price,err := strconv.ParseFloat(priceStr,64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse amount")
		}
		amount,err := strconv.ParseFloat(amountStr,64)
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
