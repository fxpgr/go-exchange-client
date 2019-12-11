package private

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/Jeffail/gabs"
	"github.com/antonholmquist/jason"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	P2PB2B_BASE_URL = "https://api.p2pb2b.io/api/v1"
)

func NewP2pb2bApi(apikey func() (string, error), apisecret func() (string, error)) (*P2pb2bApi, error) {
	hitbtcPublic, err := public.NewP2pb2bPublicApi()
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

	return &P2pb2bApi{
		BaseURL:           P2PB2B_BASE_URL,
		RateCacheDuration: 30 * time.Second,
		ApiKeyFunc:        apikey,
		SecretKeyFunc:     apisecret,
		settlements:       uniq,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		rt:                &http.Transport{},

		m: new(sync.Mutex),
	}, nil
}

type P2pb2bApi struct {
	ApiKeyFunc        func() (string, error)
	SecretKeyFunc     func() (string, error)
	BaseURL           string
	RateCacheDuration time.Duration
	HttpClient        http.Client
	rt                *http.Transport
	settlements       []string

	volumeMap       map[string]map[string]float64
	rateMap         map[string]map[string]float64
	precisionMap    map[string]map[string]models.Precisions
	rateLastUpdated time.Time

	m *sync.Mutex
}

func (h *P2pb2bApi) privateApiUrl() string {
	return h.BaseURL
}

func (h *P2pb2bApi) publicApiUrl(command string) string {
	return h.BaseURL + command
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
			PricePrecision:  public.Precision(last),
			AmountPrecision: public.Precision(volume),
		}
	}
	return nil
}

