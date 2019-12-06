package private

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/helpers"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	BINANCE_BASE_URL = "https://api.binance.com"
)

func NewBinanceApi(apikey func() (string, error), apisecret func() (string, error)) (*BinanceApi, error) {
	hitbtcPublic, err := public.NewBinancePublicApi()
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize public client")
	}
	pairs, err := hitbtcPublic.CurrencyPairs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pairs")
	}
	var settlements []string
	for _, v := range pairs {
		settlements = append(settlements, v.Settlement)
	}
	m := make(map[string]bool)
	uniq := []string{}
	for _, ele := range settlements {
		if !m[ele] {
			m[ele] = true
			uniq = append(uniq, ele)
		}
	}
	b := &BinanceApi{
		BaseURL:           BINANCE_BASE_URL,
		apiV1:             BINANCE_BASE_URL + "/api/v1/",
		apiV3:             BINANCE_BASE_URL + "/api/v3/",
		RateCacheDuration: 30 * time.Second,
		ApiKeyFunc:        apikey,
		SecretKeyFunc:     apisecret,
		settlements:       uniq,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		rt:                &http.Transport{},

		m:         new(sync.Mutex),
		currencyM: new(sync.Mutex),
	}
	b.setTimeOffset()
	return b, nil
}

type BinanceApi struct {
	ApiKeyFunc        func() (string, error)
	SecretKeyFunc     func() (string, error)
	BaseURL           string
	RateCacheDuration time.Duration
	HttpClient        http.Client
	rt                *http.Transport
	settlements       []string
	apiV1             string
	apiV3             string
	timeoffset        int64

	volumeMap       map[string]map[string]float64
	rateMap         map[string]map[string]float64
	precisionMap    map[string]map[string]models.Precisions
	currencyPairs   []models.CurrencyPair
	rateLastUpdated time.Time

	m         *sync.Mutex
	currencyM *sync.Mutex
}

func (h *BinanceApi) privateApiUrl() string {
	return h.BaseURL
}

func (h *BinanceApi) publicApiUrl(command string) string {
	return h.BaseURL + command
}

