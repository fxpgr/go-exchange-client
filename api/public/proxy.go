package public

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/logger"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

func RandomProxyUrl(proxyUrlListGroup ProxyUrlList) func(*http.Request) (*url.URL, error) {
	proxyUrlList, _ := proxyUrlListGroup.GetList()
	rand.Seed(time.Now().UnixNano())

	return func(*http.Request) (*url.URL, error) {
		if len(proxyUrlList) == 0 {
			fmt.Println("there is no proxy")
			return nil, nil
		}
		i := rand.Intn(len(proxyUrlList))
		fmt.Println(proxyUrlList[i])
		return proxyUrlList[i], nil
	}
}

func NewProxyUrlList(baseUrl string) ProxyUrlList {
	pul := ProxyUrlList{BASE_URL: baseUrl, listLastUpdated: time.Now(), ListCacheDuration: time.Minute * 5}
	pul.urlList = make([]*url.URL, 0)
	pul.fetchList()
	return pul
}

type ProxyUrlList struct {
	BASE_URL string

	listLastUpdated   time.Time
	ListCacheDuration time.Duration
	urlList           []*url.URL
}

func (pul *ProxyUrlList) GetList() ([]*url.URL, error) {
	now := time.Now()
	if now.Sub(pul.listLastUpdated) >= pul.ListCacheDuration {
		logger.Get().Info("update proxies")
		err := pul.fetchList()
		if err != nil {
			return pul.urlList, nil
		}
		pul.listLastUpdated = time.Now()
	}
	return pul.urlList, nil
}

func (pul *ProxyUrlList) fetchList() error {
	res, err := http.Get(pul.BASE_URL)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	byteArray, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	urlList := make([]*url.URL, 0)
	for _, v := range regexp.MustCompile("\n").Split(string(byteArray), -1) {
		if v == "" {
			continue
		}
		url, err := url.Parse("http://" + v)
		if err != nil {
			continue
		}
		urlList = append(urlList, url)
	}
	pul.urlList = urlList
	return nil
}
