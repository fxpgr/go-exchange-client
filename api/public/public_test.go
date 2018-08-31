package public

import (
	"io/ioutil"
	"math"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type FakeRoundTripper struct {
	message string
	status  int
	header  map[string]string
}

func (rt *FakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	body := strings.NewReader(rt.message)
	res := &http.Response{
		StatusCode: rt.status,
		Body:       ioutil.NopCloser(body),
		Request:    r,
		Header:     make(http.Header),
	}
	res.Header.Set("Content-Type", "application/json")
	return res, nil
}

func TestNewClient(t *testing.T) {
	_, err := NewClient("bitflyer")
	if err != nil {
		panic(err)
	}
	_, err = NewClient("poloniex")
	if err != nil {
		panic(err)
	}
	_, err = NewClient("hitbtc")
	if err != nil {
		panic(err)
	}
}

func newTestPoloniexPublicClient(rt http.RoundTripper) PublicClient {
	endpoint := "http://localhost:4243"
	api := &PoloniexApi{
		BaseURL:           endpoint,
		RateCacheDuration: 30 * time.Second,
		HttpClient:        http.Client{Transport: rt},
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		m:                 new(sync.Mutex),
	}
	return api
}
func newTestHitbtcPublicClient(rt http.RoundTripper) PublicClient {
	endpoint := "http://localhost:4243"
	api := &HitbtcApi{
		BaseURL:           endpoint,
		RateCacheDuration: 30 * time.Second,
		HttpClient:        http.Client{Transport: rt},
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		m:                 new(sync.Mutex),
	}
	api.fetchSettlements()
	return api
}

func newTestLbankPublicClient(rt http.RoundTripper) PublicClient {
	endpoint := "http://localhost:4243"
	api := &LbankApi{
		BaseURL:           endpoint,
		RateCacheDuration: 30 * time.Second,
		HttpClient:        &http.Client{Transport: rt},
		rt:                rt,
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		m:                 new(sync.Mutex),
		rateM:             new(sync.Mutex),
		currencyM:         new(sync.Mutex),
	}
	return api
}

func newTestHuobiPublicClient(rt http.RoundTripper) PublicClient {
	endpoint := "http://localhost:4243"
	n := make(map[string]float64)
	n["BTC"] = 0.1
	m := make(map[string]map[string]float64)
	m["ETH"] = n
	l := make(map[string]float64)
	l["BTC"] = 0.1
	k := make(map[string]map[string]float64)
	k["ETH"] = l
	api := &HuobiApi{
		BaseURL:           endpoint,
		RateCacheDuration: 30 * time.Second,
		HttpClient:        &http.Client{Transport: rt},
		rt:                rt,
		rateMap:           m,
		volumeMap:         k,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		m:                 new(sync.Mutex),
		rateM:             new(sync.Mutex),
		currencyM:         new(sync.Mutex),
	}
	api.fetchSettlements()
	return api
}

func newTestBitflyerPublicClient(rt http.RoundTripper) PublicClient {
	endpoint := "http://localhost:4243"
	api := &BitflyerApi{
		BaseURL:           endpoint,
		RateCacheDuration: 30 * time.Second,
		HttpClient:        http.Client{Transport: rt},
		rateMap:           nil,
		volumeMap:         nil,
		rateLastUpdated:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		m:                 new(sync.Mutex),
	}
	api.fetchSettlements()
	return api
}

func TestBitflyerRate(t *testing.T) {

	jsonTicker := `{
  "product_code": "BTC_JPY",
  "timestamp": "2015-07-08T02:50:59.97",
  "tick_id": 3579,
  "best_bid": 30000,
  "best_ask": 36640,
  "best_bid_size": 0.1,
  "best_ask_size": 5,
  "total_bid_depth": 15.13,
  "total_ask_depth": 20,
  "ltp": 31690,
  "volume": 16819.26,
  "volume_by_product": 6819.26
}`
	client := newTestBitflyerPublicClient(&FakeRoundTripper{message: jsonTicker, status: http.StatusOK})
	rate, err := client.Rate("BTC", "JPY")
	if err != nil {
		panic(err)
	}
	if rate != math.Trunc(31690) {
		t.Errorf("BitflyerPublicApi: Expected %v. Got %v", 31690, rate)
	}
}

func TestBitflyerVolume(t *testing.T) {

	jsonTicker := `{
  "product_code": "BTC_JPY",
  "timestamp": "2015-07-08T02:50:59.97",
  "tick_id": 3579,
  "best_bid": 30000,
  "best_ask": 36640,
  "best_bid_size": 0.1,
  "best_ask_size": 5,
  "total_bid_depth": 15.13,
  "total_ask_depth": 20,
  "ltp": 31690,
  "volume": 16819.26,
  "volume_by_product": 6819.26
}`
	client := newTestBitflyerPublicClient(&FakeRoundTripper{message: jsonTicker, status: http.StatusOK})
	volume, err := client.Volume("BTC", "JPY")
	if err != nil {
		panic(err)
	}
	if volume != 16819.26 {
		t.Errorf("BitflyerPublicApi: Expected %v. Got %v", 16819.26, volume)
	}
}

func TestBitflyerCurrencyPairs(t *testing.T) {

	jsonTicker := `{
  "product_code": "BTC_JPY",
  "timestamp": "2015-07-08T02:50:59.97",
  "tick_id": 3579,
  "best_bid": 30000,
  "best_ask": 36640,
  "best_bid_size": 0.1,
  "best_ask_size": 5,
  "total_bid_depth": 15.13,
  "total_ask_depth": 20,
  "ltp": 31690,
  "volume": 16819.26,
  "volume_by_product": 6819.26
}`
	client := newTestBitflyerPublicClient(&FakeRoundTripper{message: jsonTicker, status: http.StatusOK})
	pairs, err := client.CurrencyPairs()
	if err != nil {
		panic(err)
	}
	for _, _ = range pairs {
	}
}

func TestBitflyerBoard(t *testing.T) {

	jsonBoard := `{"mid_price":33320,"bids":[{"price":30000,"size":0.1},{"price":25570,"size":3}],"asks":[{"price":36640,"size":5},{"price":36700,"size":1.2}]}`
	client := newTestBitflyerPublicClient(&FakeRoundTripper{message: jsonBoard, status: http.StatusOK})
	_, err := client.Board("BTC", "JPY")
	if err != nil {
		panic(err)
	}
}

func TestPoloniexRate(t *testing.T) {

	jsonTicker := `{"BTC_BCN":{"id":7,"last":"0.00000044","lowestAsk":"0.00000044","highestBid":"0.00000043","percentChange":"-0.04347826","baseVolume":"29.09099079","quoteVolume":"64263958.33949675","isFrozen":"0","high24hr":"0.00000048","low24hr":"0.00000042"},"BTC_BELA":{"id":8,"last":"0.00001605","lowestAsk":"0.00001612","highestBid":"0.00001606","percentChange":"-0.08022922","baseVolume":"4.07014224","quoteVolume":"239482.67219866","isFrozen":"0","high24hr":"0.00001767","low24hr":"0.00001601"},"BTC_BLK":{"id":10,"last":"0.00003141","lowestAsk":"0.00003141","highestBid":"0.00003119","percentChange":"-0.03620742","baseVolume":"5.25336081","quoteVolume":"164929.08275402","isFrozen":"0","high24hr":"0.00003285","low24hr":"0.00003101"},"BTC_BTCD":{"id":12,"last":"0.00979795","lowestAsk":"0.00979549","highestBid":"0.00975102","percentChange":"-0.03547155","baseVolume":"1.09034776","quoteVolume":"111.38118807","isFrozen":"0","high24hr":"0.01000535","low24hr":"0.00975034"},"BTC_BTM":{"id":13,"last":"0.00008519","lowestAsk":"0.00008696","highestBid":"0.00008520","percentChange":"0.12033140","baseVolume":"5.75976561","quoteVolume":"69069.35601392","isFrozen":"0","high24hr":"0.00009258","low24hr":"0.00007000"},"BTC_BTS":{"id":14,"last":"0.00002029","lowestAsk":"0.00002028","highestBid":"0.00002022","percentChange":"-0.01120857","baseVolume":"79.53976080","quoteVolume":"3889105.34421891","isFrozen":"0","high24hr":"0.00002110","low24hr":"0.00002000"},"BTC_BURST":{"id":15,"last":"0.00000360","lowestAsk":"0.00000362","highestBid":"0.00000360","percentChange":"0.18811881","baseVolume":"78.38171781","quoteVolume":"22280856.88496521","isFrozen":"0","high24hr":"0.00000389","low24hr":"0.00000302"},"BTC_CLAM":{"id":20,"last":"0.00053002","lowestAsk":"0.00053498","highestBid":"0.00053002","percentChange":"-0.01229920","baseVolume":"3.67717167","quoteVolume":"6823.55539077","isFrozen":"0","high24hr":"0.00055182","low24hr":"0.00052990"},"BTC_DASH":{"id":24,"last":"0.05637575","lowestAsk":"0.05664179","highestBid":"0.05637631","percentChange":"-0.01845420","baseVolume":"209.06256707","quoteVolume":"3699.61400244","isFrozen":"0","high24hr":"0.05859546","low24hr":"0.05498114"},"BTC_DGB":{"id":25,"last":"0.00000329","lowestAsk":"0.00000329","highestBid":"0.00000327","percentChange":"-0.04081632","baseVolume":"57.95039019","quoteVolume":"17129885.46202723","isFrozen":"0","high24hr":"0.00000347","low24hr":"0.00000324"},"BTC_DOGE":{"id":27,"last":"0.00000059","lowestAsk":"0.00000059","highestBid":"0.00000058","percentChange":"-0.01666666","baseVolume":"192.26309111","quoteVolume":"330655712.41955251","isFrozen":"0","high24hr":"0.00000061","low24hr":"0.00000055"},"BTC_EMC2":{"id":28,"last":"0.00002755","lowestAsk":"0.00002782","highestBid":"0.00002755","percentChange":"-0.03536414","baseVolume":"9.66700911","quoteVolume":"342687.43192313","isFrozen":"0","high24hr":"0.00002971","low24hr":"0.00002735"},"BTC_FLDC":{"id":31,"last":"0.00000246","lowestAsk":"0.00000247","highestBid":"0.00000246","percentChange":"0.00819672","baseVolume":"1.16473043","quoteVolume":"473001.75777511","isFrozen":"0","high24hr":"0.00000254","low24hr":"0.00000242"},"BTC_FLO":{"id":32,"last":"0.00000957","lowestAsk":"0.00000968","highestBid":"0.00000958","percentChange":"0.00525210","baseVolume":"2.08213321","quoteVolume":"214758.93398200","isFrozen":"0","high24hr":"0.00000991","low24hr":"0.00000926"},"BTC_GAME":{"id":38,"last":"0.00019178","lowestAsk":"0.00019178","highestBid":"0.00019158","percentChange":"-0.03004248","baseVolume":"27.09161532","quoteVolume":"132787.64539663","isFrozen":"0","high24hr":"0.00021536","low24hr":"0.00019005"},"BTC_GRC":{"id":40,"last":"0.00000610","lowestAsk":"0.00000620","highestBid":"0.00000610","percentChange":"-0.06441717","baseVolume":"3.04065612","quoteVolume":"489424.75359611","isFrozen":"0","high24hr":"0.00000660","low24hr":"0.00000607"},"BTC_HUC":{"id":43,"last":"0.00002365","lowestAsk":"0.00002365","highestBid":"0.00002354","percentChange":"-0.04289761","baseVolume":"0.72457847","quoteVolume":"30161.52149021","isFrozen":"0","high24hr":"0.00002474","low24hr":"0.00002316"},"BTC_LTC":{"id":50,"last":"0.01978000","lowestAsk":"0.01977999","highestBid":"0.01977410","percentChange":"-0.03653331","baseVolume":"1167.53090263","quoteVolume":"57502.88609392","isFrozen":"0","high24hr":"0.02077132","low24hr":"0.01970000"},"BTC_MAID":{"id":51,"last":"0.00003518","lowestAsk":"0.00003518","highestBid":"0.00003498","percentChange":"0.03837072","baseVolume":"132.02767750","quoteVolume":"3651418.39478196","isFrozen":"0","high24hr":"0.00003934","low24hr":"0.00003257"},"BTC_OMNI":{"id":58,"last":"0.00369998","lowestAsk":"0.00369998","highestBid":"0.00364190","percentChange":"0.02343116","baseVolume":"1.79218231","quoteVolume":"489.38759050","isFrozen":"0","high24hr":"0.00373942","low24hr":"0.00361527"},"BTC_NAV":{"id":61,"last":"0.00017341","lowestAsk":"0.00017337","highestBid":"0.00017293","percentChange":"-0.00970818","baseVolume":"7.70942666","quoteVolume":"44717.15737983","isFrozen":"0","high24hr":"0.00017976","low24hr":"0.00016813"},"BTC_NEOS":{"id":63,"last":"0.00039001","lowestAsk":"0.00039037","highestBid":"0.00039000","percentChange":"-0.05828805","baseVolume":"3.28758311","quoteVolume":"8261.17380835","isFrozen":"0","high24hr":"0.00041902","low24hr":"0.00039000"},"BTC_NMC":{"id":64,"last":"0.00024009","lowestAsk":"0.00024125","highestBid":"0.00024009","percentChange":"-0.04017750","baseVolume":"0.52687827","quoteVolume":"2141.10580645","isFrozen":"0","high24hr":"0.00025264","low24hr":"0.00024009"},"BTC_NXT":{"id":69,"last":"0.00001911","lowestAsk":"0.00001912","highestBid":"0.00001911","percentChange":"-0.04735792","baseVolume":"32.54118716","quoteVolume":"1666537.75384808","isFrozen":"0","high24hr":"0.00002021","low24hr":"0.00001893"},"BTC_PINK":{"id":73,"last":"0.00000281","lowestAsk":"0.00000283","highestBid":"0.00000280","percentChange":"-0.02090592","baseVolume":"1.20652265","quoteVolume":"427629.44370182","isFrozen":"0","high24hr":"0.00000292","low24hr":"0.00000278"},"BTC_POT":{"id":74,"last":"0.00001522","lowestAsk":"0.00001528","highestBid":"0.00001522","percentChange":"-0.02933673","baseVolume":"3.39324883","quoteVolume":"218770.38580925","isFrozen":"0","high24hr":"0.00001606","low24hr":"0.00001510"},"BTC_PPC":{"id":75,"last":"0.00029162","lowestAsk":"0.00029162","highestBid":"0.00028701","percentChange":"-0.06150033","baseVolume":"12.46475422","quoteVolume":"41523.06111629","isFrozen":"0","high24hr":"0.00031600","low24hr":"0.00027900"},"BTC_RIC":{"id":83,"last":"0.00002639","lowestAsk":"0.00002683","highestBid":"0.00002651","percentChange":"-0.08748271","baseVolume":"53.36435932","quoteVolume":"1894264.91718601","isFrozen":"0","high24hr":"0.00003439","low24hr":"0.00002486"},"BTC_STR":{"id":89,"last":"0.00003220","lowestAsk":"0.00003220","highestBid":"0.00003215","percentChange":"-0.04394299","baseVolume":"528.89081221","quoteVolume":"16175607.89367209","isFrozen":"0","high24hr":"0.00003411","low24hr":"0.00003143"},"BTC_SYS":{"id":92,"last":"0.00006099","lowestAsk":"0.00006090","highestBid":"0.00006034","percentChange":"-0.01549636","baseVolume":"28.60142171","quoteVolume":"466305.29034872","isFrozen":"0","high24hr":"0.00006363","low24hr":"0.00006015"},"BTC_VIA":{"id":97,"last":"0.00024391","lowestAsk":"0.00024373","highestBid":"0.00024119","percentChange":"-0.02044176","baseVolume":"8.41407416","quoteVolume":"33693.34977288","isFrozen":"0","high24hr":"0.00025572","low24hr":"0.00024062"},"BTC_XVC":{"id":98,"last":"0.00004138","lowestAsk":"0.00004191","highestBid":"0.00004138","percentChange":"-0.00409145","baseVolume":"0.62065903","quoteVolume":"14793.19876026","isFrozen":"0","high24hr":"0.00004397","low24hr":"0.00004101"},"BTC_VRC":{"id":99,"last":"0.00008190","lowestAsk":"0.00008200","highestBid":"0.00008139","percentChange":"-0.00967351","baseVolume":"22.68904050","quoteVolume":"269368.45138333","isFrozen":"0","high24hr":"0.00008855","low24hr":"0.00008084"},"BTC_VTC":{"id":100,"last":"0.00036194","lowestAsk":"0.00036194","highestBid":"0.00036193","percentChange":"-0.07950152","baseVolume":"19.39531405","quoteVolume":"51120.56163987","isFrozen":"0","high24hr":"0.00039357","low24hr":"0.00036160"},"BTC_XBC":{"id":104,"last":"0.00705084","lowestAsk":"0.00705084","highestBid":"0.00697113","percentChange":"-0.00251816","baseVolume":"0.69091585","quoteVolume":"97.55912960","isFrozen":"0","high24hr":"0.00717000","low24hr":"0.00697112"},"BTC_XCP":{"id":108,"last":"0.00207553","lowestAsk":"0.00207553","highestBid":"0.00206080","percentChange":"0.00343255","baseVolume":"8.65748955","quoteVolume":"4218.24473378","isFrozen":"0","high24hr":"0.00213802","low24hr":"0.00200000"},"BTC_XEM":{"id":112,"last":"0.00003850","lowestAsk":"0.00003850","highestBid":"0.00003846","percentChange":"0.05335157","baseVolume":"186.76423244","quoteVolume":"4880505.48589732","isFrozen":"0","high24hr":"0.00003999","low24hr":"0.00003630"},"BTC_XMR":{"id":114,"last":"0.02755976","lowestAsk":"0.02755980","highestBid":"0.02755976","percentChange":"-0.01360704","baseVolume":"383.72144157","quoteVolume":"13765.56273024","isFrozen":"0","high24hr":"0.02840041","low24hr":"0.02750000"},"BTC_XPM":{"id":116,"last":"0.00008172","lowestAsk":"0.00008268","highestBid":"0.00008173","percentChange":"-0.14312676","baseVolume":"22.77991718","quoteVolume":"246492.03590984","isFrozen":"0","high24hr":"0.00010400","low24hr":"0.00008028"},"BTC_XRP":{"id":117,"last":"0.00008535","lowestAsk":"0.00008545","highestBid":"0.00008535","percentChange":"-0.02009184","baseVolume":"1329.81359724","quoteVolume":"15483518.38295366","isFrozen":"0","high24hr":"0.00008843","low24hr":"0.00008011"},"USDT_BTC":{"id":121,"last":"10624.99998773","lowestAsk":"10624.99998664","highestBid":"10608.00000003","percentChange":"-0.00692886","baseVolume":"35691429.96539170","quoteVolume":"3332.58429269","isFrozen":"0","high24hr":"11074.00000000","low24hr":"10469.32778879"},"USDT_DASH":{"id":122,"last":"600.00000000","lowestAsk":"599.99999991","highestBid":"596.93035101","percentChange":"-0.03219404","baseVolume":"1283299.41066996","quoteVolume":"2098.92266394","isFrozen":"0","high24hr":"622.57893075","low24hr":"591.95179691"},"USDT_LTC":{"id":123,"last":"210.95749000","lowestAsk":"210.94748953","highestBid":"209.88560829","percentChange":"-0.03787506","baseVolume":"4594398.92543038","quoteVolume":"21232.61767653","isFrozen":"0","high24hr":"223.54000007","low24hr":"208.20000000"},"USDT_NXT":{"id":124,"last":"0.20223804","lowestAsk":"0.20325222","highestBid":"0.20223804","percentChange":"-0.05806965","baseVolume":"603478.19811368","quoteVolume":"2885725.55969930","isFrozen":"0","high24hr":"0.21881127","low24hr":"0.19921189"},"USDT_STR":{"id":125,"last":"0.34222321","lowestAsk":"0.34222323","highestBid":"0.34222321","percentChange":"-0.05410942","baseVolume":"2295735.51730013","quoteVolume":"6456715.13838263","isFrozen":"0","high24hr":"0.36500000","low24hr":"0.33551234"},"USDT_XMR":{"id":126,"last":"292.67178868","lowestAsk":"292.67178807","highestBid":"291.00000421","percentChange":"-0.01620845","baseVolume":"1123622.33137267","quoteVolume":"3760.05453072","isFrozen":"0","high24hr":"304.77803231","low24hr":"290.00000002"},"USDT_XRP":{"id":127,"last":"0.90978146","lowestAsk":"0.90978000","highestBid":"0.90938146","percentChange":"-0.02979624","baseVolume":"3653275.18642841","quoteVolume":"3938239.54477194","isFrozen":"0","high24hr":"0.95303906","low24hr":"0.89500000"},"XMR_BCN":{"id":129,"last":"0.00001583","lowestAsk":"0.00001636","highestBid":"0.00001607","percentChange":"-0.04118715","baseVolume":"8.05137794","quoteVolume":"486388.87722166","isFrozen":"0","high24hr":"0.00001722","low24hr":"0.00001560"},"XMR_BLK":{"id":130,"last":"0.00114510","lowestAsk":"0.00114901","highestBid":"0.00113176","percentChange":"-0.01076402","baseVolume":"1.72978309","quoteVolume":"1516.34903291","isFrozen":"0","high24hr":"0.00117441","low24hr":"0.00111542"},"XMR_BTCD":{"id":131,"last":"0.35215967","lowestAsk":"0.35215967","highestBid":"0.34902601","percentChange":"-0.00019359","baseVolume":"2.42824965","quoteVolume":"6.96217009","isFrozen":"0","high24hr":"0.35950689","low24hr":"0.34687854"},"XMR_DASH":{"id":132,"last":"2.04288022","lowestAsk":"2.04599999","highestBid":"2.03300002","percentChange":"-0.02441250","baseVolume":"12.77948163","quoteVolume":"6.29569469","isFrozen":"0","high24hr":"2.07725856","low24hr":"2.00970004"},"XMR_LTC":{"id":137,"last":"0.71559210","lowestAsk":"0.72490000","highestBid":"0.71559210","percentChange":"-0.00999425","baseVolume":"42.29974914","quoteVolume":"58.22486474","isFrozen":"0","high24hr":"0.74179000","low24hr":"0.71510000"},"XMR_MAID":{"id":138,"last":"0.00127178","lowestAsk":"0.00129599","highestBid":"0.00126490","percentChange":"0.05855522","baseVolume":"13.56874950","quoteVolume":"10541.32711178","isFrozen":"0","high24hr":"0.00137807","low24hr":"0.00115701"},"XMR_NXT":{"id":140,"last":"0.00069503","lowestAsk":"0.00069464","highestBid":"0.00068597","percentChange":"-0.02257129","baseVolume":"2.89409514","quoteVolume":"4144.04804373","isFrozen":"0","high24hr":"0.00072300","low24hr":"0.00068330"},"BTC_ETH":{"id":148,"last":"0.08184499","lowestAsk":"0.08184000","highestBid":"0.08179850","percentChange":"-0.00580760","baseVolume":"1533.30420352","quoteVolume":"18714.83519723","isFrozen":"0","high24hr":"0.08284246","low24hr":"0.07985001"},"USDT_ETH":{"id":149,"last":"869.39534497","lowestAsk":"869.52999973","highestBid":"867.61588914","percentChange":"-0.01455483","baseVolume":"3855929.24959329","quoteVolume":"4396.20321972","isFrozen":"0","high24hr":"890.01000000","low24hr":"858.87562001"},"BTC_SC":{"id":150,"last":"0.00000187","lowestAsk":"0.00000187","highestBid":"0.00000186","percentChange":"-0.02604166","baseVolume":"94.57654683","quoteVolume":"49360230.33320522","isFrozen":"0","high24hr":"0.00000199","low24hr":"0.00000186"},"BTC_BCY":{"id":151,"last":"0.00004390","lowestAsk":"0.00004442","highestBid":"0.00004370","percentChange":"-0.01701746","baseVolume":"1.80036972","quoteVolume":"41088.53490459","isFrozen":"0","high24hr":"0.00004500","low24hr":"0.00004278"},"BTC_EXP":{"id":153,"last":"0.00025868","lowestAsk":"0.00025868","highestBid":"0.00025822","percentChange":"-0.00675779","baseVolume":"4.84435099","quoteVolume":"18298.59663925","isFrozen":"0","high24hr":"0.00028105","low24hr":"0.00025512"},"BTC_FCT":{"id":155,"last":"0.00310006","lowestAsk":"0.00312607","highestBid":"0.00310006","percentChange":"0.03817044","baseVolume":"93.68957267","quoteVolume":"29302.05113453","isFrozen":"0","high24hr":"0.00342000","low24hr":"0.00295864"},"BTC_RADS":{"id":158,"last":"0.00055595","lowestAsk":"0.00055603","highestBid":"0.00055595","percentChange":"0.00171171","baseVolume":"2.40792647","quoteVolume":"4384.29903283","isFrozen":"0","high24hr":"0.00056004","low24hr":"0.00054184"},"BTC_AMP":{"id":160,"last":"0.00003266","lowestAsk":"0.00003266","highestBid":"0.00003237","percentChange":"0.12233676","baseVolume":"22.15341699","quoteVolume":"700411.61775904","isFrozen":"0","high24hr":"0.00003470","low24hr":"0.00002900"},"BTC_DCR":{"id":162,"last":"0.00700000","lowestAsk":"0.00700000","highestBid":"0.00699626","percentChange":"-0.02845246","baseVolume":"71.68165727","quoteVolume":"10026.64310783","isFrozen":"0","high24hr":"0.00740000","low24hr":"0.00699625"},"BTC_LSK":{"id":163,"last":"0.00177549","lowestAsk":"0.00177549","highestBid":"0.00177087","percentChange":"-0.05370790","baseVolume":"76.77220271","quoteVolume":"41640.05985932","isFrozen":"0","high24hr":"0.00191517","low24hr":"0.00176237"},"ETH_LSK":{"id":166,"last":"0.02170507","lowestAsk":"0.02196807","highestBid":"0.02170507","percentChange":"-0.05219193","baseVolume":"153.34380628","quoteVolume":"6817.51468673","isFrozen":"0","high24hr":"0.02338368","low24hr":"0.02170507"},"BTC_LBC":{"id":167,"last":"0.00003495","lowestAsk":"0.00003495","highestBid":"0.00003485","percentChange":"-0.02265100","baseVolume":"16.53299322","quoteVolume":"471422.21906920","isFrozen":"0","high24hr":"0.00003776","low24hr":"0.00003366"},"BTC_STEEM":{"id":168,"last":"0.00029382","lowestAsk":"0.00029527","highestBid":"0.00028997","percentChange":"-0.07405773","baseVolume":"18.15624885","quoteVolume":"59642.97013872","isFrozen":"0","high24hr":"0.00031830","low24hr":"0.00028992"},"ETH_STEEM":{"id":169,"last":"0.00358331","lowestAsk":"0.00361200","highestBid":"0.00358346","percentChange":"-0.05709812","baseVolume":"14.63196451","quoteVolume":"3981.26836671","isFrozen":"0","high24hr":"0.00383840","low24hr":"0.00357319"},"BTC_SBD":{"id":170,"last":"0.00032115","lowestAsk":"0.00032300","highestBid":"0.00032254","percentChange":"-0.04906431","baseVolume":"0.79186808","quoteVolume":"2398.48810574","isFrozen":"0","high24hr":"0.00034255","low24hr":"0.00032003"},"BTC_ETC":{"id":171,"last":"0.00318050","lowestAsk":"0.00318003","highestBid":"0.00318000","percentChange":"-0.05426702","baseVolume":"548.96356016","quoteVolume":"167054.79595649","isFrozen":"0","high24hr":"0.00341741","low24hr":"0.00315000"},"ETH_ETC":{"id":172,"last":"0.03909435","lowestAsk":"0.03909417","highestBid":"0.03881661","percentChange":"-0.04181641","baseVolume":"580.95719579","quoteVolume":"14458.59615482","isFrozen":"0","high24hr":"0.04152581","low24hr":"0.03881692"},"USDT_ETC":{"id":173,"last":"33.87000000","lowestAsk":"33.88819156","highestBid":"33.85265630","percentChange":"-0.05628309","baseVolume":"4799570.40286407","quoteVolume":"136788.35369587","isFrozen":"0","high24hr":"36.39788875","low24hr":"33.09999997"},"BTC_REP":{"id":174,"last":"0.00440716","lowestAsk":"0.00442112","highestBid":"0.00441317","percentChange":"-0.03114427","baseVolume":"29.79618928","quoteVolume":"6464.80143022","isFrozen":"0","high24hr":"0.00488758","low24hr":"0.00440168"},"USDT_REP":{"id":175,"last":"46.63290958","lowestAsk":"46.63290958","highestBid":"46.63290910","percentChange":"-0.03918645","baseVolume":"254489.36280387","quoteVolume":"5145.92607861","isFrozen":"0","high24hr":"52.12894591","low24hr":"46.63290958"},"ETH_REP":{"id":176,"last":"0.05408318","lowestAsk":"0.05425802","highestBid":"0.05408318","percentChange":"-0.01386902","baseVolume":"94.24453787","quoteVolume":"1652.51345408","isFrozen":"0","high24hr":"0.05923229","low24hr":"0.05408318"},"BTC_ARDR":{"id":177,"last":"0.00003701","lowestAsk":"0.00003721","highestBid":"0.00003702","percentChange":"-0.06185044","baseVolume":"12.30772940","quoteVolume":"318929.56988432","isFrozen":"0","high24hr":"0.00004087","low24hr":"0.00003701"},"BTC_ZEC":{"id":178,"last":"0.03716137","lowestAsk":"0.03722000","highestBid":"0.03716137","percentChange":"-0.03977301","baseVolume":"141.72242798","quoteVolume":"3726.05946816","isFrozen":"0","high24hr":"0.03890397","low24hr":"0.03711373"},"ETH_ZEC":{"id":179,"last":"0.45847077","lowestAsk":"0.45860212","highestBid":"0.45593483","percentChange":"-0.02721483","baseVolume":"28.88945904","quoteVolume":"62.20448974","isFrozen":"0","high24hr":"0.47321875","low24hr":"0.45469474"},"USDT_ZEC":{"id":180,"last":"394.35039285","lowestAsk":"397.12551178","highestBid":"395.00000000","percentChange":"-0.04279766","baseVolume":"506512.38061647","quoteVolume":"1240.40556073","isFrozen":"0","high24hr":"418.68484294","low24hr":"392.70000000"},"XMR_ZEC":{"id":181,"last":"1.35941597","lowestAsk":"1.36317677","highestBid":"1.34500001","percentChange":"-0.01906218","baseVolume":"20.96692015","quoteVolume":"15.32647405","isFrozen":"0","high24hr":"1.40000000","low24hr":"1.33318373"},"BTC_STRAT":{"id":182,"last":"0.00071293","lowestAsk":"0.00071946","highestBid":"0.00071293","percentChange":"-0.01243922","baseVolume":"40.42123101","quoteVolume":"55776.25886482","isFrozen":"0","high24hr":"0.00074500","low24hr":"0.00070500"},"BTC_NXC":{"id":183,"last":"0.00002000","lowestAsk":"0.00002000","highestBid":"0.00001990","percentChange":"0.00553041","baseVolume":"0.69218465","quoteVolume":"34624.52960262","isFrozen":"0","high24hr":"0.00002096","low24hr":"0.00001969"},"BTC_PASC":{"id":184,"last":"0.00013496","lowestAsk":"0.00013530","highestBid":"0.00013496","percentChange":"-0.13214584","baseVolume":"15.50617150","quoteVolume":"107817.40106833","isFrozen":"0","high24hr":"0.00015700","low24hr":"0.00013059"},"BTC_GNT":{"id":185,"last":"0.00003309","lowestAsk":"0.00003322","highestBid":"0.00003309","percentChange":"-0.05618938","baseVolume":"16.62020155","quoteVolume":"487963.62072953","isFrozen":"0","high24hr":"0.00003549","low24hr":"0.00003309"},"ETH_GNT":{"id":186,"last":"0.00040941","lowestAsk":"0.00040876","highestBid":"0.00040430","percentChange":"-0.04372503","baseVolume":"10.09480288","quoteVolume":"24137.14149454","isFrozen":"0","high24hr":"0.00042840","low24hr":"0.00040478"},"BTC_GNO":{"id":187,"last":"0.01225507","lowestAsk":"0.01244914","highestBid":"0.01225555","percentChange":"-0.03210425","baseVolume":"1.58931513","quoteVolume":"128.22232126","isFrozen":"0","high24hr":"0.01266156","low24hr":"0.01220000"},"ETH_GNO":{"id":188,"last":"0.15050303","lowestAsk":"0.15229149","highestBid":"0.15071966","percentChange":"-0.01625557","baseVolume":"8.67687758","quoteVolume":"57.38001905","isFrozen":"0","high24hr":"0.15525901","low24hr":"0.15050000"},"BTC_BCH":{"id":189,"last":"0.11518232","lowestAsk":"0.11534375","highestBid":"0.11528132","percentChange":"-0.03500843","baseVolume":"275.98819978","quoteVolume":"2371.47230887","isFrozen":"0","high24hr":"0.11964799","low24hr":"0.11407690"},"ETH_BCH":{"id":190,"last":"1.40300000","lowestAsk":"1.41500000","highestBid":"1.40212240","percentChange":"-0.03308063","baseVolume":"161.67026842","quoteVolume":"113.37256684","isFrozen":"0","high24hr":"1.45321418","low24hr":"1.39957466"},"USDT_BCH":{"id":191,"last":"1224.19537308","lowestAsk":"1224.26904572","highestBid":"1220.51173609","percentChange":"-0.03788480","baseVolume":"1739159.36883248","quoteVolume":"1395.20317541","isFrozen":"0","high24hr":"1290.07329597","low24hr":"1200.00000000"},"BTC_ZRX":{"id":192,"last":"0.00009060","lowestAsk":"0.00009092","highestBid":"0.00009060","percentChange":"-0.07664084","baseVolume":"84.24667407","quoteVolume":"866347.75724553","isFrozen":"0","high24hr":"0.00010789","low24hr":"0.00009000"},"ETH_ZRX":{"id":193,"last":"0.00110985","lowestAsk":"0.00111195","highestBid":"0.00110400","percentChange":"-0.06894121","baseVolume":"97.55442267","quoteVolume":"81734.47819416","isFrozen":"0","high24hr":"0.00131243","low24hr":"0.00110100"},"BTC_CVC":{"id":194,"last":"0.00003385","lowestAsk":"0.00003386","highestBid":"0.00003375","percentChange":"-0.01052323","baseVolume":"10.95368997","quoteVolume":"317216.20691091","isFrozen":"0","high24hr":"0.00003600","low24hr":"0.00003356"},"ETH_CVC":{"id":195,"last":"0.00041877","lowestAsk":"0.00041995","highestBid":"0.00041457","percentChange":"0.00901139","baseVolume":"16.45389610","quoteVolume":"38944.62157283","isFrozen":"0","high24hr":"0.00042999","low24hr":"0.00040678"},"BTC_OMG":{"id":196,"last":"0.00188805","lowestAsk":"0.00189927","highestBid":"0.00188806","percentChange":"0.07584874","baseVolume":"294.90308428","quoteVolume":"157895.60549944","isFrozen":"0","high24hr":"0.00194765","low24hr":"0.00174590"},"ETH_OMG":{"id":197,"last":"0.02324351","lowestAsk":"0.02324380","highestBid":"0.02299925","percentChange":"0.10140545","baseVolume":"309.24005022","quoteVolume":"13519.35720959","isFrozen":"0","high24hr":"0.02370911","low24hr":"0.02128299"},"BTC_GAS":{"id":198,"last":"0.00374597","lowestAsk":"0.00374597","highestBid":"0.00373248","percentChange":"-0.08963718","baseVolume":"19.77538762","quoteVolume":"5062.54665375","isFrozen":"0","high24hr":"0.00413748","low24hr":"0.00371000"},"ETH_GAS":{"id":199,"last":"0.04561716","lowestAsk":"0.04561716","highestBid":"0.04561374","percentChange":"-0.08874920","baseVolume":"29.59015810","quoteVolume":"617.68366442","isFrozen":"0","high24hr":"0.05040887","low24hr":"0.04561624"},"BTC_STORJ":{"id":200,"last":"0.00008461","lowestAsk":"0.00008500","highestBid":"0.00008462","percentChange":"-0.03567358","baseVolume":"5.29465126","quoteVolume":"60905.94469730","isFrozen":"0","high24hr":"0.00008960","low24hr":"0.00008460"}}`
	client := newTestPoloniexPublicClient(&FakeRoundTripper{message: jsonTicker, status: http.StatusOK})
	rate, err := client.Rate("BCN", "BTC")
	if err != nil {
		panic(err)
	}
	if rate != 0.00000044 {
		t.Errorf("PoloniexPublicApi: Expected %v. Got %v", 0.00000044, rate)
	}
}

func TestPoloniexVolume(t *testing.T) {

	jsonTicker := `{"BTC_BCN":{"id":7,"last":"0.00000044","lowestAsk":"0.00000044","highestBid":"0.00000043","percentChange":"-0.04347826","baseVolume":"29.09099079","quoteVolume":"64263958.33949675","isFrozen":"0","high24hr":"0.00000048","low24hr":"0.00000042"},"BTC_BELA":{"id":8,"last":"0.00001605","lowestAsk":"0.00001612","highestBid":"0.00001606","percentChange":"-0.08022922","baseVolume":"4.07014224","quoteVolume":"239482.67219866","isFrozen":"0","high24hr":"0.00001767","low24hr":"0.00001601"},"BTC_BLK":{"id":10,"last":"0.00003141","lowestAsk":"0.00003141","highestBid":"0.00003119","percentChange":"-0.03620742","baseVolume":"5.25336081","quoteVolume":"164929.08275402","isFrozen":"0","high24hr":"0.00003285","low24hr":"0.00003101"},"BTC_BTCD":{"id":12,"last":"0.00979795","lowestAsk":"0.00979549","highestBid":"0.00975102","percentChange":"-0.03547155","baseVolume":"1.09034776","quoteVolume":"111.38118807","isFrozen":"0","high24hr":"0.01000535","low24hr":"0.00975034"},"BTC_BTM":{"id":13,"last":"0.00008519","lowestAsk":"0.00008696","highestBid":"0.00008520","percentChange":"0.12033140","baseVolume":"5.75976561","quoteVolume":"69069.35601392","isFrozen":"0","high24hr":"0.00009258","low24hr":"0.00007000"},"BTC_BTS":{"id":14,"last":"0.00002029","lowestAsk":"0.00002028","highestBid":"0.00002022","percentChange":"-0.01120857","baseVolume":"79.53976080","quoteVolume":"3889105.34421891","isFrozen":"0","high24hr":"0.00002110","low24hr":"0.00002000"},"BTC_BURST":{"id":15,"last":"0.00000360","lowestAsk":"0.00000362","highestBid":"0.00000360","percentChange":"0.18811881","baseVolume":"78.38171781","quoteVolume":"22280856.88496521","isFrozen":"0","high24hr":"0.00000389","low24hr":"0.00000302"},"BTC_CLAM":{"id":20,"last":"0.00053002","lowestAsk":"0.00053498","highestBid":"0.00053002","percentChange":"-0.01229920","baseVolume":"3.67717167","quoteVolume":"6823.55539077","isFrozen":"0","high24hr":"0.00055182","low24hr":"0.00052990"},"BTC_DASH":{"id":24,"last":"0.05637575","lowestAsk":"0.05664179","highestBid":"0.05637631","percentChange":"-0.01845420","baseVolume":"209.06256707","quoteVolume":"3699.61400244","isFrozen":"0","high24hr":"0.05859546","low24hr":"0.05498114"},"BTC_DGB":{"id":25,"last":"0.00000329","lowestAsk":"0.00000329","highestBid":"0.00000327","percentChange":"-0.04081632","baseVolume":"57.95039019","quoteVolume":"17129885.46202723","isFrozen":"0","high24hr":"0.00000347","low24hr":"0.00000324"},"BTC_DOGE":{"id":27,"last":"0.00000059","lowestAsk":"0.00000059","highestBid":"0.00000058","percentChange":"-0.01666666","baseVolume":"192.26309111","quoteVolume":"330655712.41955251","isFrozen":"0","high24hr":"0.00000061","low24hr":"0.00000055"},"BTC_EMC2":{"id":28,"last":"0.00002755","lowestAsk":"0.00002782","highestBid":"0.00002755","percentChange":"-0.03536414","baseVolume":"9.66700911","quoteVolume":"342687.43192313","isFrozen":"0","high24hr":"0.00002971","low24hr":"0.00002735"},"BTC_FLDC":{"id":31,"last":"0.00000246","lowestAsk":"0.00000247","highestBid":"0.00000246","percentChange":"0.00819672","baseVolume":"1.16473043","quoteVolume":"473001.75777511","isFrozen":"0","high24hr":"0.00000254","low24hr":"0.00000242"},"BTC_FLO":{"id":32,"last":"0.00000957","lowestAsk":"0.00000968","highestBid":"0.00000958","percentChange":"0.00525210","baseVolume":"2.08213321","quoteVolume":"214758.93398200","isFrozen":"0","high24hr":"0.00000991","low24hr":"0.00000926"},"BTC_GAME":{"id":38,"last":"0.00019178","lowestAsk":"0.00019178","highestBid":"0.00019158","percentChange":"-0.03004248","baseVolume":"27.09161532","quoteVolume":"132787.64539663","isFrozen":"0","high24hr":"0.00021536","low24hr":"0.00019005"},"BTC_GRC":{"id":40,"last":"0.00000610","lowestAsk":"0.00000620","highestBid":"0.00000610","percentChange":"-0.06441717","baseVolume":"3.04065612","quoteVolume":"489424.75359611","isFrozen":"0","high24hr":"0.00000660","low24hr":"0.00000607"},"BTC_HUC":{"id":43,"last":"0.00002365","lowestAsk":"0.00002365","highestBid":"0.00002354","percentChange":"-0.04289761","baseVolume":"0.72457847","quoteVolume":"30161.52149021","isFrozen":"0","high24hr":"0.00002474","low24hr":"0.00002316"},"BTC_LTC":{"id":50,"last":"0.01978000","lowestAsk":"0.01977999","highestBid":"0.01977410","percentChange":"-0.03653331","baseVolume":"1167.53090263","quoteVolume":"57502.88609392","isFrozen":"0","high24hr":"0.02077132","low24hr":"0.01970000"},"BTC_MAID":{"id":51,"last":"0.00003518","lowestAsk":"0.00003518","highestBid":"0.00003498","percentChange":"0.03837072","baseVolume":"132.02767750","quoteVolume":"3651418.39478196","isFrozen":"0","high24hr":"0.00003934","low24hr":"0.00003257"},"BTC_OMNI":{"id":58,"last":"0.00369998","lowestAsk":"0.00369998","highestBid":"0.00364190","percentChange":"0.02343116","baseVolume":"1.79218231","quoteVolume":"489.38759050","isFrozen":"0","high24hr":"0.00373942","low24hr":"0.00361527"},"BTC_NAV":{"id":61,"last":"0.00017341","lowestAsk":"0.00017337","highestBid":"0.00017293","percentChange":"-0.00970818","baseVolume":"7.70942666","quoteVolume":"44717.15737983","isFrozen":"0","high24hr":"0.00017976","low24hr":"0.00016813"},"BTC_NEOS":{"id":63,"last":"0.00039001","lowestAsk":"0.00039037","highestBid":"0.00039000","percentChange":"-0.05828805","baseVolume":"3.28758311","quoteVolume":"8261.17380835","isFrozen":"0","high24hr":"0.00041902","low24hr":"0.00039000"},"BTC_NMC":{"id":64,"last":"0.00024009","lowestAsk":"0.00024125","highestBid":"0.00024009","percentChange":"-0.04017750","baseVolume":"0.52687827","quoteVolume":"2141.10580645","isFrozen":"0","high24hr":"0.00025264","low24hr":"0.00024009"},"BTC_NXT":{"id":69,"last":"0.00001911","lowestAsk":"0.00001912","highestBid":"0.00001911","percentChange":"-0.04735792","baseVolume":"32.54118716","quoteVolume":"1666537.75384808","isFrozen":"0","high24hr":"0.00002021","low24hr":"0.00001893"},"BTC_PINK":{"id":73,"last":"0.00000281","lowestAsk":"0.00000283","highestBid":"0.00000280","percentChange":"-0.02090592","baseVolume":"1.20652265","quoteVolume":"427629.44370182","isFrozen":"0","high24hr":"0.00000292","low24hr":"0.00000278"},"BTC_POT":{"id":74,"last":"0.00001522","lowestAsk":"0.00001528","highestBid":"0.00001522","percentChange":"-0.02933673","baseVolume":"3.39324883","quoteVolume":"218770.38580925","isFrozen":"0","high24hr":"0.00001606","low24hr":"0.00001510"},"BTC_PPC":{"id":75,"last":"0.00029162","lowestAsk":"0.00029162","highestBid":"0.00028701","percentChange":"-0.06150033","baseVolume":"12.46475422","quoteVolume":"41523.06111629","isFrozen":"0","high24hr":"0.00031600","low24hr":"0.00027900"},"BTC_RIC":{"id":83,"last":"0.00002639","lowestAsk":"0.00002683","highestBid":"0.00002651","percentChange":"-0.08748271","baseVolume":"53.36435932","quoteVolume":"1894264.91718601","isFrozen":"0","high24hr":"0.00003439","low24hr":"0.00002486"},"BTC_STR":{"id":89,"last":"0.00003220","lowestAsk":"0.00003220","highestBid":"0.00003215","percentChange":"-0.04394299","baseVolume":"528.89081221","quoteVolume":"16175607.89367209","isFrozen":"0","high24hr":"0.00003411","low24hr":"0.00003143"},"BTC_SYS":{"id":92,"last":"0.00006099","lowestAsk":"0.00006090","highestBid":"0.00006034","percentChange":"-0.01549636","baseVolume":"28.60142171","quoteVolume":"466305.29034872","isFrozen":"0","high24hr":"0.00006363","low24hr":"0.00006015"},"BTC_VIA":{"id":97,"last":"0.00024391","lowestAsk":"0.00024373","highestBid":"0.00024119","percentChange":"-0.02044176","baseVolume":"8.41407416","quoteVolume":"33693.34977288","isFrozen":"0","high24hr":"0.00025572","low24hr":"0.00024062"},"BTC_XVC":{"id":98,"last":"0.00004138","lowestAsk":"0.00004191","highestBid":"0.00004138","percentChange":"-0.00409145","baseVolume":"0.62065903","quoteVolume":"14793.19876026","isFrozen":"0","high24hr":"0.00004397","low24hr":"0.00004101"},"BTC_VRC":{"id":99,"last":"0.00008190","lowestAsk":"0.00008200","highestBid":"0.00008139","percentChange":"-0.00967351","baseVolume":"22.68904050","quoteVolume":"269368.45138333","isFrozen":"0","high24hr":"0.00008855","low24hr":"0.00008084"},"BTC_VTC":{"id":100,"last":"0.00036194","lowestAsk":"0.00036194","highestBid":"0.00036193","percentChange":"-0.07950152","baseVolume":"19.39531405","quoteVolume":"51120.56163987","isFrozen":"0","high24hr":"0.00039357","low24hr":"0.00036160"},"BTC_XBC":{"id":104,"last":"0.00705084","lowestAsk":"0.00705084","highestBid":"0.00697113","percentChange":"-0.00251816","baseVolume":"0.69091585","quoteVolume":"97.55912960","isFrozen":"0","high24hr":"0.00717000","low24hr":"0.00697112"},"BTC_XCP":{"id":108,"last":"0.00207553","lowestAsk":"0.00207553","highestBid":"0.00206080","percentChange":"0.00343255","baseVolume":"8.65748955","quoteVolume":"4218.24473378","isFrozen":"0","high24hr":"0.00213802","low24hr":"0.00200000"},"BTC_XEM":{"id":112,"last":"0.00003850","lowestAsk":"0.00003850","highestBid":"0.00003846","percentChange":"0.05335157","baseVolume":"186.76423244","quoteVolume":"4880505.48589732","isFrozen":"0","high24hr":"0.00003999","low24hr":"0.00003630"},"BTC_XMR":{"id":114,"last":"0.02755976","lowestAsk":"0.02755980","highestBid":"0.02755976","percentChange":"-0.01360704","baseVolume":"383.72144157","quoteVolume":"13765.56273024","isFrozen":"0","high24hr":"0.02840041","low24hr":"0.02750000"},"BTC_XPM":{"id":116,"last":"0.00008172","lowestAsk":"0.00008268","highestBid":"0.00008173","percentChange":"-0.14312676","baseVolume":"22.77991718","quoteVolume":"246492.03590984","isFrozen":"0","high24hr":"0.00010400","low24hr":"0.00008028"},"BTC_XRP":{"id":117,"last":"0.00008535","lowestAsk":"0.00008545","highestBid":"0.00008535","percentChange":"-0.02009184","baseVolume":"1329.81359724","quoteVolume":"15483518.38295366","isFrozen":"0","high24hr":"0.00008843","low24hr":"0.00008011"},"USDT_BTC":{"id":121,"last":"10624.99998773","lowestAsk":"10624.99998664","highestBid":"10608.00000003","percentChange":"-0.00692886","baseVolume":"35691429.96539170","quoteVolume":"3332.58429269","isFrozen":"0","high24hr":"11074.00000000","low24hr":"10469.32778879"},"USDT_DASH":{"id":122,"last":"600.00000000","lowestAsk":"599.99999991","highestBid":"596.93035101","percentChange":"-0.03219404","baseVolume":"1283299.41066996","quoteVolume":"2098.92266394","isFrozen":"0","high24hr":"622.57893075","low24hr":"591.95179691"},"USDT_LTC":{"id":123,"last":"210.95749000","lowestAsk":"210.94748953","highestBid":"209.88560829","percentChange":"-0.03787506","baseVolume":"4594398.92543038","quoteVolume":"21232.61767653","isFrozen":"0","high24hr":"223.54000007","low24hr":"208.20000000"},"USDT_NXT":{"id":124,"last":"0.20223804","lowestAsk":"0.20325222","highestBid":"0.20223804","percentChange":"-0.05806965","baseVolume":"603478.19811368","quoteVolume":"2885725.55969930","isFrozen":"0","high24hr":"0.21881127","low24hr":"0.19921189"},"USDT_STR":{"id":125,"last":"0.34222321","lowestAsk":"0.34222323","highestBid":"0.34222321","percentChange":"-0.05410942","baseVolume":"2295735.51730013","quoteVolume":"6456715.13838263","isFrozen":"0","high24hr":"0.36500000","low24hr":"0.33551234"},"USDT_XMR":{"id":126,"last":"292.67178868","lowestAsk":"292.67178807","highestBid":"291.00000421","percentChange":"-0.01620845","baseVolume":"1123622.33137267","quoteVolume":"3760.05453072","isFrozen":"0","high24hr":"304.77803231","low24hr":"290.00000002"},"USDT_XRP":{"id":127,"last":"0.90978146","lowestAsk":"0.90978000","highestBid":"0.90938146","percentChange":"-0.02979624","baseVolume":"3653275.18642841","quoteVolume":"3938239.54477194","isFrozen":"0","high24hr":"0.95303906","low24hr":"0.89500000"},"XMR_BCN":{"id":129,"last":"0.00001583","lowestAsk":"0.00001636","highestBid":"0.00001607","percentChange":"-0.04118715","baseVolume":"8.05137794","quoteVolume":"486388.87722166","isFrozen":"0","high24hr":"0.00001722","low24hr":"0.00001560"},"XMR_BLK":{"id":130,"last":"0.00114510","lowestAsk":"0.00114901","highestBid":"0.00113176","percentChange":"-0.01076402","baseVolume":"1.72978309","quoteVolume":"1516.34903291","isFrozen":"0","high24hr":"0.00117441","low24hr":"0.00111542"},"XMR_BTCD":{"id":131,"last":"0.35215967","lowestAsk":"0.35215967","highestBid":"0.34902601","percentChange":"-0.00019359","baseVolume":"2.42824965","quoteVolume":"6.96217009","isFrozen":"0","high24hr":"0.35950689","low24hr":"0.34687854"},"XMR_DASH":{"id":132,"last":"2.04288022","lowestAsk":"2.04599999","highestBid":"2.03300002","percentChange":"-0.02441250","baseVolume":"12.77948163","quoteVolume":"6.29569469","isFrozen":"0","high24hr":"2.07725856","low24hr":"2.00970004"},"XMR_LTC":{"id":137,"last":"0.71559210","lowestAsk":"0.72490000","highestBid":"0.71559210","percentChange":"-0.00999425","baseVolume":"42.29974914","quoteVolume":"58.22486474","isFrozen":"0","high24hr":"0.74179000","low24hr":"0.71510000"},"XMR_MAID":{"id":138,"last":"0.00127178","lowestAsk":"0.00129599","highestBid":"0.00126490","percentChange":"0.05855522","baseVolume":"13.56874950","quoteVolume":"10541.32711178","isFrozen":"0","high24hr":"0.00137807","low24hr":"0.00115701"},"XMR_NXT":{"id":140,"last":"0.00069503","lowestAsk":"0.00069464","highestBid":"0.00068597","percentChange":"-0.02257129","baseVolume":"2.89409514","quoteVolume":"4144.04804373","isFrozen":"0","high24hr":"0.00072300","low24hr":"0.00068330"},"BTC_ETH":{"id":148,"last":"0.08184499","lowestAsk":"0.08184000","highestBid":"0.08179850","percentChange":"-0.00580760","baseVolume":"1533.30420352","quoteVolume":"18714.83519723","isFrozen":"0","high24hr":"0.08284246","low24hr":"0.07985001"},"USDT_ETH":{"id":149,"last":"869.39534497","lowestAsk":"869.52999973","highestBid":"867.61588914","percentChange":"-0.01455483","baseVolume":"3855929.24959329","quoteVolume":"4396.20321972","isFrozen":"0","high24hr":"890.01000000","low24hr":"858.87562001"},"BTC_SC":{"id":150,"last":"0.00000187","lowestAsk":"0.00000187","highestBid":"0.00000186","percentChange":"-0.02604166","baseVolume":"94.57654683","quoteVolume":"49360230.33320522","isFrozen":"0","high24hr":"0.00000199","low24hr":"0.00000186"},"BTC_BCY":{"id":151,"last":"0.00004390","lowestAsk":"0.00004442","highestBid":"0.00004370","percentChange":"-0.01701746","baseVolume":"1.80036972","quoteVolume":"41088.53490459","isFrozen":"0","high24hr":"0.00004500","low24hr":"0.00004278"},"BTC_EXP":{"id":153,"last":"0.00025868","lowestAsk":"0.00025868","highestBid":"0.00025822","percentChange":"-0.00675779","baseVolume":"4.84435099","quoteVolume":"18298.59663925","isFrozen":"0","high24hr":"0.00028105","low24hr":"0.00025512"},"BTC_FCT":{"id":155,"last":"0.00310006","lowestAsk":"0.00312607","highestBid":"0.00310006","percentChange":"0.03817044","baseVolume":"93.68957267","quoteVolume":"29302.05113453","isFrozen":"0","high24hr":"0.00342000","low24hr":"0.00295864"},"BTC_RADS":{"id":158,"last":"0.00055595","lowestAsk":"0.00055603","highestBid":"0.00055595","percentChange":"0.00171171","baseVolume":"2.40792647","quoteVolume":"4384.29903283","isFrozen":"0","high24hr":"0.00056004","low24hr":"0.00054184"},"BTC_AMP":{"id":160,"last":"0.00003266","lowestAsk":"0.00003266","highestBid":"0.00003237","percentChange":"0.12233676","baseVolume":"22.15341699","quoteVolume":"700411.61775904","isFrozen":"0","high24hr":"0.00003470","low24hr":"0.00002900"},"BTC_DCR":{"id":162,"last":"0.00700000","lowestAsk":"0.00700000","highestBid":"0.00699626","percentChange":"-0.02845246","baseVolume":"71.68165727","quoteVolume":"10026.64310783","isFrozen":"0","high24hr":"0.00740000","low24hr":"0.00699625"},"BTC_LSK":{"id":163,"last":"0.00177549","lowestAsk":"0.00177549","highestBid":"0.00177087","percentChange":"-0.05370790","baseVolume":"76.77220271","quoteVolume":"41640.05985932","isFrozen":"0","high24hr":"0.00191517","low24hr":"0.00176237"},"ETH_LSK":{"id":166,"last":"0.02170507","lowestAsk":"0.02196807","highestBid":"0.02170507","percentChange":"-0.05219193","baseVolume":"153.34380628","quoteVolume":"6817.51468673","isFrozen":"0","high24hr":"0.02338368","low24hr":"0.02170507"},"BTC_LBC":{"id":167,"last":"0.00003495","lowestAsk":"0.00003495","highestBid":"0.00003485","percentChange":"-0.02265100","baseVolume":"16.53299322","quoteVolume":"471422.21906920","isFrozen":"0","high24hr":"0.00003776","low24hr":"0.00003366"},"BTC_STEEM":{"id":168,"last":"0.00029382","lowestAsk":"0.00029527","highestBid":"0.00028997","percentChange":"-0.07405773","baseVolume":"18.15624885","quoteVolume":"59642.97013872","isFrozen":"0","high24hr":"0.00031830","low24hr":"0.00028992"},"ETH_STEEM":{"id":169,"last":"0.00358331","lowestAsk":"0.00361200","highestBid":"0.00358346","percentChange":"-0.05709812","baseVolume":"14.63196451","quoteVolume":"3981.26836671","isFrozen":"0","high24hr":"0.00383840","low24hr":"0.00357319"},"BTC_SBD":{"id":170,"last":"0.00032115","lowestAsk":"0.00032300","highestBid":"0.00032254","percentChange":"-0.04906431","baseVolume":"0.79186808","quoteVolume":"2398.48810574","isFrozen":"0","high24hr":"0.00034255","low24hr":"0.00032003"},"BTC_ETC":{"id":171,"last":"0.00318050","lowestAsk":"0.00318003","highestBid":"0.00318000","percentChange":"-0.05426702","baseVolume":"548.96356016","quoteVolume":"167054.79595649","isFrozen":"0","high24hr":"0.00341741","low24hr":"0.00315000"},"ETH_ETC":{"id":172,"last":"0.03909435","lowestAsk":"0.03909417","highestBid":"0.03881661","percentChange":"-0.04181641","baseVolume":"580.95719579","quoteVolume":"14458.59615482","isFrozen":"0","high24hr":"0.04152581","low24hr":"0.03881692"},"USDT_ETC":{"id":173,"last":"33.87000000","lowestAsk":"33.88819156","highestBid":"33.85265630","percentChange":"-0.05628309","baseVolume":"4799570.40286407","quoteVolume":"136788.35369587","isFrozen":"0","high24hr":"36.39788875","low24hr":"33.09999997"},"BTC_REP":{"id":174,"last":"0.00440716","lowestAsk":"0.00442112","highestBid":"0.00441317","percentChange":"-0.03114427","baseVolume":"29.79618928","quoteVolume":"6464.80143022","isFrozen":"0","high24hr":"0.00488758","low24hr":"0.00440168"},"USDT_REP":{"id":175,"last":"46.63290958","lowestAsk":"46.63290958","highestBid":"46.63290910","percentChange":"-0.03918645","baseVolume":"254489.36280387","quoteVolume":"5145.92607861","isFrozen":"0","high24hr":"52.12894591","low24hr":"46.63290958"},"ETH_REP":{"id":176,"last":"0.05408318","lowestAsk":"0.05425802","highestBid":"0.05408318","percentChange":"-0.01386902","baseVolume":"94.24453787","quoteVolume":"1652.51345408","isFrozen":"0","high24hr":"0.05923229","low24hr":"0.05408318"},"BTC_ARDR":{"id":177,"last":"0.00003701","lowestAsk":"0.00003721","highestBid":"0.00003702","percentChange":"-0.06185044","baseVolume":"12.30772940","quoteVolume":"318929.56988432","isFrozen":"0","high24hr":"0.00004087","low24hr":"0.00003701"},"BTC_ZEC":{"id":178,"last":"0.03716137","lowestAsk":"0.03722000","highestBid":"0.03716137","percentChange":"-0.03977301","baseVolume":"141.72242798","quoteVolume":"3726.05946816","isFrozen":"0","high24hr":"0.03890397","low24hr":"0.03711373"},"ETH_ZEC":{"id":179,"last":"0.45847077","lowestAsk":"0.45860212","highestBid":"0.45593483","percentChange":"-0.02721483","baseVolume":"28.88945904","quoteVolume":"62.20448974","isFrozen":"0","high24hr":"0.47321875","low24hr":"0.45469474"},"USDT_ZEC":{"id":180,"last":"394.35039285","lowestAsk":"397.12551178","highestBid":"395.00000000","percentChange":"-0.04279766","baseVolume":"506512.38061647","quoteVolume":"1240.40556073","isFrozen":"0","high24hr":"418.68484294","low24hr":"392.70000000"},"XMR_ZEC":{"id":181,"last":"1.35941597","lowestAsk":"1.36317677","highestBid":"1.34500001","percentChange":"-0.01906218","baseVolume":"20.96692015","quoteVolume":"15.32647405","isFrozen":"0","high24hr":"1.40000000","low24hr":"1.33318373"},"BTC_STRAT":{"id":182,"last":"0.00071293","lowestAsk":"0.00071946","highestBid":"0.00071293","percentChange":"-0.01243922","baseVolume":"40.42123101","quoteVolume":"55776.25886482","isFrozen":"0","high24hr":"0.00074500","low24hr":"0.00070500"},"BTC_NXC":{"id":183,"last":"0.00002000","lowestAsk":"0.00002000","highestBid":"0.00001990","percentChange":"0.00553041","baseVolume":"0.69218465","quoteVolume":"34624.52960262","isFrozen":"0","high24hr":"0.00002096","low24hr":"0.00001969"},"BTC_PASC":{"id":184,"last":"0.00013496","lowestAsk":"0.00013530","highestBid":"0.00013496","percentChange":"-0.13214584","baseVolume":"15.50617150","quoteVolume":"107817.40106833","isFrozen":"0","high24hr":"0.00015700","low24hr":"0.00013059"},"BTC_GNT":{"id":185,"last":"0.00003309","lowestAsk":"0.00003322","highestBid":"0.00003309","percentChange":"-0.05618938","baseVolume":"16.62020155","quoteVolume":"487963.62072953","isFrozen":"0","high24hr":"0.00003549","low24hr":"0.00003309"},"ETH_GNT":{"id":186,"last":"0.00040941","lowestAsk":"0.00040876","highestBid":"0.00040430","percentChange":"-0.04372503","baseVolume":"10.09480288","quoteVolume":"24137.14149454","isFrozen":"0","high24hr":"0.00042840","low24hr":"0.00040478"},"BTC_GNO":{"id":187,"last":"0.01225507","lowestAsk":"0.01244914","highestBid":"0.01225555","percentChange":"-0.03210425","baseVolume":"1.58931513","quoteVolume":"128.22232126","isFrozen":"0","high24hr":"0.01266156","low24hr":"0.01220000"},"ETH_GNO":{"id":188,"last":"0.15050303","lowestAsk":"0.15229149","highestBid":"0.15071966","percentChange":"-0.01625557","baseVolume":"8.67687758","quoteVolume":"57.38001905","isFrozen":"0","high24hr":"0.15525901","low24hr":"0.15050000"},"BTC_BCH":{"id":189,"last":"0.11518232","lowestAsk":"0.11534375","highestBid":"0.11528132","percentChange":"-0.03500843","baseVolume":"275.98819978","quoteVolume":"2371.47230887","isFrozen":"0","high24hr":"0.11964799","low24hr":"0.11407690"},"ETH_BCH":{"id":190,"last":"1.40300000","lowestAsk":"1.41500000","highestBid":"1.40212240","percentChange":"-0.03308063","baseVolume":"161.67026842","quoteVolume":"113.37256684","isFrozen":"0","high24hr":"1.45321418","low24hr":"1.39957466"},"USDT_BCH":{"id":191,"last":"1224.19537308","lowestAsk":"1224.26904572","highestBid":"1220.51173609","percentChange":"-0.03788480","baseVolume":"1739159.36883248","quoteVolume":"1395.20317541","isFrozen":"0","high24hr":"1290.07329597","low24hr":"1200.00000000"},"BTC_ZRX":{"id":192,"last":"0.00009060","lowestAsk":"0.00009092","highestBid":"0.00009060","percentChange":"-0.07664084","baseVolume":"84.24667407","quoteVolume":"866347.75724553","isFrozen":"0","high24hr":"0.00010789","low24hr":"0.00009000"},"ETH_ZRX":{"id":193,"last":"0.00110985","lowestAsk":"0.00111195","highestBid":"0.00110400","percentChange":"-0.06894121","baseVolume":"97.55442267","quoteVolume":"81734.47819416","isFrozen":"0","high24hr":"0.00131243","low24hr":"0.00110100"},"BTC_CVC":{"id":194,"last":"0.00003385","lowestAsk":"0.00003386","highestBid":"0.00003375","percentChange":"-0.01052323","baseVolume":"10.95368997","quoteVolume":"317216.20691091","isFrozen":"0","high24hr":"0.00003600","low24hr":"0.00003356"},"ETH_CVC":{"id":195,"last":"0.00041877","lowestAsk":"0.00041995","highestBid":"0.00041457","percentChange":"0.00901139","baseVolume":"16.45389610","quoteVolume":"38944.62157283","isFrozen":"0","high24hr":"0.00042999","low24hr":"0.00040678"},"BTC_OMG":{"id":196,"last":"0.00188805","lowestAsk":"0.00189927","highestBid":"0.00188806","percentChange":"0.07584874","baseVolume":"294.90308428","quoteVolume":"157895.60549944","isFrozen":"0","high24hr":"0.00194765","low24hr":"0.00174590"},"ETH_OMG":{"id":197,"last":"0.02324351","lowestAsk":"0.02324380","highestBid":"0.02299925","percentChange":"0.10140545","baseVolume":"309.24005022","quoteVolume":"13519.35720959","isFrozen":"0","high24hr":"0.02370911","low24hr":"0.02128299"},"BTC_GAS":{"id":198,"last":"0.00374597","lowestAsk":"0.00374597","highestBid":"0.00373248","percentChange":"-0.08963718","baseVolume":"19.77538762","quoteVolume":"5062.54665375","isFrozen":"0","high24hr":"0.00413748","low24hr":"0.00371000"},"ETH_GAS":{"id":199,"last":"0.04561716","lowestAsk":"0.04561716","highestBid":"0.04561374","percentChange":"-0.08874920","baseVolume":"29.59015810","quoteVolume":"617.68366442","isFrozen":"0","high24hr":"0.05040887","low24hr":"0.04561624"},"BTC_STORJ":{"id":200,"last":"0.00008461","lowestAsk":"0.00008500","highestBid":"0.00008462","percentChange":"-0.03567358","baseVolume":"5.29465126","quoteVolume":"60905.94469730","isFrozen":"0","high24hr":"0.00008960","low24hr":"0.00008460"}}`
	client := newTestPoloniexPublicClient(&FakeRoundTripper{message: jsonTicker, status: http.StatusOK})
	volume, err := client.Volume("BCN", "BTC")
	if err != nil {
		panic(err)
	}
	if volume != 29.09099079 {
		t.Errorf("PoloniexPublicApi: Expected %v. Got %v", 29.09099079, volume)
	}
}

func TestPoloniexCurrencyPairs(t *testing.T) {

	jsonTicker := `{"BTC_BCN":{"id":7,"last":"0.00000044","lowestAsk":"0.00000044","highestBid":"0.00000043","percentChange":"-0.04347826","baseVolume":"29.09099079","quoteVolume":"64263958.33949675","isFrozen":"0","high24hr":"0.00000048","low24hr":"0.00000042"},"BTC_BELA":{"id":8,"last":"0.00001605","lowestAsk":"0.00001612","highestBid":"0.00001606","percentChange":"-0.08022922","baseVolume":"4.07014224","quoteVolume":"239482.67219866","isFrozen":"0","high24hr":"0.00001767","low24hr":"0.00001601"},"BTC_BLK":{"id":10,"last":"0.00003141","lowestAsk":"0.00003141","highestBid":"0.00003119","percentChange":"-0.03620742","baseVolume":"5.25336081","quoteVolume":"164929.08275402","isFrozen":"0","high24hr":"0.00003285","low24hr":"0.00003101"},"BTC_BTCD":{"id":12,"last":"0.00979795","lowestAsk":"0.00979549","highestBid":"0.00975102","percentChange":"-0.03547155","baseVolume":"1.09034776","quoteVolume":"111.38118807","isFrozen":"0","high24hr":"0.01000535","low24hr":"0.00975034"},"BTC_BTM":{"id":13,"last":"0.00008519","lowestAsk":"0.00008696","highestBid":"0.00008520","percentChange":"0.12033140","baseVolume":"5.75976561","quoteVolume":"69069.35601392","isFrozen":"0","high24hr":"0.00009258","low24hr":"0.00007000"},"BTC_BTS":{"id":14,"last":"0.00002029","lowestAsk":"0.00002028","highestBid":"0.00002022","percentChange":"-0.01120857","baseVolume":"79.53976080","quoteVolume":"3889105.34421891","isFrozen":"0","high24hr":"0.00002110","low24hr":"0.00002000"},"BTC_BURST":{"id":15,"last":"0.00000360","lowestAsk":"0.00000362","highestBid":"0.00000360","percentChange":"0.18811881","baseVolume":"78.38171781","quoteVolume":"22280856.88496521","isFrozen":"0","high24hr":"0.00000389","low24hr":"0.00000302"},"BTC_CLAM":{"id":20,"last":"0.00053002","lowestAsk":"0.00053498","highestBid":"0.00053002","percentChange":"-0.01229920","baseVolume":"3.67717167","quoteVolume":"6823.55539077","isFrozen":"0","high24hr":"0.00055182","low24hr":"0.00052990"},"BTC_DASH":{"id":24,"last":"0.05637575","lowestAsk":"0.05664179","highestBid":"0.05637631","percentChange":"-0.01845420","baseVolume":"209.06256707","quoteVolume":"3699.61400244","isFrozen":"0","high24hr":"0.05859546","low24hr":"0.05498114"},"BTC_DGB":{"id":25,"last":"0.00000329","lowestAsk":"0.00000329","highestBid":"0.00000327","percentChange":"-0.04081632","baseVolume":"57.95039019","quoteVolume":"17129885.46202723","isFrozen":"0","high24hr":"0.00000347","low24hr":"0.00000324"},"BTC_DOGE":{"id":27,"last":"0.00000059","lowestAsk":"0.00000059","highestBid":"0.00000058","percentChange":"-0.01666666","baseVolume":"192.26309111","quoteVolume":"330655712.41955251","isFrozen":"0","high24hr":"0.00000061","low24hr":"0.00000055"},"BTC_EMC2":{"id":28,"last":"0.00002755","lowestAsk":"0.00002782","highestBid":"0.00002755","percentChange":"-0.03536414","baseVolume":"9.66700911","quoteVolume":"342687.43192313","isFrozen":"0","high24hr":"0.00002971","low24hr":"0.00002735"},"BTC_FLDC":{"id":31,"last":"0.00000246","lowestAsk":"0.00000247","highestBid":"0.00000246","percentChange":"0.00819672","baseVolume":"1.16473043","quoteVolume":"473001.75777511","isFrozen":"0","high24hr":"0.00000254","low24hr":"0.00000242"},"BTC_FLO":{"id":32,"last":"0.00000957","lowestAsk":"0.00000968","highestBid":"0.00000958","percentChange":"0.00525210","baseVolume":"2.08213321","quoteVolume":"214758.93398200","isFrozen":"0","high24hr":"0.00000991","low24hr":"0.00000926"},"BTC_GAME":{"id":38,"last":"0.00019178","lowestAsk":"0.00019178","highestBid":"0.00019158","percentChange":"-0.03004248","baseVolume":"27.09161532","quoteVolume":"132787.64539663","isFrozen":"0","high24hr":"0.00021536","low24hr":"0.00019005"},"BTC_GRC":{"id":40,"last":"0.00000610","lowestAsk":"0.00000620","highestBid":"0.00000610","percentChange":"-0.06441717","baseVolume":"3.04065612","quoteVolume":"489424.75359611","isFrozen":"0","high24hr":"0.00000660","low24hr":"0.00000607"},"BTC_HUC":{"id":43,"last":"0.00002365","lowestAsk":"0.00002365","highestBid":"0.00002354","percentChange":"-0.04289761","baseVolume":"0.72457847","quoteVolume":"30161.52149021","isFrozen":"0","high24hr":"0.00002474","low24hr":"0.00002316"},"BTC_LTC":{"id":50,"last":"0.01978000","lowestAsk":"0.01977999","highestBid":"0.01977410","percentChange":"-0.03653331","baseVolume":"1167.53090263","quoteVolume":"57502.88609392","isFrozen":"0","high24hr":"0.02077132","low24hr":"0.01970000"},"BTC_MAID":{"id":51,"last":"0.00003518","lowestAsk":"0.00003518","highestBid":"0.00003498","percentChange":"0.03837072","baseVolume":"132.02767750","quoteVolume":"3651418.39478196","isFrozen":"0","high24hr":"0.00003934","low24hr":"0.00003257"},"BTC_OMNI":{"id":58,"last":"0.00369998","lowestAsk":"0.00369998","highestBid":"0.00364190","percentChange":"0.02343116","baseVolume":"1.79218231","quoteVolume":"489.38759050","isFrozen":"0","high24hr":"0.00373942","low24hr":"0.00361527"},"BTC_NAV":{"id":61,"last":"0.00017341","lowestAsk":"0.00017337","highestBid":"0.00017293","percentChange":"-0.00970818","baseVolume":"7.70942666","quoteVolume":"44717.15737983","isFrozen":"0","high24hr":"0.00017976","low24hr":"0.00016813"},"BTC_NEOS":{"id":63,"last":"0.00039001","lowestAsk":"0.00039037","highestBid":"0.00039000","percentChange":"-0.05828805","baseVolume":"3.28758311","quoteVolume":"8261.17380835","isFrozen":"0","high24hr":"0.00041902","low24hr":"0.00039000"},"BTC_NMC":{"id":64,"last":"0.00024009","lowestAsk":"0.00024125","highestBid":"0.00024009","percentChange":"-0.04017750","baseVolume":"0.52687827","quoteVolume":"2141.10580645","isFrozen":"0","high24hr":"0.00025264","low24hr":"0.00024009"},"BTC_NXT":{"id":69,"last":"0.00001911","lowestAsk":"0.00001912","highestBid":"0.00001911","percentChange":"-0.04735792","baseVolume":"32.54118716","quoteVolume":"1666537.75384808","isFrozen":"0","high24hr":"0.00002021","low24hr":"0.00001893"},"BTC_PINK":{"id":73,"last":"0.00000281","lowestAsk":"0.00000283","highestBid":"0.00000280","percentChange":"-0.02090592","baseVolume":"1.20652265","quoteVolume":"427629.44370182","isFrozen":"0","high24hr":"0.00000292","low24hr":"0.00000278"},"BTC_POT":{"id":74,"last":"0.00001522","lowestAsk":"0.00001528","highestBid":"0.00001522","percentChange":"-0.02933673","baseVolume":"3.39324883","quoteVolume":"218770.38580925","isFrozen":"0","high24hr":"0.00001606","low24hr":"0.00001510"},"BTC_PPC":{"id":75,"last":"0.00029162","lowestAsk":"0.00029162","highestBid":"0.00028701","percentChange":"-0.06150033","baseVolume":"12.46475422","quoteVolume":"41523.06111629","isFrozen":"0","high24hr":"0.00031600","low24hr":"0.00027900"},"BTC_RIC":{"id":83,"last":"0.00002639","lowestAsk":"0.00002683","highestBid":"0.00002651","percentChange":"-0.08748271","baseVolume":"53.36435932","quoteVolume":"1894264.91718601","isFrozen":"0","high24hr":"0.00003439","low24hr":"0.00002486"},"BTC_STR":{"id":89,"last":"0.00003220","lowestAsk":"0.00003220","highestBid":"0.00003215","percentChange":"-0.04394299","baseVolume":"528.89081221","quoteVolume":"16175607.89367209","isFrozen":"0","high24hr":"0.00003411","low24hr":"0.00003143"},"BTC_SYS":{"id":92,"last":"0.00006099","lowestAsk":"0.00006090","highestBid":"0.00006034","percentChange":"-0.01549636","baseVolume":"28.60142171","quoteVolume":"466305.29034872","isFrozen":"0","high24hr":"0.00006363","low24hr":"0.00006015"},"BTC_VIA":{"id":97,"last":"0.00024391","lowestAsk":"0.00024373","highestBid":"0.00024119","percentChange":"-0.02044176","baseVolume":"8.41407416","quoteVolume":"33693.34977288","isFrozen":"0","high24hr":"0.00025572","low24hr":"0.00024062"},"BTC_XVC":{"id":98,"last":"0.00004138","lowestAsk":"0.00004191","highestBid":"0.00004138","percentChange":"-0.00409145","baseVolume":"0.62065903","quoteVolume":"14793.19876026","isFrozen":"0","high24hr":"0.00004397","low24hr":"0.00004101"},"BTC_VRC":{"id":99,"last":"0.00008190","lowestAsk":"0.00008200","highestBid":"0.00008139","percentChange":"-0.00967351","baseVolume":"22.68904050","quoteVolume":"269368.45138333","isFrozen":"0","high24hr":"0.00008855","low24hr":"0.00008084"},"BTC_VTC":{"id":100,"last":"0.00036194","lowestAsk":"0.00036194","highestBid":"0.00036193","percentChange":"-0.07950152","baseVolume":"19.39531405","quoteVolume":"51120.56163987","isFrozen":"0","high24hr":"0.00039357","low24hr":"0.00036160"},"BTC_XBC":{"id":104,"last":"0.00705084","lowestAsk":"0.00705084","highestBid":"0.00697113","percentChange":"-0.00251816","baseVolume":"0.69091585","quoteVolume":"97.55912960","isFrozen":"0","high24hr":"0.00717000","low24hr":"0.00697112"},"BTC_XCP":{"id":108,"last":"0.00207553","lowestAsk":"0.00207553","highestBid":"0.00206080","percentChange":"0.00343255","baseVolume":"8.65748955","quoteVolume":"4218.24473378","isFrozen":"0","high24hr":"0.00213802","low24hr":"0.00200000"},"BTC_XEM":{"id":112,"last":"0.00003850","lowestAsk":"0.00003850","highestBid":"0.00003846","percentChange":"0.05335157","baseVolume":"186.76423244","quoteVolume":"4880505.48589732","isFrozen":"0","high24hr":"0.00003999","low24hr":"0.00003630"},"BTC_XMR":{"id":114,"last":"0.02755976","lowestAsk":"0.02755980","highestBid":"0.02755976","percentChange":"-0.01360704","baseVolume":"383.72144157","quoteVolume":"13765.56273024","isFrozen":"0","high24hr":"0.02840041","low24hr":"0.02750000"},"BTC_XPM":{"id":116,"last":"0.00008172","lowestAsk":"0.00008268","highestBid":"0.00008173","percentChange":"-0.14312676","baseVolume":"22.77991718","quoteVolume":"246492.03590984","isFrozen":"0","high24hr":"0.00010400","low24hr":"0.00008028"},"BTC_XRP":{"id":117,"last":"0.00008535","lowestAsk":"0.00008545","highestBid":"0.00008535","percentChange":"-0.02009184","baseVolume":"1329.81359724","quoteVolume":"15483518.38295366","isFrozen":"0","high24hr":"0.00008843","low24hr":"0.00008011"},"USDT_BTC":{"id":121,"last":"10624.99998773","lowestAsk":"10624.99998664","highestBid":"10608.00000003","percentChange":"-0.00692886","baseVolume":"35691429.96539170","quoteVolume":"3332.58429269","isFrozen":"0","high24hr":"11074.00000000","low24hr":"10469.32778879"},"USDT_DASH":{"id":122,"last":"600.00000000","lowestAsk":"599.99999991","highestBid":"596.93035101","percentChange":"-0.03219404","baseVolume":"1283299.41066996","quoteVolume":"2098.92266394","isFrozen":"0","high24hr":"622.57893075","low24hr":"591.95179691"},"USDT_LTC":{"id":123,"last":"210.95749000","lowestAsk":"210.94748953","highestBid":"209.88560829","percentChange":"-0.03787506","baseVolume":"4594398.92543038","quoteVolume":"21232.61767653","isFrozen":"0","high24hr":"223.54000007","low24hr":"208.20000000"},"USDT_NXT":{"id":124,"last":"0.20223804","lowestAsk":"0.20325222","highestBid":"0.20223804","percentChange":"-0.05806965","baseVolume":"603478.19811368","quoteVolume":"2885725.55969930","isFrozen":"0","high24hr":"0.21881127","low24hr":"0.19921189"},"USDT_STR":{"id":125,"last":"0.34222321","lowestAsk":"0.34222323","highestBid":"0.34222321","percentChange":"-0.05410942","baseVolume":"2295735.51730013","quoteVolume":"6456715.13838263","isFrozen":"0","high24hr":"0.36500000","low24hr":"0.33551234"},"USDT_XMR":{"id":126,"last":"292.67178868","lowestAsk":"292.67178807","highestBid":"291.00000421","percentChange":"-0.01620845","baseVolume":"1123622.33137267","quoteVolume":"3760.05453072","isFrozen":"0","high24hr":"304.77803231","low24hr":"290.00000002"},"USDT_XRP":{"id":127,"last":"0.90978146","lowestAsk":"0.90978000","highestBid":"0.90938146","percentChange":"-0.02979624","baseVolume":"3653275.18642841","quoteVolume":"3938239.54477194","isFrozen":"0","high24hr":"0.95303906","low24hr":"0.89500000"},"XMR_BCN":{"id":129,"last":"0.00001583","lowestAsk":"0.00001636","highestBid":"0.00001607","percentChange":"-0.04118715","baseVolume":"8.05137794","quoteVolume":"486388.87722166","isFrozen":"0","high24hr":"0.00001722","low24hr":"0.00001560"},"XMR_BLK":{"id":130,"last":"0.00114510","lowestAsk":"0.00114901","highestBid":"0.00113176","percentChange":"-0.01076402","baseVolume":"1.72978309","quoteVolume":"1516.34903291","isFrozen":"0","high24hr":"0.00117441","low24hr":"0.00111542"},"XMR_BTCD":{"id":131,"last":"0.35215967","lowestAsk":"0.35215967","highestBid":"0.34902601","percentChange":"-0.00019359","baseVolume":"2.42824965","quoteVolume":"6.96217009","isFrozen":"0","high24hr":"0.35950689","low24hr":"0.34687854"},"XMR_DASH":{"id":132,"last":"2.04288022","lowestAsk":"2.04599999","highestBid":"2.03300002","percentChange":"-0.02441250","baseVolume":"12.77948163","quoteVolume":"6.29569469","isFrozen":"0","high24hr":"2.07725856","low24hr":"2.00970004"},"XMR_LTC":{"id":137,"last":"0.71559210","lowestAsk":"0.72490000","highestBid":"0.71559210","percentChange":"-0.00999425","baseVolume":"42.29974914","quoteVolume":"58.22486474","isFrozen":"0","high24hr":"0.74179000","low24hr":"0.71510000"},"XMR_MAID":{"id":138,"last":"0.00127178","lowestAsk":"0.00129599","highestBid":"0.00126490","percentChange":"0.05855522","baseVolume":"13.56874950","quoteVolume":"10541.32711178","isFrozen":"0","high24hr":"0.00137807","low24hr":"0.00115701"},"XMR_NXT":{"id":140,"last":"0.00069503","lowestAsk":"0.00069464","highestBid":"0.00068597","percentChange":"-0.02257129","baseVolume":"2.89409514","quoteVolume":"4144.04804373","isFrozen":"0","high24hr":"0.00072300","low24hr":"0.00068330"},"BTC_ETH":{"id":148,"last":"0.08184499","lowestAsk":"0.08184000","highestBid":"0.08179850","percentChange":"-0.00580760","baseVolume":"1533.30420352","quoteVolume":"18714.83519723","isFrozen":"0","high24hr":"0.08284246","low24hr":"0.07985001"},"USDT_ETH":{"id":149,"last":"869.39534497","lowestAsk":"869.52999973","highestBid":"867.61588914","percentChange":"-0.01455483","baseVolume":"3855929.24959329","quoteVolume":"4396.20321972","isFrozen":"0","high24hr":"890.01000000","low24hr":"858.87562001"},"BTC_SC":{"id":150,"last":"0.00000187","lowestAsk":"0.00000187","highestBid":"0.00000186","percentChange":"-0.02604166","baseVolume":"94.57654683","quoteVolume":"49360230.33320522","isFrozen":"0","high24hr":"0.00000199","low24hr":"0.00000186"},"BTC_BCY":{"id":151,"last":"0.00004390","lowestAsk":"0.00004442","highestBid":"0.00004370","percentChange":"-0.01701746","baseVolume":"1.80036972","quoteVolume":"41088.53490459","isFrozen":"0","high24hr":"0.00004500","low24hr":"0.00004278"},"BTC_EXP":{"id":153,"last":"0.00025868","lowestAsk":"0.00025868","highestBid":"0.00025822","percentChange":"-0.00675779","baseVolume":"4.84435099","quoteVolume":"18298.59663925","isFrozen":"0","high24hr":"0.00028105","low24hr":"0.00025512"},"BTC_FCT":{"id":155,"last":"0.00310006","lowestAsk":"0.00312607","highestBid":"0.00310006","percentChange":"0.03817044","baseVolume":"93.68957267","quoteVolume":"29302.05113453","isFrozen":"0","high24hr":"0.00342000","low24hr":"0.00295864"},"BTC_RADS":{"id":158,"last":"0.00055595","lowestAsk":"0.00055603","highestBid":"0.00055595","percentChange":"0.00171171","baseVolume":"2.40792647","quoteVolume":"4384.29903283","isFrozen":"0","high24hr":"0.00056004","low24hr":"0.00054184"},"BTC_AMP":{"id":160,"last":"0.00003266","lowestAsk":"0.00003266","highestBid":"0.00003237","percentChange":"0.12233676","baseVolume":"22.15341699","quoteVolume":"700411.61775904","isFrozen":"0","high24hr":"0.00003470","low24hr":"0.00002900"},"BTC_DCR":{"id":162,"last":"0.00700000","lowestAsk":"0.00700000","highestBid":"0.00699626","percentChange":"-0.02845246","baseVolume":"71.68165727","quoteVolume":"10026.64310783","isFrozen":"0","high24hr":"0.00740000","low24hr":"0.00699625"},"BTC_LSK":{"id":163,"last":"0.00177549","lowestAsk":"0.00177549","highestBid":"0.00177087","percentChange":"-0.05370790","baseVolume":"76.77220271","quoteVolume":"41640.05985932","isFrozen":"0","high24hr":"0.00191517","low24hr":"0.00176237"},"ETH_LSK":{"id":166,"last":"0.02170507","lowestAsk":"0.02196807","highestBid":"0.02170507","percentChange":"-0.05219193","baseVolume":"153.34380628","quoteVolume":"6817.51468673","isFrozen":"0","high24hr":"0.02338368","low24hr":"0.02170507"},"BTC_LBC":{"id":167,"last":"0.00003495","lowestAsk":"0.00003495","highestBid":"0.00003485","percentChange":"-0.02265100","baseVolume":"16.53299322","quoteVolume":"471422.21906920","isFrozen":"0","high24hr":"0.00003776","low24hr":"0.00003366"},"BTC_STEEM":{"id":168,"last":"0.00029382","lowestAsk":"0.00029527","highestBid":"0.00028997","percentChange":"-0.07405773","baseVolume":"18.15624885","quoteVolume":"59642.97013872","isFrozen":"0","high24hr":"0.00031830","low24hr":"0.00028992"},"ETH_STEEM":{"id":169,"last":"0.00358331","lowestAsk":"0.00361200","highestBid":"0.00358346","percentChange":"-0.05709812","baseVolume":"14.63196451","quoteVolume":"3981.26836671","isFrozen":"0","high24hr":"0.00383840","low24hr":"0.00357319"},"BTC_SBD":{"id":170,"last":"0.00032115","lowestAsk":"0.00032300","highestBid":"0.00032254","percentChange":"-0.04906431","baseVolume":"0.79186808","quoteVolume":"2398.48810574","isFrozen":"0","high24hr":"0.00034255","low24hr":"0.00032003"},"BTC_ETC":{"id":171,"last":"0.00318050","lowestAsk":"0.00318003","highestBid":"0.00318000","percentChange":"-0.05426702","baseVolume":"548.96356016","quoteVolume":"167054.79595649","isFrozen":"0","high24hr":"0.00341741","low24hr":"0.00315000"},"ETH_ETC":{"id":172,"last":"0.03909435","lowestAsk":"0.03909417","highestBid":"0.03881661","percentChange":"-0.04181641","baseVolume":"580.95719579","quoteVolume":"14458.59615482","isFrozen":"0","high24hr":"0.04152581","low24hr":"0.03881692"},"USDT_ETC":{"id":173,"last":"33.87000000","lowestAsk":"33.88819156","highestBid":"33.85265630","percentChange":"-0.05628309","baseVolume":"4799570.40286407","quoteVolume":"136788.35369587","isFrozen":"0","high24hr":"36.39788875","low24hr":"33.09999997"},"BTC_REP":{"id":174,"last":"0.00440716","lowestAsk":"0.00442112","highestBid":"0.00441317","percentChange":"-0.03114427","baseVolume":"29.79618928","quoteVolume":"6464.80143022","isFrozen":"0","high24hr":"0.00488758","low24hr":"0.00440168"},"USDT_REP":{"id":175,"last":"46.63290958","lowestAsk":"46.63290958","highestBid":"46.63290910","percentChange":"-0.03918645","baseVolume":"254489.36280387","quoteVolume":"5145.92607861","isFrozen":"0","high24hr":"52.12894591","low24hr":"46.63290958"},"ETH_REP":{"id":176,"last":"0.05408318","lowestAsk":"0.05425802","highestBid":"0.05408318","percentChange":"-0.01386902","baseVolume":"94.24453787","quoteVolume":"1652.51345408","isFrozen":"0","high24hr":"0.05923229","low24hr":"0.05408318"},"BTC_ARDR":{"id":177,"last":"0.00003701","lowestAsk":"0.00003721","highestBid":"0.00003702","percentChange":"-0.06185044","baseVolume":"12.30772940","quoteVolume":"318929.56988432","isFrozen":"0","high24hr":"0.00004087","low24hr":"0.00003701"},"BTC_ZEC":{"id":178,"last":"0.03716137","lowestAsk":"0.03722000","highestBid":"0.03716137","percentChange":"-0.03977301","baseVolume":"141.72242798","quoteVolume":"3726.05946816","isFrozen":"0","high24hr":"0.03890397","low24hr":"0.03711373"},"ETH_ZEC":{"id":179,"last":"0.45847077","lowestAsk":"0.45860212","highestBid":"0.45593483","percentChange":"-0.02721483","baseVolume":"28.88945904","quoteVolume":"62.20448974","isFrozen":"0","high24hr":"0.47321875","low24hr":"0.45469474"},"USDT_ZEC":{"id":180,"last":"394.35039285","lowestAsk":"397.12551178","highestBid":"395.00000000","percentChange":"-0.04279766","baseVolume":"506512.38061647","quoteVolume":"1240.40556073","isFrozen":"0","high24hr":"418.68484294","low24hr":"392.70000000"},"XMR_ZEC":{"id":181,"last":"1.35941597","lowestAsk":"1.36317677","highestBid":"1.34500001","percentChange":"-0.01906218","baseVolume":"20.96692015","quoteVolume":"15.32647405","isFrozen":"0","high24hr":"1.40000000","low24hr":"1.33318373"},"BTC_STRAT":{"id":182,"last":"0.00071293","lowestAsk":"0.00071946","highestBid":"0.00071293","percentChange":"-0.01243922","baseVolume":"40.42123101","quoteVolume":"55776.25886482","isFrozen":"0","high24hr":"0.00074500","low24hr":"0.00070500"},"BTC_NXC":{"id":183,"last":"0.00002000","lowestAsk":"0.00002000","highestBid":"0.00001990","percentChange":"0.00553041","baseVolume":"0.69218465","quoteVolume":"34624.52960262","isFrozen":"0","high24hr":"0.00002096","low24hr":"0.00001969"},"BTC_PASC":{"id":184,"last":"0.00013496","lowestAsk":"0.00013530","highestBid":"0.00013496","percentChange":"-0.13214584","baseVolume":"15.50617150","quoteVolume":"107817.40106833","isFrozen":"0","high24hr":"0.00015700","low24hr":"0.00013059"},"BTC_GNT":{"id":185,"last":"0.00003309","lowestAsk":"0.00003322","highestBid":"0.00003309","percentChange":"-0.05618938","baseVolume":"16.62020155","quoteVolume":"487963.62072953","isFrozen":"0","high24hr":"0.00003549","low24hr":"0.00003309"},"ETH_GNT":{"id":186,"last":"0.00040941","lowestAsk":"0.00040876","highestBid":"0.00040430","percentChange":"-0.04372503","baseVolume":"10.09480288","quoteVolume":"24137.14149454","isFrozen":"0","high24hr":"0.00042840","low24hr":"0.00040478"},"BTC_GNO":{"id":187,"last":"0.01225507","lowestAsk":"0.01244914","highestBid":"0.01225555","percentChange":"-0.03210425","baseVolume":"1.58931513","quoteVolume":"128.22232126","isFrozen":"0","high24hr":"0.01266156","low24hr":"0.01220000"},"ETH_GNO":{"id":188,"last":"0.15050303","lowestAsk":"0.15229149","highestBid":"0.15071966","percentChange":"-0.01625557","baseVolume":"8.67687758","quoteVolume":"57.38001905","isFrozen":"0","high24hr":"0.15525901","low24hr":"0.15050000"},"BTC_BCH":{"id":189,"last":"0.11518232","lowestAsk":"0.11534375","highestBid":"0.11528132","percentChange":"-0.03500843","baseVolume":"275.98819978","quoteVolume":"2371.47230887","isFrozen":"0","high24hr":"0.11964799","low24hr":"0.11407690"},"ETH_BCH":{"id":190,"last":"1.40300000","lowestAsk":"1.41500000","highestBid":"1.40212240","percentChange":"-0.03308063","baseVolume":"161.67026842","quoteVolume":"113.37256684","isFrozen":"0","high24hr":"1.45321418","low24hr":"1.39957466"},"USDT_BCH":{"id":191,"last":"1224.19537308","lowestAsk":"1224.26904572","highestBid":"1220.51173609","percentChange":"-0.03788480","baseVolume":"1739159.36883248","quoteVolume":"1395.20317541","isFrozen":"0","high24hr":"1290.07329597","low24hr":"1200.00000000"},"BTC_ZRX":{"id":192,"last":"0.00009060","lowestAsk":"0.00009092","highestBid":"0.00009060","percentChange":"-0.07664084","baseVolume":"84.24667407","quoteVolume":"866347.75724553","isFrozen":"0","high24hr":"0.00010789","low24hr":"0.00009000"},"ETH_ZRX":{"id":193,"last":"0.00110985","lowestAsk":"0.00111195","highestBid":"0.00110400","percentChange":"-0.06894121","baseVolume":"97.55442267","quoteVolume":"81734.47819416","isFrozen":"0","high24hr":"0.00131243","low24hr":"0.00110100"},"BTC_CVC":{"id":194,"last":"0.00003385","lowestAsk":"0.00003386","highestBid":"0.00003375","percentChange":"-0.01052323","baseVolume":"10.95368997","quoteVolume":"317216.20691091","isFrozen":"0","high24hr":"0.00003600","low24hr":"0.00003356"},"ETH_CVC":{"id":195,"last":"0.00041877","lowestAsk":"0.00041995","highestBid":"0.00041457","percentChange":"0.00901139","baseVolume":"16.45389610","quoteVolume":"38944.62157283","isFrozen":"0","high24hr":"0.00042999","low24hr":"0.00040678"},"BTC_OMG":{"id":196,"last":"0.00188805","lowestAsk":"0.00189927","highestBid":"0.00188806","percentChange":"0.07584874","baseVolume":"294.90308428","quoteVolume":"157895.60549944","isFrozen":"0","high24hr":"0.00194765","low24hr":"0.00174590"},"ETH_OMG":{"id":197,"last":"0.02324351","lowestAsk":"0.02324380","highestBid":"0.02299925","percentChange":"0.10140545","baseVolume":"309.24005022","quoteVolume":"13519.35720959","isFrozen":"0","high24hr":"0.02370911","low24hr":"0.02128299"},"BTC_GAS":{"id":198,"last":"0.00374597","lowestAsk":"0.00374597","highestBid":"0.00373248","percentChange":"-0.08963718","baseVolume":"19.77538762","quoteVolume":"5062.54665375","isFrozen":"0","high24hr":"0.00413748","low24hr":"0.00371000"},"ETH_GAS":{"id":199,"last":"0.04561716","lowestAsk":"0.04561716","highestBid":"0.04561374","percentChange":"-0.08874920","baseVolume":"29.59015810","quoteVolume":"617.68366442","isFrozen":"0","high24hr":"0.05040887","low24hr":"0.04561624"},"BTC_STORJ":{"id":200,"last":"0.00008461","lowestAsk":"0.00008500","highestBid":"0.00008462","percentChange":"-0.03567358","baseVolume":"5.29465126","quoteVolume":"60905.94469730","isFrozen":"0","high24hr":"0.00008960","low24hr":"0.00008460"}}`
	client := newTestPoloniexPublicClient(&FakeRoundTripper{message: jsonTicker, status: http.StatusOK})
	pairs, err := client.CurrencyPairs()
	if err != nil {
		panic(err)
	}
	for _, _ = range pairs {
	}
}

func TestPoloniexBoard(t *testing.T) {

	jsonBoard := `{"asks":[["0.00001664",732.55279357],["0.00001665",57.47059057],["0.00001667",51.11441191],["0.00001668",10.06],["0.00001670",76.38797045],["0.00001671",54187.5139264],["0.00001672",6.8391264],["0.00001673",342.64370419],["0.00001675",35.73134328],["0.00001680",10.12036]],"bids":[["0.00001660",15060.24096385],["0.00001655",543.46163143],["0.00001653",626.73125375],["0.00001652",121.0653753],["0.00001651",60.5693519],["0.00001650",5599.01094336],["0.00001649",470.31331283],["0.00001648",840.56462378],["0.00001647",40],["0.00001645",25]],"isFrozen":"0","seq":71651883}`
	client := newTestPoloniexPublicClient(&FakeRoundTripper{message: jsonBoard, status: http.StatusOK})
	_, err := client.Board("BTC", "JPY")
	if err != nil {
		panic(err)
	}
}

func TestHitbtcRate(t *testing.T) {
	jsonSymbol := `[
	{
		"id": "ETHBTC",
		"baseCurrency": "ETH",
		"quoteCurrency": "BTC",
		"quantityIncrement": "0.001",
		"tickSize": "0.000001",
		"takeLiquidityRate": "0.001",
		"provideLiquidityRate": "-0.0001",
		"feeCurrency": "BTC"
	}
	]`
	jsonTicker := `[{
    "ask": "0.050043",
    "bid": "0.050042",
    "last": "0.050042",
    "open": "0.047800",
    "low": "0.047052",
    "high": "0.051679",
    "volume": "36456.720",
    "volumeQuote": "1782.625000",
    "timestamp": "2017-05-12T14:57:19.999Z",
    "symbol": "ETHBTC"
  }]`
	fakeRoundTripper := &FakeRoundTripper{message: jsonSymbol, status: http.StatusOK}
	client := newTestHitbtcPublicClient(fakeRoundTripper)
	fakeRoundTripper.message = jsonTicker
	rate, err := client.Rate("ETH", "BTC")
	if err != nil {
		panic(err)
	}
	if rate != 0.050042 {
		t.Errorf("HitbtcPublicApi: Expected %v. Got %v", 0.050042, rate)
	}
}

func TestHitbtcVolume(t *testing.T) {
	jsonSymbol := `[
	{
		"id": "ETHBTC",
		"baseCurrency": "ETH",
		"quoteCurrency": "BTC",
		"quantityIncrement": "0.001",
		"tickSize": "0.000001",
		"takeLiquidityRate": "0.001",
		"provideLiquidityRate": "-0.0001",
		"feeCurrency": "BTC"
	}
	]`
	jsonTicker := `[
  {
    "ask": "0.050043",
    "bid": "0.050042",
    "last": "0.050042",
    "open": "0.047800",
    "low": "0.047052",
    "high": "0.051679",
    "volume": "36456.720",
    "volumeQuote": "1782.625000",
    "timestamp": "2017-05-12T14:57:19.999Z",
    "symbol": "ETHBTC"
  }
]`
	fakeRoundTripper := &FakeRoundTripper{message: jsonSymbol, status: http.StatusOK}
	client := newTestHitbtcPublicClient(fakeRoundTripper)
	fakeRoundTripper.message = jsonTicker
	volume, err := client.Volume("ETH", "BTC")
	if err != nil {
		panic(err)
	}
	if volume != 36456.720 {
		t.Errorf("HitbtcPublicApi: Expected %v. Got %v", 36456.720, volume)
	}
}

func TestHitbtcCurrencyPairs(t *testing.T) {

	jsonSymbol := `[
	{
		"id": "ETHBTC",
		"baseCurrency": "ETH",
		"quoteCurrency": "BTC",
		"quantityIncrement": "0.001",
		"tickSize": "0.000001",
		"takeLiquidityRate": "0.001",
		"provideLiquidityRate": "-0.0001",
		"feeCurrency": "BTC"
	}
	]`
	jsonTicker := `[
  {
    "ask": "0.050043",
    "bid": "0.050042",
    "last": "0.050042",
    "open": "0.047800",
    "low": "0.047052",
    "high": "0.051679",
    "volume": "36456.720",
    "volumeQuote": "1782.625000",
    "timestamp": "2017-05-12T14:57:19.999Z",
    "symbol": "ETHBTC"
  }
]`
	fakeRoundTripper := &FakeRoundTripper{message: jsonSymbol, status: http.StatusOK}
	client := newTestHitbtcPublicClient(fakeRoundTripper)
	fakeRoundTripper.message = jsonTicker
	pairs, err := client.CurrencyPairs()
	if err != nil {
		panic(err)
	}
	for _, _ = range pairs {
	}
}

func TestHitbtcBoard(t *testing.T) {

	jsonBoard := `{"ask":[{"price":"0.046002","size":"0.088"},{"price":"0.046800","size":"0.200"}],"bid":[{"price":"0.046001","size":"0.005"},{"price":"0.046000","size":"0.200"}]}`
	client := newTestHitbtcPublicClient(&FakeRoundTripper{message: jsonBoard, status: http.StatusOK})
	_, err := client.Board("BTC", "JPY")
	if err != nil {
		panic(err)
	}
}

func TestHuobiRate(t *testing.T) {
	jsonSymbol := `{"status":"ok","data":[{"base-currency":"nas","quote-currency":"eth","price-precision":6,"amount-precision":4,"symbol-partition":"innovation"},{"base-currency":"eos","quote-currency":"eth","price-precision":8,"amount-precision":2,"symbol-partition":"main"},{"base-currency":"swftc","quote-currency":"btc","price-precision":8,"amount-precision":2,"symbol-partition":"innovation"},{"base-currency":"zec","quote-currency":"usdt","price-precision":2,"amount-precision":4,"symbol-partition":"main"},{"base-currency":"evx","quote-currency":"btc","price-precision":8,"amount-precision":2,"symbol-partition":"innovation"},{"base-currency":"mds","quote-currency":"eth","price-precision":8,"amount-precision":0,"symbol-partition":"innovation"}]}`
	jsonTicker := `{"status":"ok","ch":"market.naseth.detail.merged","ts":1520335882838,"tick":{"amount":285754.506381807669901550,"open":0.009318000000000000,"close":0.008959000000000000,"high":0.009385000000000000,"id":3404226217,"count":7073,"low":0.008800000000000000,"version":3404226217,"ask":[0.009001000000000000,74.000000000000000000],"vol":2618.884466247149233010811750000000000000,"bid":[0.008888000000000000,57.917400000000000000]}}`
	fakeRoundTripper := &FakeRoundTripper{message: jsonSymbol, status: http.StatusOK}
	client := newTestHuobiPublicClient(fakeRoundTripper)
	client.CurrencyPairs()
	fakeRoundTripper.message = jsonTicker
	rate, err := client.Rate("NAS", "ETH")
	if err != nil {
		t.Error(err)
	}
	if rate != 0.0089590 {
		t.Errorf("HuobiPublicApi: Expected %v. Got %v", 0.0089590, rate)
	}
}

func TestHuobiVolume(t *testing.T) {
	jsonSymbol := `{"status":"ok","data":[{"base-currency":"nas","quote-currency":"eth","price-precision":6,"amount-precision":4,"symbol-partition":"innovation"},{"base-currency":"eos","quote-currency":"eth","price-precision":8,"amount-precision":2,"symbol-partition":"main"},{"base-currency":"swftc","quote-currency":"btc","price-precision":8,"amount-precision":2,"symbol-partition":"innovation"},{"base-currency":"zec","quote-currency":"usdt","price-precision":2,"amount-precision":4,"symbol-partition":"main"},{"base-currency":"evx","quote-currency":"btc","price-precision":8,"amount-precision":2,"symbol-partition":"innovation"},{"base-currency":"mds","quote-currency":"eth","price-precision":8,"amount-precision":0,"symbol-partition":"innovation"}]}`
	jsonTicker := `{"status":"ok","ch":"market.naseth.detail.merged","ts":1520335882838,"tick":{"amount":285754.506381807669901550,"open":0.009318000000000000,"close":0.008959000000000000,"high":0.009385000000000000,"id":3404226217,"count":7073,"low":0.008800000000000000,"version":3404226217,"ask":[0.009001000000000000,74.000000000000000000],"vol":2618.884466247149233010811750000000000000,"bid":[0.008888000000000000,57.917400000000000000]}}`
	fakeRoundTripper := &FakeRoundTripper{message: jsonSymbol, status: http.StatusOK}
	client := newTestHuobiPublicClient(fakeRoundTripper)
	client.CurrencyPairs()
	fakeRoundTripper.message = jsonTicker
	volume, err := client.Volume("NAS", "ETH")
	if err != nil {
		panic(err)
	}
	if volume != 2618.88446624714923301081175 {
		t.Errorf("HuobiPublicApi: Expected %v. Got %v", 2618.88446624714923301081175, volume)
	}
}

func TestHuobiCurrencyPairs(t *testing.T) {
	jsonSymbol := `{"status":"ok","data":[{"base-currency":"nas","quote-currency":"eth","price-precision":6,"amount-precision":4,"symbol-partition":"innovation"},{"base-currency":"eos","quote-currency":"eth","price-precision":8,"amount-precision":2,"symbol-partition":"main"},{"base-currency":"swftc","quote-currency":"btc","price-precision":8,"amount-precision":2,"symbol-partition":"innovation"},{"base-currency":"zec","quote-currency":"usdt","price-precision":2,"amount-precision":4,"symbol-partition":"main"},{"base-currency":"evx","quote-currency":"btc","price-precision":8,"amount-precision":2,"symbol-partition":"innovation"},{"base-currency":"mds","quote-currency":"eth","price-precision":8,"amount-precision":0,"symbol-partition":"innovation"}]}`
	fakeRoundTripper := &FakeRoundTripper{message: jsonSymbol, status: http.StatusOK}
	client := newTestHuobiPublicClient(fakeRoundTripper)
	fakeRoundTripper.message = jsonSymbol
	pairs, err := client.CurrencyPairs()
	if err != nil {
		panic(err)
	}
	for _, _ = range pairs {
	}
}

func TestHuobiFrozenCurrency(t *testing.T) {
	jsonSymbol := `{"status":"ok","data":[{"name":"ela","display-name":"ELA","withdraw-precision":8,"currency-type":"eth","currency-partition":"pro","otc-enable":0,"deposit-min-amount":"0.1","withdraw-min-amount":"0.2","show-precision":"8","weight":"4995","visible":true,"deposit-desc":"","withdraw-desc":"","deposit-enabled":true,"withdraw-enabled":true,"currency-addr-with-tag":false,"fast-confirms":16,"safe-confirms":16},{"name":"bcx","display-name":"BCX","withdraw-precision":8,"currency-type":"eth","currency-partition":"pro","otc-enable":0,"deposit-min-amount":"1","withdraw-min-amount":"2","show-precision":"4","weight":"3000","visible":true,"deposit-desc":"","withdraw-desc":"","deposit-enabled":false,"withdraw-enabled":false,"currency-addr-with-tag":false,"fast-confirms":6,"safe-confirms":6},{"name":"sbtc","display-name":"SBTC","withdraw-precision":8,"currency-type":"eth","currency-partition":"pro","otc-enable":0,"deposit-min-amount":"0.001","withdraw-min-amount":"0.001","show-precision":"4","weight":"2999","visible":true,"deposit-desc":"","withdraw-desc":"","deposit-enabled":false,"withdraw-enabled":false,"currency-addr-with-tag":false,"fast-confirms":6,"safe-confirms":6},{"name":"etf","display-name":"ETF","withdraw-precision":8,"currency-type":"eth","currency-partition":"pro","otc-enable":0,"deposit-min-amount":"1","withdraw-min-amount":"1","show-precision":"8","weight":"2998","visible":true,"deposit-desc":"","withdraw-desc":"","deposit-enabled":false,"withdraw-enabled":false,"currency-addr-with-tag":false,"fast-confirms":6,"safe-confirms":6},{"name":"abt","display-name":"ABT","withdraw-precision":8,"currency-type":"eth","currency-partition":"pro","otc-enable":0,"deposit-min-amount":"2","withdraw-min-amount":"4","show-precision":"8","weight":"2989","visible":true,"deposit-desc":"","withdraw-desc":"","deposit-enabled":true,"withdraw-enabled":true,"currency-addr-with-tag":false,"fast-confirms":15,"safe-confirms":30},{"name":"ont","display-name":"ONT","withdraw-precision":8,"currency-type":"eth","currency-partition":"pro","otc-enable":0,"deposit-min-amount":"0.02","withdraw-min-amount":"0.04","show-precision":"8","weight":"2988","visible":true,"deposit-desc":"","withdraw-desc":"","deposit-enabled":true,"withdraw-enabled":false,"currency-addr-with-tag":false,"fast-confirms":1,"safe-confirms":1},{"name":"bt1","display-name":"BT1","withdraw-precision":8,"currency-type":"btc","currency-partition":"pro","otc-enable":0,"deposit-min-amount":"0.01","withdraw-min-amount":"0.01","show-precision":"4","weight":"1","visible":true,"deposit-desc":"","withdraw-desc":"","deposit-enabled":false,"withdraw-enabled":false,"currency-addr-with-tag":false,"fast-confirms":6,"safe-confirms":6}]}`
	fakeRoundTripper := &FakeRoundTripper{message: jsonSymbol, status: http.StatusOK}
	client := newTestHuobiPublicClient(fakeRoundTripper)
	fakeRoundTripper.message = jsonSymbol
	currencies, err := client.FrozenCurrency()
	if err != nil {
		panic(err)
	}
	for _, _ = range currencies {
	}
}

func TestHuobiBoard(t *testing.T) {
	jsonBoard := `{"status":"ok","ch":"market.ethusdt.depth.step5","ts":1520420586792,"tick":{"bids":[[782.000000000000000000,64.990900000000000000],[781.900000000000000000,0.151700000000000000],[781.600000000000000000,6.397000000000000000],[781.500000000000000000,2.175500000000000000],[781.200000000000000000,0.950000000000000000],[781.000000000000000000,1.388261892409029865],[780.900000000000000000,6.000000000000000000],[780.800000000000000000,1.000000000000000000],[780.500000000000000000,1.092500000000000000],[780.000000000000000000,41.101800000000000000],[779.900000000000000000,0.283800000000000000],[779.800000000000000000,9.939000000000000000],[779.600000000000000000,2.100000000000000000],[779.500000000000000000,1.960000000000000000],[779.200000000000000000,11.920000000000000000],[778.500000000000000000,8.121100000000000000],[778.000000000000000000,1.879300000000000000],[777.900000000000000000,1.128600000000000000],[777.700000000000000000,25.505300000000000000],[777.600000000000000000,3.838600000000000000]],"asks":[[782.200000000000000000,3.000000000000000000],[782.800000000000000000,15.000000000000000000],[783.100000000000000000,0.778400000000000000],[783.200000000000000000,0.071400000000000000],[783.400000000000000000,0.800000000000000000],[783.500000000000000000,2.547000000000000000],[783.600000000000000000,0.400000000000000000],[783.700000000000000000,10.456900000000000000],[783.800000000000000000,2.060000000000000000],[783.900000000000000000,6.928979539705826073],[784.000000000000000000,40.287900000000000000],[784.200000000000000000,5.000000000000000000],[784.600000000000000000,0.400000000000000000],[784.700000000000000000,0.838100000000000000],[784.800000000000000000,3.644600000000000000],[785.000000000000000000,35.140800000000000000],[785.400000000000000000,0.186000000000000000],[785.500000000000000000,0.843600000000000000],[785.700000000000000000,10.000000000000000000],[785.900000000000000000,0.127200000000000000]],"ts":1520420586047,"version":3452363876}}`
	client := newTestHuobiPublicClient(&FakeRoundTripper{message: jsonBoard, status: http.StatusOK})
	_, err := client.Board("BTC", "JPY")
	if err != nil {
		panic(err)
	}
}

func TestLbankCurrencyPairs(t *testing.T) {
	jsonSymbol := `[
  "bcc_eth","etc_btc","dbc_neo","eth_btc",
  "zec_btc","qtum_btc","sc_btc","ven_btc",
  "ven_eth","sc_eth","zec_eth"
]`
	fakeRoundTripper := &FakeRoundTripper{message: jsonSymbol, status: http.StatusOK}
	client := newTestLbankPublicClient(fakeRoundTripper)
	fakeRoundTripper.message = jsonSymbol
	_, err := client.CurrencyPairs()
	if err != nil {
		panic(err)
	}
}

func TestLbankBoard(t *testing.T) {
	jsonBoard := `{"asks":[[5370.4, 0.32],[5369.5, 0.28],[5369.24, 0.05],[5368.2, 0.079],[5367.9, 0.023]],"bids":[[5367.24, 0.32],[5367.16, 1.31],[5366.18, 0.56],[5366.03, 1.42],[5365.77, 2.64]]}`
	fakeRoundTripper := &FakeRoundTripper{message: jsonBoard, status: http.StatusOK}
	client := newTestLbankPublicClient(fakeRoundTripper)
	fakeRoundTripper.message = jsonBoard
	_, err := client.Board("EOS", "ETH")
	if err != nil {
		panic(err)
	}
}

func TestLbankRate(t *testing.T) {
	jsonSymbol := `[
  "bcc_eth","etc_btc","dbc_neo","eth_btc",
  "zec_btc","qtum_btc","sc_btc","ven_btc",
  "ven_eth","sc_eth","zec_eth"
]`
	jsonTicker := `[{"symbol":"eth_btc","timestamp":"1410431279000","ticker":{"change":"4.21","high":"7722.58","latest":"7682.29","low":"7348.30","turnover":"0.00","vol":"1316.3235"}},{"symbol":"sc_btc","timestamp":"1410431279000","ticker":{"change":"4.21","high":"7722.58","latest":"7682.29","low":"7348.30","turnover":"0.00","vol":"1316.3235"}}]`
	fakeRoundTripper := &FakeRoundTripper{message: jsonSymbol, status: http.StatusOK}
	client := newTestLbankPublicClient(fakeRoundTripper)
	client.CurrencyPairs()
	fakeRoundTripper.message = jsonTicker
	rate, err := client.Rate("ETH", "BTC")
	if err != nil {
		t.Error(err)
	}
	if rate != 7682.29 {
		t.Errorf("LbankPublicApi: Expected %v. Got %v", 7282.29, rate)
	}
}

func TestLbankVolume(t *testing.T) {
	jsonSymbol := `[
  "bcc_eth","etc_btc","dbc_neo","eth_btc",
  "zec_btc","qtum_btc","sc_btc","ven_btc",
  "ven_eth","sc_eth","zec_eth"
]`
	jsonTicker := `[{"symbol":"eth_btc","timestamp":"1410431279000","ticker":{"change":"4.21","high":"7722.58","latest":"7682.29","low":"7348.30","turnover":"0.00","vol":"1316.3235"}},{"symbol":"sc_btc","timestamp":"1410431279000","ticker":{"change":"4.21","high":"7722.58","latest":"7682.29","low":"7348.30","turnover":"0.00","vol":"1316.3235"}}]`
	fakeRoundTripper := &FakeRoundTripper{message: jsonSymbol, status: http.StatusOK}
	client := newTestLbankPublicClient(fakeRoundTripper)
	client.CurrencyPairs()
	fakeRoundTripper.message = jsonTicker
	volume, err := client.Volume("ETH", "BTC")
	if err != nil {
		panic(err)
	}
	if volume != 1316.3235 {
		t.Errorf("LbankPublicApi: Expected %v. Got %v", 1316.3235, volume)
	}
}