func (h *BinanceApi) fetchPrecision() error {
	if h.precisionMap != nil {
		return nil
	}

	h.precisionMap = make(map[string]map[string]models.Precisions)
	url := h.publicApiUrl("/api/v1/exchangeInfo")
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
	value := gjson.ParseBytes(byteArray)
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

func (h *BinanceApi) precise(trading string, settlement string) (*models.Precisions, error) {
	if trading == settlement {
		return &models.Precisions{}, nil
	}

	h.fetchPrecision()
	if m, ok := h.precisionMap[trading]; !ok {
		return &models.Precisions{}, errors.Errorf("%s/%s missing trading", trading, settlement)
	} else if precisions, ok := m[settlement]; !ok {
		return &models.Precisions{}, errors.Errorf("%s/%s missing settlement", trading, settlement)
	} else {
		return &precisions, nil
	}
}
func (h *BinanceApi) privateApi(method string, path string, params *url.Values) ([]byte, error) {
	urlStr := h.BaseURL + path
	if strings.ToUpper(method) == "GET" {
		urlStr = urlStr + "?" + params.Encode()
	}
	nonce := time.Now().Unix() * 1000
	params.Set("timestamp", fmt.Sprintf("%d", nonce))

	reader := bytes.NewReader([]byte(params.Encode()))
	req, err := http.NewRequest(method, urlStr, reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	apiKey, err := h.ApiKeyFunc()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	secKey, err := h.SecretKeyFunc()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	req.Header.Set("Accept", "application/json")

	req.Header.Set("X-MBX-APIKEY", apiKey)
	mac := hmac.New(sha256.New, []byte(secKey))
	_, err = mac.Write([]byte(params.Encode()))
	if err != nil {
		return nil, err
	}
	signature := hex.EncodeToString(mac.Sum(nil))
	req.URL.RawQuery = params.Encode() + "&signature=" + signature

	res, err := h.HttpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to request command %s", path)
	}
	defer res.Body.Close()
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch result of command %s", path)
	}
	return resBody, err
}
func (h *BinanceApi) coins() ([]string,error) {
	h.currencyM.Lock()
	defer h.currencyM.Unlock()
	coins := make([]string,0)
	if len(h.currencyPairs) != 0 {
		coinsWithDup := make([]string,0)
		for _,v := range h.currencyPairs {
			coinsWithDup=append(coinsWithDup,v.Settlement)
			coinsWithDup=append(coinsWithDup,v.Trading)
		}
		m := make(map[string]struct{})
		for _, element := range coinsWithDup {
			// mapでは、第二引数にその値が入っているかどうかの真偽値が入っている
			if _, ok := m[element]; !ok {
				m[element] = struct{}{}
				coins = append(coins, element)
			}
		}
		return coins, nil
	}
	url := h.publicApiUrl("/api/v1/exchangeInfo")
	req, err := requestGetAsChrome(url)
	if err != nil {
		return coins, errors.Wrapf(err, "failed to fetch %s", url)
	}
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return coins, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()
	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return coins, errors.Wrapf(err, "failed to fetch %s", url)
	}
	value := gjson.ParseBytes(byteArray)
	coinsWithDup := make([]string,0)
	for _,v:= range value.Get("symbols").Array() {
		coinsWithDup = append(coinsWithDup, v.Get("baseAsset").String())
		coinsWithDup = append(coinsWithDup, v.Get("quoteAsset").String())
	}
	m := make(map[string]struct{})
	for _, element := range coinsWithDup {
		// mapでは、第二引数にその値が入っているかどうかの真偽値が入っている
		if _, ok := m[element]; !ok {
			m[element] = struct{}{}
			coins = append(coins, element)
		}
	}
	return coins,nil
}
func (h *BinanceApi) CurrencyPairs() ([]models.CurrencyPair, error) {
	h.currencyM.Lock()
	defer h.currencyM.Unlock()
	if len(h.currencyPairs) != 0 {
		return h.currencyPairs, nil
	}
	url := h.publicApiUrl("/api/v1/exchangeInfo")
	req, err := requestGetAsChrome(url)
	if err != nil {
		return h.currencyPairs, errors.Wrapf(err, "failed to fetch %s", url)
	}
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return h.currencyPairs, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()
	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return h.currencyPairs, errors.Wrapf(err, "failed to fetch %s", url)
	}
	currencyPairs := make([]models.CurrencyPair, 0)
	value := gjson.ParseBytes(byteArray)
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

func (h *BinanceApi) TradeFeeRates() (map[string]map[string]TradeFee, error) {
	url := public.BINANCE_BASE_URL + "/api/v3/account"
	resp, err := h.HttpClient.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	value := gjson.ParseBytes(byteArray)
	makerFee := value.Get("makerCommission").Num / 10000
	takerFee := value.Get("takerCommission").Num / 10000
	traderFeeMap := make(map[string]map[string]TradeFee)
	for _, pair := range h.currencyPairs {
		m, ok := traderFeeMap[pair.Trading]
		if !ok {
			m = make(map[string]TradeFee)
			traderFeeMap[pair.Trading] = m
		}
		m[pair.Settlement] = TradeFee{makerFee, takerFee}

	}
	return traderFeeMap, nil
}

func (b *BinanceApi) TradeFeeRate(trading string, settlement string) (TradeFee, error) {
	feeMap, err := b.TradeFeeRates()
	if err != nil {
		return TradeFee{}, err
	}
	return feeMap[trading][settlement], nil
}

type BinanceTransferFeeResponse struct {
	response []byte
	Currency string
	err      error
}

type binanceTransferFeeMap map[string]float64
type binanceTransferFeeSyncMap struct {
	binanceTransferFeeMap
	m *sync.Mutex
}

