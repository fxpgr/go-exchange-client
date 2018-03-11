package public

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"encoding/json"
	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-ccex-api-client/logger"
	"github.com/fxpgr/go-ccex-api-client/models"
	"github.com/pkg/errors"
	"io/ioutil"
	url2 "net/url"
)

const (
	POLONIEX_BASE_URL = "https://poloniex.com"
)

func NewPoloniexPublicApi() (*PoloniexApi, error) {
	api := &PoloniexApi{
		BaseURL:           POLONIEX_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		HttpClient:        http.Client{},

		m: new(sync.Mutex),
	}
	return api, nil
}

func parsePoloCurrencyPair(s string) (string, string, error) {
	xs := strings.Split(s, "_")

	if len(xs) != 2 {
		return "", "", errors.New("invalid ticker title")
	}

	return xs[0], xs[1], nil
}

type PoloniexApi struct {
	BaseURL           string
	RateCacheDuration time.Duration
	volumeMap         map[string]map[string]float64
	rateMap           map[string]map[string]float64
	rateLastUpdated   time.Time
	HttpClient        http.Client

	m *sync.Mutex
}

func (p *PoloniexApi) publicApiUrl(command string) string {
	return p.BaseURL + "/public?command=" + command
}

func (p *PoloniexApi) fetchRate() error {
	p.rateMap = make(map[string]map[string]float64)
	p.volumeMap = make(map[string]map[string]float64)
	url := p.publicApiUrl("returnTicker")

	resp, err := p.HttpClient.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	json, err := jason.NewObjectFromReader(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to parse json")
	}

	rateMap := json.Map()
	for k, v := range rateMap {
		settlement, trading, err := parsePoloCurrencyPair(k)
		if err != nil {
			logger.Get().Warn("couldn't parse currency pair", err)
			continue
		}

		obj, err := v.Object()
		if err != nil {
			return err
		}

		// update rate
		last, err := obj.GetString("last")
		if err != nil {
			return err
		}

		lastf, err := strconv.ParseFloat(last, 64)
		if err != nil {
			return err
		}

		m, ok := p.rateMap[trading]
		if !ok {
			m = make(map[string]float64)
			p.rateMap[trading] = m
		}
		m[settlement] = lastf

		// update volume
		volume, err := obj.GetString("baseVolume")
		if err != nil {
			return err
		}

		volumef, err := strconv.ParseFloat(volume, 64)
		if err != nil {
			return err
		}

		m, ok = p.volumeMap[trading]
		if !ok {
			m = make(map[string]float64)
			p.volumeMap[trading] = m
		}
		m[settlement] = volumef
	}
	return nil
}

func (p *PoloniexApi) CurrencyPairs() ([]models.CurrencyPair, error) {
	p.m.Lock()
	defer p.m.Unlock()

	now := time.Now()
	if now.Sub(p.rateLastUpdated) >= p.RateCacheDuration {
		err := p.fetchRate()
		if err != nil {
			return nil, err
		}
		p.rateLastUpdated = now
	}

	var pairs []models.CurrencyPair
	for trading, m := range p.rateMap {
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

func (p *PoloniexApi) Volume(trading string, settlement string) (float64, error) {
	p.m.Lock()
	defer p.m.Unlock()

	now := time.Now()
	if now.Sub(p.rateLastUpdated) >= p.RateCacheDuration {
		err := p.fetchRate()
		if err != nil {
			return 0, err
		}
		p.rateLastUpdated = now
	}

	if m, ok := p.volumeMap[trading]; !ok {
		return 0, errors.New("trading volume not found")
	} else if volume, ok := m[settlement]; !ok {
		return 0, errors.New("settlement volume not found")
	} else {
		return volume, nil
	}
}

func (p *PoloniexApi) RateMap() (map[string]map[string]float64, error) {
	p.m.Lock()
	defer p.m.Unlock()
	now := time.Now()
	if now.Sub(p.rateLastUpdated) >= p.RateCacheDuration {
		err := p.fetchRate()
		if err != nil {
			return nil, err
		}
		p.rateLastUpdated = now
	}
	return p.rateMap, nil
}

func (p *PoloniexApi) Rate(trading string, settlement string) (float64, error) {
	p.m.Lock()
	defer p.m.Unlock()

	if trading == settlement {
		return 1, nil
	}

	now := time.Now()
	if now.Sub(p.rateLastUpdated) >= p.RateCacheDuration {
		err := p.fetchRate()
		if err != nil {
			return 0, err
		}
		p.rateLastUpdated = now
	}
	if m, ok := p.rateMap[trading]; !ok {
		return 0, errors.New("trading rate not found")
	} else if rate, ok := m[settlement]; !ok {
		return 0, errors.New("settlement rate not found")
	} else {
		return rate, nil
	}
}

func (p *PoloniexApi) FrozenCurrency() ([]string, error) {
	url := p.publicApiUrl("returnCurrencies")
	resp, err := p.HttpClient.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	var frozens []string
	m := make(map[string]Currency)
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, errors.Wrap(err, "failed to parse response")
	}
	for k, v := range m {
		if v.Frozen == 1 || v.Delisted == 1 || v.Disabled == 1 {
			frozens = append(frozens, k)
		}
	}
	return frozens, nil
}

type Currency struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	TxFee          float64 `json:"txFee,string"`
	MinConf        int     `json:"minConf"`
	DepositAddress string  `json:"depositAddress"`
	Disabled       int     `json:"disabled"`
	Delisted       int     `json:"delisted"`
	Frozen         int     `json:"frozen"`
}

func (p *PoloniexApi) Board(trading string, settlement string) (*models.Board, error) {
	args := url2.Values{}
	args.Add("currencyPair", settlement+"_"+trading)
	url := p.publicApiUrl("returnOrderBook") + "?" + args.Encode()
	resp, err := p.HttpClient.Get(url)
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
	jsonBids, err := json.GetValueArray("bids")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	jsonAsks, err := json.GetValueArray("asks")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
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
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		price, err := strconv.ParseFloat(priceStr, 10)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		amount, err := s[1].Float64()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
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
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		price, err := strconv.ParseFloat(priceStr, 10)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		amount, err := s[1].Float64()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		asks = append(asks, models.BoardOrder{
			Price:  price,
			Amount: amount,
			Type:   models.Ask,
		})
	}

	board := &models.Board{
		Bids: bids,
		Asks: asks,
	}
	return board, nil
}