func (h *P2pb2bApi) precise(trading string, settlement string) (*models.Precisions, error) {
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

func (h *P2pb2bApi) privateApi(method string, path string, params *url.Values) ([]byte, error) {
	apiKey, err := h.ApiKeyFunc()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	secretKey, err := h.SecretKeyFunc()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	urlStr := h.BaseURL + path
	if strings.ToUpper(method) == "GET" {
		urlStr = urlStr + "?" + params.Encode()
	}

	reader := bytes.NewReader([]byte(params.Encode()))
	req, err := http.NewRequest(method, urlStr, reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request command %s", path)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	req.Header.Set("Accept", "application/json")
	var b bytes.Buffer
	b.WriteString(method)
	b.WriteString(path)
	b.WriteString(params.Encode())
	t := strconv.FormatInt((time.Now().UnixNano() / 1000000), 10)
	p := []byte(t + b.String())
	hm := hmac.New(sha256.New, []byte(secretKey))
	hm.Write(p)
	s := base64.StdEncoding.EncodeToString(hm.Sum(nil))
	req.Header.Set("KC-API-KEY", apiKey)
	req.Header.Set("KC-API-TIMESTAMP", t)
	req.Header.Set(
		"KC-API-SIGN", s,
	)
	req.Header.Set("KC-API-PASSPHRASE", apiKey)
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

func (h *P2pb2bApi) TradeFeeRates() (map[string]map[string]TradeFee, error) {

	url := h.publicApiUrl("/api/v1/market/allTickers")
	req, err := requestGetAsChrome(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	defer resp.Body.Close()
	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch %s", url)
	}
	value := gjson.ParseBytes(byteArray)
	traderFeeMap := make(map[string]map[string]TradeFee)
	for _, v := range value.Get("data.ticker").Array() {
		currencies := strings.Split(v.Get("symbol").Str, "-")
		if len(currencies) < 2 {
			continue
		}
		trading := currencies[0]
		settlement := currencies[1]

		feeRate := 0.001
		m, ok := traderFeeMap[trading]
		if !ok {
			m = make(map[string]TradeFee)
			traderFeeMap[trading] = m
		}
		m[settlement] = TradeFee{feeRate, feeRate}
	}
	return traderFeeMap, nil
}

func (b *P2pb2bApi) TradeFeeRate(trading string, settlement string) (TradeFee, error) {
	feeMap, err := b.TradeFeeRates()
	if err != nil {
		return TradeFee{}, err
	}
	return feeMap[trading][settlement], nil
}

type P2pb2bTransferFeeResponse struct {
	response []byte
	Currency string
	err      error
}

func (h *P2pb2bApi) TransferFee() (map[string]float64, error) {
	transferFeeMap := kucoinTransferFeeSyncMap{make(map[string]float64), new(sync.Mutex)}
	transferFeeMap.Set("BTC", 0.001)
	transferFeeMap.Set("ETH", 0.015)
	transferFeeMap.Set("USDT", 15)
	transferFeeMap.Set("1GOLD", 0.02)
	transferFeeMap.Set("1SG", 2)
	transferFeeMap.Set("1UP", 500)
	transferFeeMap.Set("3DC", 50)
	transferFeeMap.Set("AAA", 5000)
	transferFeeMap.Set("ABBC", 4)
	transferFeeMap.Set("ACA", 2000)
	transferFeeMap.Set("ADK", 0)
	transferFeeMap.Set("ADN", 100)
	transferFeeMap.Set("ADRX", 100)
	transferFeeMap.Set("AER", 25)
	transferFeeMap.Set("AFIN", 14)
	transferFeeMap.Set("AFO", 0)
	transferFeeMap.Set("AGI", 25)
	transferFeeMap.Set("AGNT", 5)
	transferFeeMap.Set("AGRO", 0.25)
	transferFeeMap.Set("AKI", 0.1)
	transferFeeMap.Set("ALFA", 1)
	transferFeeMap.Set("ALLBI", 50)
	transferFeeMap.Set("AMR", 10)
	transferFeeMap.Set("APG", 10000)
	transferFeeMap.Set("ARAW", 3000)
	transferFeeMap.Set("ARMR", 6)
	transferFeeMap.Set("ARRR", 20)
	transferFeeMap.Set("ASG", 5000)
	transferFeeMap.Set("ASTR", 0)
	transferFeeMap.Set("AYA", 300)
	transferFeeMap.Set("BAT", 3)
	transferFeeMap.Set("BBTA", 0.5)
	transferFeeMap.Set("BCCN", 200)
	transferFeeMap.Set("BCH", 0.0045)
	transferFeeMap.Set("BCL", 0.12)
	transferFeeMap.Set("BD", 0)
	transferFeeMap.Set("BHIG", 20)
	transferFeeMap.Set("BIRD", 500)
	transferFeeMap.Set("BIT", 0.1)
	transferFeeMap.Set("BLOK", 10)
	transferFeeMap.Set("BLTV", 1)
	transferFeeMap.Set("BNB", 0.05)
	transferFeeMap.Set("BNT", 1.6)
	transferFeeMap.Set("BNY", 300)
	transferFeeMap.Set("BOR", 0)
	transferFeeMap.Set("BOTX", 200)
	transferFeeMap.Set("BPK", 100000)
	transferFeeMap.Set("BPX", 200)
	transferFeeMap.Set("BQTX", 10)
	transferFeeMap.Set("BRC", 1)
	transferFeeMap.Set("BRT", 3500)
	transferFeeMap.Set("BST", 15)
	transferFeeMap.Set("BTG", 0.1)
	transferFeeMap.Set("BTT", 0)
	transferFeeMap.Set("BUX", 0.4)
	transferFeeMap.Set("BWN", 5)
	transferFeeMap.Set("C20", 5)
	transferFeeMap.Set("C3", 0.4)
	transferFeeMap.Set("CALL", 40)
	transferFeeMap.Set("CAND", 0)
	transferFeeMap.Set("CBC", 20)
	transferFeeMap.Set("CCA", 0.1)
	transferFeeMap.Set("CCC", 1)
	transferFeeMap.Set("CCC1", 4)
	transferFeeMap.Set("CCXX", 0.1)
	transferFeeMap.Set("CFC", 10000)
	transferFeeMap.Set("CITY", 10)
	transferFeeMap.Set("CLM", 500)
	transferFeeMap.Set("CLN", 0)
	transferFeeMap.Set("CMC", 200)
	transferFeeMap.Set("CMK", 2.5)
	transferFeeMap.Set("COI", 2)
	transferFeeMap.Set("CRAD", 10)
	transferFeeMap.Set("CRC", 15)
	transferFeeMap.Set("CRGO", 100)
	transferFeeMap.Set("CRON", 1)
	transferFeeMap.Set("CRYP", 1000)
	transferFeeMap.Set("CSNP", 2)
	transferFeeMap.Set("CTAG", 300)
	transferFeeMap.Set("CUR8", 10)
	transferFeeMap.Set("CUT", 40)
	transferFeeMap.Set("CVC", 13)
	transferFeeMap.Set("DAI", 1.5)
	transferFeeMap.Set("DASH", 0.01)
	transferFeeMap.Set("DFS", 20)
	transferFeeMap.Set("DIVO", 100)
	transferFeeMap.Set("DOGE", 300)
	transferFeeMap.Set("DPT", 0.05)
	transferFeeMap.Set("DRA", 1000)
	transferFeeMap.Set("DRGB", 1)
	transferFeeMap.Set("DSLA", 6000)
	transferFeeMap.Set("DSYS", 100)
	transferFeeMap.Set("DUC", 3000)
	transferFeeMap.Set("DXP", 7)
	transferFeeMap.Set("DYNMT", 50)
	transferFeeMap.Set("ECHT", 300)
	transferFeeMap.Set("ECOREAL", 5)
	transferFeeMap.Set("ECP", 40)
	transferFeeMap.Set("ECTE", 30)
	transferFeeMap.Set("EDC", 100)
	transferFeeMap.Set("EDR", 1500)
	transferFeeMap.Set("EDT", 10)
	transferFeeMap.Set("ELE", 15)
	transferFeeMap.Set("EMBR", 0.5)
	transferFeeMap.Set("ENJ", 7)
	transferFeeMap.Set("EOS", 0)
	transferFeeMap.Set("ERG", 0.3)
	transferFeeMap.Set("ERK", 50)
	transferFeeMap.Set("ESAX", 50)
	transferFeeMap.Set("ESCB", 10)
	transferFeeMap.Set("EST", 8)
	transferFeeMap.Set("ETC", 0.2)
	transferFeeMap.Set("EUNO", 30)
	transferFeeMap.Set("EVN", 100)
	transferFeeMap.Set("EVR", 100)
	transferFeeMap.Set("EVX", 1.5)
	transferFeeMap.Set("EVY", 1000)
	transferFeeMap.Set("FAIRC", 80)
	transferFeeMap.Set("FIH", 20)
	transferFeeMap.Set("FLXC", 250)
	transferFeeMap.Set("FOIN", 0.0007)
	transferFeeMap.Set("0", 7000000)
	transferFeeMap.Set("FST", 1)
	transferFeeMap.Set("FXP", 2500)
	transferFeeMap.Set("FYE", 0.1)
	transferFeeMap.Set("GAS", 1)
	transferFeeMap.Set("GBTC", 5)
	transferFeeMap.Set("GC", 200)
	transferFeeMap.Set("GECA", 10)
	transferFeeMap.Set("GEX", 14)
	transferFeeMap.Set("GFCS", 2)
	transferFeeMap.Set("GFUN", 2000)
	transferFeeMap.Set("GLC", 20)
	transferFeeMap.Set("GNT", 30)
	transferFeeMap.Set("GNY", 10)
	transferFeeMap.Set("GOL", 90)
	transferFeeMap.Set("GOXT", 25)
	transferFeeMap.Set("GRS", 4)
	transferFeeMap.Set("GSH", 4)
	transferFeeMap.Set("GST", 1000)
	transferFeeMap.Set("GT", 1.4)
	transferFeeMap.Set("GWP", 1000)
	transferFeeMap.Set("HEDG", 15)
	transferFeeMap.Set("HELP", 0.1)
	transferFeeMap.Set("HLX", 0)
	transferFeeMap.Set("HNT", 2)
	transferFeeMap.Set("HOT", 800)
	transferFeeMap.Set("HRD", 25)
	transferFeeMap.Set("HYPX", 7500)
	transferFeeMap.Set("ICT", 250)
	transferFeeMap.Set("IFR", 1.5)
	transferFeeMap.Set("IFV", 80)
	transferFeeMap.Set("ILC", 20)
	transferFeeMap.Set("ILK", 100)
	transferFeeMap.Set("IMPCN", 10)
	transferFeeMap.Set("IMT", 4000)
	transferFeeMap.Set("INL", 0)
	transferFeeMap.Set("IOUX", 4)
	transferFeeMap.Set("JCT", 35)
	transferFeeMap.Set("JNB", 0.03)
	transferFeeMap.Set("JWL", 10)
	transferFeeMap.Set("KAM", 5)
	transferFeeMap.Set("KBC", 30)
	transferFeeMap.Set("KEP", 50)
	transferFeeMap.Set("KEY", 280)
	transferFeeMap.Set("KICK", 1000)
	transferFeeMap.Set("KIN", 20000)
	transferFeeMap.Set("KNC", 5)
	transferFeeMap.Set("KRI", 30)
	transferFeeMap.Set("KRS", 1)
	transferFeeMap.Set("KSH", 1)
	transferFeeMap.Set("LBN", 5)
	transferFeeMap.Set("LDN", 2)
	transferFeeMap.Set("LEO", 17)
	transferFeeMap.Set("LEVL", 2.8)
	transferFeeMap.Set("LHT", 200)
	transferFeeMap.Set("LINA", 14)
	transferFeeMap.Set("LINK", 0.5)
	transferFeeMap.Set("LK", 100)
	transferFeeMap.Set("LOT", 100)
	transferFeeMap.Set("LST", 10)
	transferFeeMap.Set("LTC", 0.01)
	transferFeeMap.Set("LVL", 2000)
	transferFeeMap.Set("LVX", 3)
	transferFeeMap.Set("MANA", 20)
	transferFeeMap.Set("MAR", 700)
	transferFeeMap.Set("MAYA", 3)
	transferFeeMap.Set("MB8", 30)
	transferFeeMap.Set("MBC", 7000)
	transferFeeMap.Set("MHC", 300)
	transferFeeMap.Set("MIC", 30)
	transferFeeMap.Set("MIN", 1)
	transferFeeMap.Set("MINX", 50)
	transferFeeMap.Set("MKK", 10)
	transferFeeMap.Set("MM", 20)
	transferFeeMap.Set("MNC", 300)
	transferFeeMap.Set("MONGOc", 5)
	transferFeeMap.Set("MPG", 150)
	transferFeeMap.Set("MTCN", 70)
	transferFeeMap.Set("MTT", 5)
	transferFeeMap.Set("MoCo", 30)
	transferFeeMap.Set("N8V", 3.5)
	transferFeeMap.Set("NACRE", 0.02)
	transferFeeMap.Set("NBX", 30)
	transferFeeMap.Set("NCI", 20)
	transferFeeMap.Set("NEO", 0)
	transferFeeMap.Set("NEXT", 0.001)
	transferFeeMap.Set("NICASH", 0.7)
	transferFeeMap.Set("NOVA", 10)
	transferFeeMap.Set("NRV", 0.1)
	transferFeeMap.Set("NTRT", 0)
	transferFeeMap.Set("NVM", 0)
	transferFeeMap.Set("OCG", 0.6)
	transferFeeMap.Set("OWC", 10)
	transferFeeMap.Set("OXY", 750)
	transferFeeMap.Set("P2PX", 2000)
	transferFeeMap.Set("PAC", 2000)
	transferFeeMap.Set("PART", 0.5)
	transferFeeMap.Set("PAX", 20)
	transferFeeMap.Set("PDATA", 10)
	transferFeeMap.Set("PENG", 14000)
	transferFeeMap.Set("PHI", 10)
	transferFeeMap.Set("PITC", 1)
	transferFeeMap.Set("PIXBY", 5)
	transferFeeMap.Set("PLA", 2)
	transferFeeMap.Set("PLC", 0.001)
	transferFeeMap.Set("PLR", 2)
	transferFeeMap.Set("POLIS", 2)
	transferFeeMap.Set("POWR", 1)
	transferFeeMap.Set("PPT", 1)
	transferFeeMap.Set("PPY", 1.5)
	transferFeeMap.Set("PWMC", 2)
	transferFeeMap.Set("PYN", 15)
	transferFeeMap.Set("QBIT", 0.1)
	transferFeeMap.Set("QTUM", 0.4)
	transferFeeMap.Set("R", 10)
	transferFeeMap.Set("RALLY", 1000)
	transferFeeMap.Set("RBC", 1)
	transferFeeMap.Set("REL", 40000)
	transferFeeMap.Set("REMCO", 12)
	transferFeeMap.Set("RFOX", 30)
	transferFeeMap.Set("RIDE", 2)
	transferFeeMap.Set("RPD", 5000)
	transferFeeMap.Set("RTD", 20)
	transferFeeMap.Set("RUB_ADVCASH", 0)
	transferFeeMap.Set("RUB_CAPITALIST", 0)
	transferFeeMap.Set("RUB_PAYEER", 0)
	transferFeeMap.Set("RVC", 5)
	transferFeeMap.Set("RVT", 1000)
	transferFeeMap.Set("RWDS", 0.5)
	transferFeeMap.Set("S8", 80)
	transferFeeMap.Set("SCO", 20)
	transferFeeMap.Set("SEM", 10)
	transferFeeMap.Set("SET", 0.1)
	transferFeeMap.Set("SHMN", 4)
	transferFeeMap.Set("SHX", 1500)
	transferFeeMap.Set("SINS", 1)
	transferFeeMap.Set("SKI", 1)
	transferFeeMap.Set("SLP", 10)
	transferFeeMap.Set("SMLY", 0)
	transferFeeMap.Set("SNB", 1)
	transferFeeMap.Set("SNT", 50)
	transferFeeMap.Set("SON", 0.027)
	transferFeeMap.Set("SPAZ", 10)
	transferFeeMap.Set("SREUR", 50)
	transferFeeMap.Set("SRK", 10000)
	transferFeeMap.Set("STREAM", 0.5)
	transferFeeMap.Set("SWACH", 20)
	transferFeeMap.Set("SWAT", 1500)
	transferFeeMap.Set("SXP", 5)
	transferFeeMap.Set("SYN", 1)
	transferFeeMap.Set("SYNC", 1)
	transferFeeMap.Set("TBC", 300)
	transferFeeMap.Set("TCAT", 200)
	transferFeeMap.Set("TCC", 300)
	transferFeeMap.Set("THC", 600)
	transferFeeMap.Set("TIME", 0.5)
	transferFeeMap.Set("TKO", 20)
	transferFeeMap.Set("TLOS", 10)
	transferFeeMap.Set("TRVT", 2)
	transferFeeMap.Set("TRX", 35)
	transferFeeMap.Set("TTN", 400)
	transferFeeMap.Set("TTP", 2)
	transferFeeMap.Set("TTT", 2)
	transferFeeMap.Set("TUSD", 10)
	transferFeeMap.Set("TWINS", 5000)
	transferFeeMap.Set("TWQ", 0)
	transferFeeMap.Set("TYPE", 2300)
	transferFeeMap.Set("UNC", 10)
	transferFeeMap.Set("UNIS", 1)
	transferFeeMap.Set("UOS", 40)
	transferFeeMap.Set("VERI", 1)
	transferFeeMap.Set("VGW", 10)
	transferFeeMap.Set("VINCI", 1)
	transferFeeMap.Set("VTM", 100)
	transferFeeMap.Set("WAVES", 1)
	transferFeeMap.Set("WBET", 160000)
	transferFeeMap.Set("WEBD", 10)
	transferFeeMap.Set("WOLF", 1000)
	transferFeeMap.Set("WTC", 0.4)
	transferFeeMap.Set("WTM", 50)
	transferFeeMap.Set("XBASE", 150)
	transferFeeMap.Set("XBOND", 400)
	transferFeeMap.Set("XBX", 1500)
	transferFeeMap.Set("XCT", 20)
	transferFeeMap.Set("XDCE", 1500)
	transferFeeMap.Set("XEM", 10)
	transferFeeMap.Set("XLAB", 30)
	transferFeeMap.Set("XLM", 15)
	transferFeeMap.Set("XNB", 100)
	transferFeeMap.Set("XPGC", 350)
	transferFeeMap.Set("XRC", 0.04)
	transferFeeMap.Set("XSCC", 100)
	transferFeeMap.Set("XSD", 2)
	transferFeeMap.Set("XTT", 10)
	transferFeeMap.Set("XTZ", 0)
	transferFeeMap.Set("YAP", 2)
	transferFeeMap.Set("YBY", 200)
	transferFeeMap.Set("ZB", 25)
	transferFeeMap.Set("ZEON", 1000)
	transferFeeMap.Set("ZRX", 4)
	transferFeeMap.Set("ZUBE", 7000)
	transferFeeMap.Set("ZYN", 0.2)
	transferFeeMap.Set("iOWN", 0)
	transferFeeMap.Set("sBTC", 0.0003)
	return transferFeeMap.GetAll(), nil
}

func (h *P2pb2bApi) Balances() (map[string]float64, error) {
	m := make(map[string]float64)
	params := &url.Values{}
	byteArray, err := h.privateApi("GET", "/api/v1/accounts", params)
	if err != nil {
		return nil, err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObjectArray("data")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key data %s", json)
	}
	for _, v := range data {
		balanceStr, err := v.GetString("balance")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse balance on %s", json)
		}
		balance, err := strconv.ParseFloat(balanceStr, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse balance on %s", json)
		}
		availableStr, err := v.GetString("available")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse available on %s", json)
		}
		available, err := strconv.ParseFloat(availableStr, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse balance on %s", json)
		}
		freeze := balance - available
		currency, err := v.GetString("currency")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse currency on %s", json)
		}
		currency = strings.ToUpper(currency)
		m[currency] = balance - freeze
	}
	return m, nil
}