func (sm *binanceTransferFeeSyncMap) Set(currency string, fee float64) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.binanceTransferFeeMap[currency] = fee
}
func (sm *binanceTransferFeeSyncMap) GetAll() map[string]float64 {
	sm.m.Lock()
	defer sm.m.Unlock()
	return sm.binanceTransferFeeMap
}

func (h *BinanceApi) TransferFee() (map[string]float64, error) {
	transferFeeMap := binanceTransferFeeSyncMap{make(binanceTransferFeeMap), new(sync.Mutex)}
	/*params := url.Values{}
	h.buildParamsSigned(&params)
	apiKey, err := h.ApiKeyFunc()
	if err != nil {
		return nil, err
	}
	path := h.BaseURL + "/wapi/v3/assetDetail.html" + params.Encode()
	respmap, err := helpers.HttpGet2(&h.HttpClient, path, map[string]string{"X-MBX-APIKEY": apiKey})
	fmt.Println(respmap)

	ad := respmap["assetDetail"]
	if ad == nil {
		return nil, errors.New("nothing trade balance")
	}
	adv := ad.(map[string]interface{})
	for k, v := range adv {
		vv := v.(map[string]interface{})
		fee := vv["withdrawFee"].(float64)
		transferFeeMap.Set(strings.ToUpper(k), fee)
	}*/
	transferFeeMap.Set("CTR",35.00000000)
	transferFeeMap.Set("MATIC",0.46000000)
	transferFeeMap.Set("IOTX",232.00000000)
	transferFeeMap.Set("CDT",96.00000000)
	transferFeeMap.Set("DOCK",91.00000000)
	transferFeeMap.Set("STX",10.00000000)
	transferFeeMap.Set("DENT",5084.00000000)
	transferFeeMap.Set("AION",0.10000000)
	transferFeeMap.Set("NPXS",5714.00000000)
	transferFeeMap.Set("BCPT",0.62000000)
	transferFeeMap.Set("VIBE",58.00000000)
	transferFeeMap.Set("DGD",0.05100000)
	transferFeeMap.Set("SUB",40.00000000)
	transferFeeMap.Set("ZRX",3.80000000)
	transferFeeMap.Set("BCD",0.01000000)
	transferFeeMap.Set("POA",62.00000000)
	transferFeeMap.Set("AE",0.10000000)
	transferFeeMap.Set("IOST",151.00000000)
	transferFeeMap.Set("BCH",0.00100000)
	transferFeeMap.Set("POE",417.00000000)
	transferFeeMap.Set("OMG",1.21000000)
	transferFeeMap.Set("BAND",2.94000000)
	transferFeeMap.Set("HOT",1100.00000000)
	transferFeeMap.Set("BTC",0.00000210)
	transferFeeMap.Set("NKN",41.00000000)
	transferFeeMap.Set("CVC",33.00000000)
	transferFeeMap.Set("IOTA",0.50000000)
	transferFeeMap.Set("BTG",0.00100000)
	transferFeeMap.Set("BCX",0.50000000)
	transferFeeMap.Set("ARK",0.20000000)
	transferFeeMap.Set("BTM",5.00000000)
	transferFeeMap.Set("TRIG",50.00000000)
	transferFeeMap.Set("RCN",18.00000000)
	transferFeeMap.Set("ARN",0.10000000)
	transferFeeMap.Set("KEY",448.00000000)
	transferFeeMap.Set("BTS",1.00000000)
	transferFeeMap.Set("BTT",47.00000000)
	transferFeeMap.Set("ONE",2.36000000)
	transferFeeMap.Set("ONG",5.40000000)
	transferFeeMap.Set("ANKR",8.32000000)
	transferFeeMap.Set("GNT",24.00000000)
	transferFeeMap.Set("ALGO",0.01000000)
	transferFeeMap.Set("SC",0.10000000)
	transferFeeMap.Set("ONT",1.00000000)
	transferFeeMap.Set("PPT",1.71000000)
	transferFeeMap.Set("RDN",7.17000000)
	transferFeeMap.Set("PIVX",0.20000000)
	transferFeeMap.Set("ARDR",2.00000000)
	transferFeeMap.Set("AST",38.00000000)
	transferFeeMap.Set("CLOAK",0.02000000)
	transferFeeMap.Set("MANA",37.00000000)
	transferFeeMap.Set("NEBL",0.01000000)
	transferFeeMap.Set("VTHO",21.00000000)
	transferFeeMap.Set("MEETONE",300.00000000)
	transferFeeMap.Set("QSP",89.00000000)
	transferFeeMap.Set("SALT",5.20000000)
	transferFeeMap.Set("STORM",636.00000000)
	transferFeeMap.Set("ATD",100.00000000)
	transferFeeMap.Set("ICN",3.40000000)
	transferFeeMap.Set("ZEC",0.00500000)
	transferFeeMap.Set("REN",24.00000000)
	transferFeeMap.Set("APPC",30.00000000)
	transferFeeMap.Set("JEX",50.00000000)
	transferFeeMap.Set("REP",0.08800000)
	transferFeeMap.Set("ADA",1.00000000)
	transferFeeMap.Set("ELF",14.00000000)
	transferFeeMap.Set("REQ",59.00000000)
	transferFeeMap.Set("STORJ",7.67000000)
	transferFeeMap.Set("ICX",0.02000000)
	transferFeeMap.Set("ADD",100.00000000)
	transferFeeMap.Set("LOOM",48.00000000)
	transferFeeMap.Set("ZEN",0.00200000)
	transferFeeMap.Set("YOYO",71.00000000)
	transferFeeMap.Set("PAX",0.87000000)
	transferFeeMap.Set("DUSK",0.35000000)
	transferFeeMap.Set("DOGE",50.00000000)
	transferFeeMap.Set("HBAR",1.00000000)
	transferFeeMap.Set("RVN",1.00000000)
	transferFeeMap.Set("NANO",0.01000000)
	transferFeeMap.Set("WAVES",0.00200000)
	transferFeeMap.Set("CHZ",1.46000000)
	transferFeeMap.Set("ADX",11.00000000)
	transferFeeMap.Set("XRP",0.07000000)
	transferFeeMap.Set("WPR",102.00000000)
	transferFeeMap.Set("KAVA",0.01600000)
	transferFeeMap.Set("HCC",0.00050000)
	transferFeeMap.Set("SYS",1.00000000)
	transferFeeMap.Set("COCOS",1210.00000000)
	transferFeeMap.Set("TUSD",0.01500000)
	transferFeeMap.Set("GAS",0.00000000)
	transferFeeMap.Set("WABI",5.46000000)
	transferFeeMap.Set("STRAT",0.10000000)
	transferFeeMap.Set("ENG",1.95000000)
	transferFeeMap.Set("THETA",0.10000000)
	transferFeeMap.Set("ENJ",11.00000000)
	transferFeeMap.Set("WAN",0.10000000)
	transferFeeMap.Set("OAX",14.00000000)
	transferFeeMap.Set("GRS",0.20000000)
	transferFeeMap.Set("PERL",33.00000000)
	transferFeeMap.Set("TFUEL",2.68000000)
	transferFeeMap.Set("LEND",60.00000000)
	transferFeeMap.Set("DLT",22.00000000)
	transferFeeMap.Set("TROY",5.00000000)
	transferFeeMap.Set("LLT",100.00000000)
	transferFeeMap.Set("SBTC",0.00050000)
	transferFeeMap.Set("XTZ",0.50000000)
	transferFeeMap.Set("AGI",41.00000000)
	transferFeeMap.Set("MOD",5.00000000)
	transferFeeMap.Set("EON",10.00000000)
	transferFeeMap.Set("EOP",5.00000000)
	transferFeeMap.Set("EOS",0.10000000)
	transferFeeMap.Set("GO",0.01000000)
	transferFeeMap.Set("NCASH",1008.00000000)
	transferFeeMap.Set("OST",90.00000000)
	transferFeeMap.Set("HC",0.00500000)
	transferFeeMap.Set("ZIL",161.00000000)
	transferFeeMap.Set("SKY",0.02000000)
	transferFeeMap.Set("NAS",0.10000000)
	transferFeeMap.Set("XEM",4.00000000)
	transferFeeMap.Set("NAV",0.20000000)
	transferFeeMap.Set("GTO",1.37000000)
	transferFeeMap.Set("CTXC",0.10000000)
	transferFeeMap.Set("WTC",1.84000000)
	transferFeeMap.Set("XVG",0.10000000)
	transferFeeMap.Set("TNB",417.00000000)
	transferFeeMap.Set("BCHSV",0.00010000)
	transferFeeMap.Set("DNT",153.00000000)
	transferFeeMap.Set("STEEM",0.01000000)
	transferFeeMap.Set("TNT",12.00000000)
	transferFeeMap.Set("KMD",0.00200000)
	transferFeeMap.Set("IQ",50.00000000)
	transferFeeMap.Set("CMT",1.00000000)
	transferFeeMap.Set("MITH",1.69000000)
	transferFeeMap.Set("ERD",6.71000000)
	transferFeeMap.Set("CND",112.00000000)
	transferFeeMap.Set("FTM",1.24000000)
	transferFeeMap.Set("POWR",22.00000000)
	transferFeeMap.Set("KNC",4.50000000)
	transferFeeMap.Set("GVT",0.84000000)
	transferFeeMap.Set("WINGS",20.00000000)
	transferFeeMap.Set("CHAT",100.00000000)
	transferFeeMap.Set("RLC",1.67000000)
	transferFeeMap.Set("PHB",3.30000000)
	transferFeeMap.Set("BGBP",0.67000000)
	transferFeeMap.Set("ATOM",0.00500000)
	transferFeeMap.Set("BLZ",42.00000000)
	transferFeeMap.Set("SNM",56.00000000)
	transferFeeMap.Set("SNT",82.00000000)
	transferFeeMap.Set("FUN",268.00000000)
	transferFeeMap.Set("SNGLS",98.00000000)
	transferFeeMap.Set("COS",1.25000000)
	transferFeeMap.Set("QKC",228.00000000)
	transferFeeMap.Set("FET",15.00000000)
	transferFeeMap.Set("ETC",0.01000000)
	transferFeeMap.Set("ETF",1.00000000)
	transferFeeMap.Set("BNB",0.00100000)
	transferFeeMap.Set("CELR",192.00000000)
	transferFeeMap.Set("ETH",0.00010000)
	transferFeeMap.Set("MCO",0.23000000)
	transferFeeMap.Set("NEO",0.00000000)
	transferFeeMap.Set("TOMO",0.05600000)
	transferFeeMap.Set("LRC",35.00000000)
	transferFeeMap.Set("MTH",74.00000000)
	transferFeeMap.Set("XZC",0.02000000)
	transferFeeMap.Set("GXS",0.30000000)
	transferFeeMap.Set("MTL",3.02000000)
	transferFeeMap.Set("VET",100.00000000)
	transferFeeMap.Set("BNT",3.38000000)
	transferFeeMap.Set("USDT",0.87000000)
	transferFeeMap.Set("QLC",1.00000000)
	transferFeeMap.Set("USDS",0.01500000)
	transferFeeMap.Set("MDA",1.69000000)
	transferFeeMap.Set("DASH",0.00200000)
	transferFeeMap.Set("EDO",3.74000000)
	transferFeeMap.Set("AMB",40.00000000)
	transferFeeMap.Set("FUEL",232.00000000)
	transferFeeMap.Set("TRX",60.00000000)
	transferFeeMap.Set("LSK",0.10000000)
	transferFeeMap.Set("NULS",3.02000000)
	transferFeeMap.Set("BEAM",0.10000000)
	transferFeeMap.Set("DCR",0.01000000)
	transferFeeMap.Set("DATA",45.00000000)
	transferFeeMap.Set("LTC",0.00034000)
	transferFeeMap.Set("USDC",0.87000000)
	transferFeeMap.Set("WIN",140.00000000)
	transferFeeMap.Set("EVX",3.27000000)
	transferFeeMap.Set("NXS",0.02000000)
	transferFeeMap.Set("CBM",200.00000000)
	transferFeeMap.Set("INS",5.25000000)
	transferFeeMap.Set("XLM",0.01000000)
	transferFeeMap.Set("LINK",0.43000000)
	transferFeeMap.Set("MFT",864.00000000)
	transferFeeMap.Set("QTUM",0.01000000)
	transferFeeMap.Set("LUN",0.94000000)
	transferFeeMap.Set("BQX",33.00000000)
	transferFeeMap.Set("POLY",41.00000000)
	transferFeeMap.Set("VIB",39.00000000)
	transferFeeMap.Set("VIA",0.01000000)
	transferFeeMap.Set("BAT",4.81000000)
	transferFeeMap.Set("BRD",3.56000000)
	transferFeeMap.Set("BUSD",0.01500000)
	transferFeeMap.Set("ARPA",1.36000000)
	transferFeeMap.Set("XMR",0.00010000)
	return transferFeeMap.GetAll(), nil
}

