package public

import (
	"io/ioutil"
	"net/http"
	"time"

	"sync"

	"strings"

	"github.com/Jeffail/gabs"
	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-ccex-api-client/models"
	"github.com/pkg/errors"
)

const (
	BITFLYER_BASE_URL = "https://api.bitflyer.jp/v1"
)

func NewBitflyerPublicApi() (*BitflyerApi, error) {
	api := &BitflyerApi{
		BaseURL:           BITFLYER_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		HttpClient:        http.Client{},

		m: new(sync.Mutex),
	}
	api.fetchSettlements()
	return api, nil
}

type BitflyerApi struct {
	BaseURL           string
	RateCacheDuration time.Duration
	HttpClient        http.Client

	volumeMap       map[string]map[string]float64
	rateMap         map[string]map[string]float64
	rateLastUpdated time.Time
	settlements     []string

	m *sync.Mutex
}

func (b *BitflyerApi) publicApiUrl(command string) string {
	return b.BaseURL + "/" + command
}

func (b *BitflyerApi) fetchSettlements() error {
	sets := make([]string, 0)
	sets = append(sets, "JPY")
	b.settlements = sets
	return nil
}

func (b *BitflyerApi) fetchRate() error {
	b.rateMap = make(map[string]map[string]float64)
	b.volumeMap = make(map[string]map[string]float64)
	url := b.publicApiUrl("ticker")
	resp, err := b.HttpClient.Get(url)
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
	pair := json.Path("product_code").Data().(string)

	var settlement string
	var trading string
	for _, s := range b.settlements {
		index := strings.LastIndex(pair, s)
		if index != 0 && index == len(pair)-len(s) {
			settlement = s
			trading = strings.Replace(pair[0:index], "_", "", -1)
		}
	}
	if settlement == "" || trading == "" {
		return errors.New("pair is not parsed")
	}
	// update rate
	last, ok := json.Path("ltp").Data().(float64)
	if !ok {
		return errors.New("close price is not parsed")
	}

	m, ok := b.rateMap[trading]
	if !ok {
		m = make(map[string]float64)
		b.rateMap[trading] = m
	}
	m[settlement] = last

	// update volume
	volume, ok := json.Path("volume").Data().(float64)
	if !ok {
		return errors.New("volume is not parsed")
	}

	m, ok = b.volumeMap[trading]
	if !ok {
		m = make(map[string]float64)
		b.volumeMap[trading] = m
	}
	m[settlement] = volume

	return nil
}

func (b *BitflyerApi) CurrencyPairs() ([]models.CurrencyPair, error) {
	b.m.Lock()
	defer b.m.Unlock()

	now := time.Now()
	if now.Sub(b.rateLastUpdated) >= b.RateCacheDuration {
		err := b.fetchRate()
		if err != nil {
			return nil, err
		}
		b.rateLastUpdated = now
	}

	var pairs []models.CurrencyPair
	for trading, m := range b.rateMap {
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

func (b *BitflyerApi) Volume(trading string, settlement string) (float64, error) {
	b.m.Lock()
	defer b.m.Unlock()

	now := time.Now()
	if now.Sub(b.rateLastUpdated) >= b.RateCacheDuration {
		err := b.fetchRate()
		if err != nil {
			return 0, err
		}
		b.rateLastUpdated = now
	}

	if m, ok := b.volumeMap[trading]; !ok {
		return 0, errors.New("trading volume not found")
	} else if volume, ok := m[settlement]; !ok {
		return 0, errors.New("settlement volume not found")
	} else {
		return volume, nil
	}
}

func (b *BitflyerApi) Rate(trading string, settlement string) (float64, error) {
	b.m.Lock()
	defer b.m.Unlock()

	if trading == settlement {
		return 1, nil
	}

	now := time.Now()
	if now.Sub(b.rateLastUpdated) >= b.RateCacheDuration {
		err := b.fetchRate()
		if err != nil {
			return 0, err
		}
		b.rateLastUpdated = now
	}
	if m, ok := b.rateMap[trading]; !ok {
		return 0, errors.New("trading rate not found")
	} else if rate, ok := m[settlement]; !ok {
		return 0, errors.New("settlement rate not found")
	} else {
		return rate, nil
	}
}

func (b *BitflyerApi) RateMap() (map[string]map[string]float64, error) {
	b.m.Lock()
	defer b.m.Unlock()
	now := time.Now()
	if now.Sub(b.rateLastUpdated) >= b.RateCacheDuration {
		err := b.fetchRate()
		if err != nil {
			return nil, err
		}
		b.rateLastUpdated = now
	}
	return b.rateMap, nil
}

func (b *BitflyerApi) FrozenCurrency() ([]string, error) {
	return []string{}, nil
}

func (b *BitflyerApi) Board(trading string, settlement string) (board *models.Board, err error) {
	url := b.publicApiUrl("board") + "?product_code=" + strings.ToUpper(trading) + "_" + strings.ToLower(settlement)
	resp, err := b.HttpClient.Get(url)
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
	jsonBids, err := json.GetObjectArray("bids")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	jsonAsks, err := json.GetObjectArray("asks")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	bids := make([]models.BoardOrder, 0)
	asks := make([]models.BoardOrder, 0)
	for _, v := range jsonBids {
		price, err := v.GetFloat64("price")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		size, err := v.GetFloat64("size")
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
		price, err := v.GetFloat64("price")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse price")
		}
		size, err := v.GetFloat64("size")
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