type P2pb2bBalance struct {
	T       string
	Balance float64
}

func (h *P2pb2bApi) CompleteBalances() (map[string]*models.Balance, error) {
	m := make(map[string]*models.Balance)
	params := &url.Values{}
	byteArray, err := h.privateApi("GET", "/api/v1/accounts", params)
	if err != nil {
		return nil, err
	}
	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json")
	}
	data, err := json.GetObjectArray("data")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse json key data %s", json)
	}
	for _, v := range data {
		balance, err := v.GetFloat64("balance")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse balance on %s", json)
		}
		available, err := v.GetFloat64("available")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse available on %s", json)
		}
		freeze := balance - available
		currency, err := v.GetString("currency")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse currency on %s", json)
		}
		currency = strings.ToUpper(currency)
		m[currency] = &models.Balance{
			Available: balance,
			OnOrders:  freeze,
		}
	}
	return m, nil
}

func (h *P2pb2bApi) CompleteBalance(coin string) (*models.Balance, error) {
	completeBalances, err := h.CompleteBalances()

	if err != nil {
		return nil, err
	}

	completeBalance, ok := completeBalances[coin]
	if !ok {
		return nil, errors.New("cannot find complete balance")
	}
	return completeBalance, nil
}

type P2pb2bActiveOrderResponse struct {
	response   []byte
	Trading    string
	Settlement string
	err        error
}