const SERVER_TIME_URL = "time"

func (bn *BinanceApi) setTimeOffset() error {
	respmap, err := helpers.HttpGet(&bn.HttpClient, bn.apiV1+SERVER_TIME_URL)
	if err != nil {
		return err
	}

	stime := int64(helpers.ToInt(respmap["serverTime"]))
	st := time.Unix(stime/1000, 1000000*(stime%1000))
	lt := time.Now()
	offset := st.Sub(lt).Nanoseconds()
	bn.timeoffset = int64(offset)
	return nil
}

func (bn *BinanceApi) buildParamsSigned(postForm *url.Values) error {
	secretKey, err := bn.SecretKeyFunc()
	if err != nil {
		return err
	}
	postForm.Set("recvWindow", "60000")
	tonce := strconv.FormatInt(time.Now().UnixNano()+bn.timeoffset, 10)[0:13]
	postForm.Set("timestamp", tonce)
	payload := postForm.Encode()
	sign, _ := helpers.GetParamHmacSHA256Sign(secretKey, payload)
	postForm.Set("signature", sign)
	return nil
}

const ACCOUNT_URI = "account?"

func (h *BinanceApi) Balances() (map[string]float64, error) {
	params := url.Values{}
	h.buildParamsSigned(&params)
	apiKey, err := h.ApiKeyFunc()
	if err != nil {
		return nil, err
	}
	path := h.apiV3 + ACCOUNT_URI + params.Encode()
	respmap, err := helpers.HttpGet2(&h.HttpClient, path, map[string]string{"X-MBX-APIKEY": apiKey})
	if err != nil {
		return nil,err
	}
	m := make(map[string]float64)
	coins,err := h.coins()
	if err != nil {
		return nil,err
	}
	for _,v := range coins {
		m[v] = 0
	}

	balances := respmap["balances"].([]interface{})
	for _, v := range balances {
		vv := v.(map[string]interface{})
		currency := vv["asset"].(string)
		currency = strings.ToUpper(currency)
		m[currency] = helpers.ToFloat64(vv["free"])
	}
	return m, nil
}

