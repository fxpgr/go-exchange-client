package public

import (
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/fxpgr/go-exchange-client/models"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	BINANCE_BASE_URL = "https://api.binance.com"
)

func NewBinancePublicApi() (*BinanceApi, error) {
	cli := &http.Client{}
	cli.Timeout = 20 * time.Second
	api := &BinanceApi{
		BaseURL:           BINANCE_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		rateMap:           nil,
		volumeMap:         nil,
		orderBookTickMap:  nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		boardCache:        cache.New(3*time.Second, 1*time.Second),
		boardTickerCache:  cache.New(3*time.Second, 1*time.Second),
		HttpClient:        cli,

		m:            new(sync.Mutex),
		rateM:        new(sync.Mutex),
		currencyM:    new(sync.Mutex),
		boardTickerM: new(sync.Mutex),
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

func (h *BinanceApi) SetTransport(transport http.RoundTripper) error {
	h.HttpClient.Transport = transport
	return nil
}

func (h *BinanceApi) renewHttpClient() error {
	rt := h.HttpClient.Transport
	h.HttpClient = &http.Client{Transport: rt}
	return nil
}

func (h *BinanceApi) publicApiUrl(command string) string {
	return h.BaseURL + command
}

func (h *BinanceApi) getRequest(url string) (string, error) {
	req, err := requestGetAsChrome(url)
	if err != nil {
		return "", errors.Wrapf(err, "failed to fetch %s", url)
	}
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "failed to fetch %s", url)
	}
	return string(byteArray), err
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

	byteArray, err := h.getRequest(url)
	if err != nil {
		return err
	}
	value := gjson.Parse(byteArray)

	if value.Get("code").String() == "-1003" {
		return errors.Errorf("ip banned %s", url)
	}
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
	byteArray, err := h.getRequest(url)
	if err != nil {
		return err
	}
	value := gjson.Parse(byteArray)
	if value.Get("code").String() == "-1003" {
		return errors.Errorf("ip banned %s", url)
	}
	currencyPairs, err := h.CurrencyPairs()
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}

	rateMap := make(map[string]map[string]float64)
	volumeMap := make(map[string]map[string]float64)
	orderBookTickMap := make(map[string]map[string]models.OrderBookTick)
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
		bestbidPrice := v.Get("bidPrice").Float()
		bestaskPrice := v.Get("askPrice").Float()
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
		l, ok := orderBookTickMap[trading]
		if !ok {
			l = make(map[string]models.OrderBookTick)
			orderBookTickMap[trading] = l
		}
		l[settlement] = models.OrderBookTick{
			BestAskPrice: bestaskPrice,
			BestBidPrice: bestbidPrice,
		}
		h.rateM.Unlock()
	}
	h.rateMap = rateMap
	h.volumeMap = volumeMap
	h.orderBookTickMap = orderBookTickMap
	return nil
}

func (h *BinanceApi) OrderBookTickMap() (map[string]map[string]models.OrderBookTick, error) {
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
	byteArray, err := h.getRequest(url)
	if err != nil {
		return h.currencyPairs, err
	}
	value := gjson.Parse(byteArray)
	currencyPairs := make([]models.CurrencyPair, 0)

	if value.Get("code").String() == "-1003" {
		return h.currencyPairs, errors.Errorf("ip banned %s", url)
	}
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

	byteArray, err := h.getRequest(url)
	if err != nil {
		return []string{}, err
	}
	value := gjson.Parse(byteArray)

	if value.Get("code").String() == "-1003" {
		return []string{}, errors.Errorf("ip banned %s", url)
	}
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

func (h *BinanceApi) fetchBoardTicker() error {
	url := h.publicApiUrl("/api/v3/ticker/bookTicker")
	byteArray, err := h.getRequest(url)
	if err != nil {
		return err
	}
	value := gjson.Parse(byteArray)
	if value.Get("code").String() == "-1003" {
		return errors.Errorf("ip banned %s", url)
	}
	currencyPairs, err := h.CurrencyPairs()
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s", url)
	}

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
		bids := make([]models.BoardBar, 0)
		asks := make([]models.BoardBar, 0)

		bestBidPricef := v.Get("bidPrice").Float()
		bestBidAmountf := v.Get("bidQty").Float()

		bids = append(bids, models.BoardBar{
			Price:  bestBidPricef,
			Amount: bestBidAmountf,
			Type:   models.Bid,
		})

		bestAskPricef := v.Get("askPrice").Float()
		bestAskAmountf := v.Get("askQty").Float()

		asks = append(asks, models.BoardBar{
			Price:  bestAskPricef,
			Amount: bestAskAmountf,
			Type:   models.Ask,
		})
		board := &models.Board{
			Bids: bids,
			Asks: asks,
		}
		h.boardTickerCache.Set(trading+"_"+settlement, board, cache.DefaultExpiration)
	}
	return nil
}

func (h *BinanceApi) Board(trading string, settlement string) (board *models.Board, err error) {
	c, found := h.boardCache.Get(trading + "_" + settlement)
	if found {
		return c.(*models.Board), nil
	}
	if trading == settlement {
		return nil, errors.Errorf("trading and settlment are same")
	}
	url := h.publicApiUrl("/api/v1/depth?limit=1000&symbol=" + trading + settlement)
	byteArray, err := h.getRequest(url)
	if err != nil {
		return nil, err
	}
	value := gjson.Parse(byteArray)
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

/*duplicated*/
func (h *BinanceApi) BoardTicker(trading string, settlement string) (board *models.Board, err error) {
	h.boardTickerM.Lock()
	defer h.boardTickerM.Unlock()
	c, found := h.boardTickerCache.Get(trading + "_" + settlement)
	if found {
		return c.(*models.Board), nil
	}
	if trading == settlement {
		return nil, errors.Errorf("trading and settlment are same")
	}
	err = h.fetchBoardTicker()
	if err != nil {
		return nil, err
	}
	c, found = h.boardTickerCache.Get(trading + "_" + settlement)
	if !found {
		return nil, errors.Errorf("cache err")
	}
	return c.(*models.Board), nil
}