func (h *P2pb2bApi) ActiveOrders() ([]*models.Order, error) {
	return nil, errors.New("not implemented")
}

func (h *P2pb2bApi) Order(trading string, settlement string, ordertype models.OrderType, price float64, amount float64) (string, error) {
	params := &url.Values{}
	if ordertype == models.Bid {
		params.Set("type", "SELL")
	} else if ordertype == models.Ask {
		params.Set("type", "BUY")
	} else {
		return "", errors.Errorf("unknown order type %d", ordertype)
	}
	precise, err := h.precise(trading, settlement)
	if err != nil {
		return "", err
	}
	params.Set("price", FloorFloat64ToStr(price, precise.PricePrecision))
	params.Set("amount", FloorFloat64ToStr(amount, precise.AmountPrecision))

	symbol := strings.ToUpper(fmt.Sprintf("%s-%s", trading, settlement))
	params.Set("symbol", symbol)
	byteArray, err := h.privateApi("POST", "/v1/order", params)
	if err != nil {
		return "", err
	}

	json, err := jason.NewObjectFromBytes(byteArray)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json object")
	}
	data, err := json.GetObject("data")
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json data %s", json)
	}
	orderId, err := data.GetString("orderOid")
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse json orderId %s", json)
	}
	return orderId, nil
}