type BinanceBalance struct {
	T       string
	Balance float64
}

func (h *BinanceApi) CompleteBalances() (map[string]*models.Balance, error) {
	params := url.Values{}
	h.buildParamsSigned(&params)
	apiKey, err := h.ApiKeyFunc()
	if err != nil {
		return nil, err
	}
	path := h.apiV3 + ACCOUNT_URI + params.Encode()
	respmap, err := helpers.HttpGet2(&h.HttpClient, path, map[string]string{"X-MBX-APIKEY": apiKey})

	m := make(map[string]*models.Balance)
	balances := respmap["balances"].([]interface{})
	for _, v := range balances {
		vv := v.(map[string]interface{})
		currency := vv["asset"].(string)
		currency = strings.ToUpper(currency)
		m[currency] = &models.Balance{
			Available: helpers.ToFloat64(vv["free"]),
			OnOrders:  helpers.ToFloat64(vv["locked"]),
		}
	}
	return m, nil
}

func (h *BinanceApi) CompleteBalance(coin string) (*models.Balance, error) {
	m, err := h.CompleteBalances()
	if err != nil {
		return nil, err
	}
	b, ok := m[coin]
	if !ok {
		return nil, errors.Errorf("no coin %s", coin)
	}
	return b, nil
}

type BinanceActiveOrderResponse struct {
	response   []byte
	Trading    string
	Settlement string
	err        error
}

