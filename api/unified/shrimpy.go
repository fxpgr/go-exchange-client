package unified

import (
	"io/ioutil"
	"net/http"
	url2 "net/url"
	"sync"
	"time"

	"github.com/fxpgr/go-exchange-client/models"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	SHRIMPY_BASE_URL = "https://dev-api.shrimpy.io/v1"
)

func NewShrimpyApi() (*ShrimpyApiClient, error) {
	cli := &http.Client{}
	cli.Timeout = 20 * time.Second
	api := &ShrimpyApiClient{
		BaseURL:         SHRIMPY_BASE_URL,
		rateLastUpdated: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		HttpClient:      cli,
	}
	return api, nil
}

type ShrimpyApiClient struct {
	BaseURL           string
	RateCacheDuration time.Duration
	rateLastUpdated   time.Time
	volumeMap         map[string]map[string]float64
	rateMap           map[string]map[string]float64
	orderBookTickMap  map[string]map[string]models.OrderBookTick
	precisionMap      map[string]map[string]models.Precisions
	boardCache        *cache.Cache
	boardTickerCache  *cache.Cache
	currencyPairs     []models.CurrencyPair

	HttpClient *http.Client

	settlements  []string
	m            *sync.Mutex
	rateM        *sync.Mutex
	currencyM    *sync.Mutex
	boardTickerM *sync.Mutex
}

func (h *ShrimpyApiClient) publicApiUrl(command string) string {
	return h.BaseURL + command
}

func (h *ShrimpyApiClient) getRequest(path string) ([]byte, error) {
	url := h.publicApiUrl(path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, err
	}
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return []byte{}, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()
	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, errors.Wrapf(err, "failed to fetch %s", url)
	}
	return byteArray, nil
}
func (h *ShrimpyApiClient) GetCurrencyPairs(exchange string) ([]models.CurrencyPair, error) {
	byteArray, err := h.getRequest("/exchanges/" + exchange + "/trading_pairs")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", exchange)
	}
	value := gjson.ParseBytes(byteArray)
	if !value.Exists() {
		return nil, errors.New("failed to parse json: this is not exists")
	}
	if !value.IsArray() {
		return nil, errors.New("failed to parse json: this is not array")
	}
	currencyPairs := make([]models.CurrencyPair, 0)
	for _, v := range value.Array() {
		trading := v.Get("baseTradingSymbol").String()
		settlement := v.Get("quoteTradingSymbol").String()
		currencyPairs = append(currencyPairs, models.CurrencyPair{
			Trading:    trading,
			Settlement: settlement,
		})
	}
	return currencyPairs, nil
}

func (h *ShrimpyApiClient) GetCurrencys(exchange string) ([]models.Asset, error) {
	byteArray, err := h.getRequest("/exchanges/" + exchange + "/assets")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", exchange)
	}
	value := gjson.ParseBytes(byteArray)
	if !value.Exists() {
		return nil, errors.New("failed to parse json: this is not exists")
	}
	if !value.IsArray() {
		return nil, errors.New("failed to parse json: this is not array")
	}
	assets := make([]models.Asset, 0)
	for _, v := range value.Array() {
		name := v.Get("name").String()
		symbol := v.Get("symbol").String()
		assets = append(assets, models.Asset{
			Name:   name,
			Symbol: symbol,
		})
	}
	return assets, nil
}

func (h *ShrimpyApiClient) GetBoards(exchange string) (map[string]map[string]models.Board, error) {
	args := url2.Values{}
	args.Add("exchange", exchange)
	byteArray, err := h.getRequest("/orderbooks?" + args.Encode())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", exchange)
	}
	value := gjson.ParseBytes(byteArray)
	if !value.Exists() {
		return nil, errors.New("failed to parse json: this is not exists")
	}
	if !value.IsArray() {
		return nil, errors.New("failed to parse json: this is not array")
	}
	boards := make(map[string]map[string]models.Board)

	for _, v := range value.Array() {
		settlement := v.Get("quoteSymbol").String()
		trading := v.Get("baseSymbol").String()
		asks := v.Get("orderBooks").Array()[0].Get("orderBook.asks")
		if !asks.IsArray() || !asks.Exists() {
			continue
		}
		bids := v.Get("orderBooks").Array()[0].Get("orderBook.bids")
		if !bids.IsArray() || !bids.Exists() {
			continue
		}
		bidBoardBars := make([]models.BoardBar, 0)
		askBoardBars := make([]models.BoardBar, 0)
		for _, ask := range asks.Array() {
			price := ask.Get("price").Float()
			quantity := ask.Get("quantity").Float()
			askBoardBars = append(askBoardBars, models.BoardBar{
				Price:  price,
				Amount: quantity,
				Type:   models.Ask,
			})
		}
		for _, bid := range bids.Array() {
			price := bid.Get("price").Float()
			quantity := bid.Get("quantity").Float()
			bidBoardBars = append(bidBoardBars, models.BoardBar{
				Price:  price,
				Amount: quantity,
				Type:   models.Bid,
			})
		}
		board := models.Board{
			Bids: bidBoardBars,
			Asks: askBoardBars,
		}
		m, ok := boards[trading]
		if !ok {
			m = make(map[string]models.Board)
			boards[trading] = m
		}
		m[settlement] = board
	}
	return boards, nil
}