func (h *P2pb2bApi) Transfer(typ string, addr string, amount float64, additionalFee float64) error {
	params := &url.Values{}
	amountStr := strconv.FormatFloat(amount, 'f', 4, 64)
	params.Set("address", addr)
	params.Set("coin", typ)
	params.Set("amount", amountStr)
	_, err := h.privateApi("POST", fmt.Sprintf("/v1/account/%s/withdraw/apply", typ), params)
	return err
}

func (h *P2pb2bApi) CancelOrder(trading string, settlement string,
	ordertype models.OrderType, orderNumber string) error {
	params := &url.Values{}
	params.Set("symbol", trading+"-"+settlement)
	params.Set("orderOid", orderNumber)
	if ordertype == models.Ask {
		params.Set("type", "BUY")
	} else {
		params.Set("type", "SELL")
	}
	bs, err := h.privateApi("POST", "/v1/cancel-order", params)
	if err != nil {
		return errors.Wrapf(err, "failed to cancel order")
	}
	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return errors.Wrapf(err, "failed to parse json %s", json)
	}
	success, err := json.GetBoolean("success")
	if err != nil {
		return errors.Wrapf(err, "failed to parse json %s", json)
	}
	if !success {
		errors.Errorf("failed to cancel order %s", json)
	}
	return nil
}