func (h *BinanceApi) ActiveOrders() ([]*models.Order, error) {
	return nil, errors.New("not implemented")
}

func (h *BinanceApi) Order(trading string, settlement string, ordertype models.OrderType, price float64, amount float64) (string, error) {
	params := &url.Values{}

	symbol := strings.ToUpper(fmt.Sprintf("%s%s", trading, settlement))
	params.Set("symbol", symbol)

	if ordertype == models.Bid {
		params.Set("side", "SELL")
	} else if ordertype == models.Ask {
		params.Set("side", "BUY")
	} else {
		return "", errors.Errorf("unknown order type %d", ordertype)
	}
	params.Set("type", "LIMIT")
	params.Set("timeInForce", "GTC")
	precise, err := h.precise(trading, settlement)
	if err != nil {
		return "", err
	}
	params.Set("quantity", FloorFloat64ToStr(amount, precise.AmountPrecision))
	params.Set("price", FloorFloat64ToStr(price, precise.PricePrecision))

	byteArray, err := h.privateApi("POST", "/api/v3/order", params)
	if err != nil {
		return "", err
	}
	value := gjson.ParseBytes(byteArray)

	return value.Get("clientOrderId").Str, nil
}

func (h *BinanceApi) Transfer(typ string, addr string, amount float64, additionalFee float64) error {
	params := &url.Values{}
	amountStr := strconv.FormatFloat(amount, 'f', 4, 64)
	params.Set("asset", typ)
	params.Set("address", addr)
	params.Set("amount", amountStr)
	_, err := h.privateApi("POST", "/wapi/v3/withdraw.html", params)
	return err
}