func (h *P2pb2bApi) IsOrderFilled(trading string, settlement string, orderNumber string) (bool, error) {
	params := &url.Values{}
	params.Set("symbol", trading+"-"+settlement)
	bs, err := h.privateApi("GET", "/v1/order/active", params)
	if err != nil {
		return false, errors.Wrapf(err, "failed to cancel order")
	}
	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse json %s", json)
	}
	data, err := json.GetObject("data")
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse json %s", json)
	}
	buys, err := data.GetValueArray("BUY")
	if err != nil {
		return false, errors.Wrap(err, "failed to parse json")
	}
	sells, err := data.GetValueArray("SELL")
	if err != nil {
		return false, errors.Wrap(err, "failed to parse json")
	}
	for _, s := range sells {
		sary, err := s.Array()
		if err != nil {
			return false, errors.Wrap(err, "failed to parse json")
		}
		orderId, err := sary[5].String()
		if err != nil {
			return false, errors.Wrap(err, "failed to parse json")
		}
		if orderId == orderNumber {
			return false, nil
		}
	}
	for _, s := range buys {
		sary, err := s.Array()
		if err != nil {
			return false, errors.Wrap(err, "failed to parse json")
		}
		orderId, err := sary[5].String()
		if err != nil {
			return false, errors.Wrap(err, "failed to parse json")
		}
		if orderId == orderNumber {
			return false, nil
		}
	}
	return true, nil
}

func (h *P2pb2bApi) Address(c string) (string, error) {
	params := &url.Values{}
	bs, err := h.privateApi("GET", fmt.Sprintf("/v1/account/%s/wallet/address", c), params)
	if err != nil {
		return "", errors.Wrapf(err, "failed to cancel order")
	}
	json, err := jason.NewObjectFromBytes(bs)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse json")
	}
	data, err := json.GetObject("data")
	if err != nil {
		return "", errors.Wrap(err, "failed to parse json")
	}
	address, err := data.GetString("address")
	if err != nil {
		return "", errors.Wrap(err, "failed to parse json")
	}
	return address, errors.New("not implemented")
}