func (h *BinanceApi) CancelOrder(trading string, settlement string,
	ordertype models.OrderType, orderNumber string) error {
	params := &url.Values{}
	params.Set("symbol", trading+settlement)
	params.Set("origClientOrderId", orderNumber)

	bs, err := h.privateApi("DELETE", "/api/v3/order", params)
	if err != nil {
		return errors.Wrapf(err, "failed to cancel order")
	}
	value := gjson.ParseBytes(bs)
	if orderNumber == value.Get("origClientOrderId").String() {
		return nil
	}
	return errors.Errorf("failed to cancel order %s", orderNumber)
}

func (h *BinanceApi) IsOrderFilled(trading string, settlement string, orderNumber string) (bool, error) {
	params := &url.Values{}
	params.Set("symbol", trading+settlement)
	params.Set("origClientOrderId", orderNumber)
	bs, err := h.privateApi("GET", "/api/v3/order", params)
	if err != nil {
		return false, errors.Wrapf(err, "failed to cancel order")
	}
	value := gjson.ParseBytes(bs)
	if value.Get("isWorking").Bool() {
		return true, nil
	}
	return false, nil
}

func (h *BinanceApi) Address(c string) (string, error) {
	params := &url.Values{}
	params.Set("asset", c)
	bs, err := h.privateApi("GET", "/wapi/v3/depositAddress.html", params)
	if err != nil {
		return "", errors.Wrapf(err, "failed to cancel order")
	}
	value := gjson.ParseBytes(bs)
	return value.Array()[0].Get("address").Str, nil
}
