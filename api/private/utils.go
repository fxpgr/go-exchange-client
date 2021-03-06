package private

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type ClientMode int

const (
	TEST ClientMode = iota
	PROJECT
)

func FloorFloat64ToStr(num float64, dig int) string {
	return fmt.Sprintf("%."+strconv.Itoa(dig)+"f", num)
}

func parseCurrencyPair(s string) (string, string, error) {
	xs := strings.Split(s, "_")

	if len(xs) != 2 {
		return "", "", errors.New("invalid ticker title")
	}
	return xs[0], xs[1], nil
}

type errorResponse struct {
	Error *string `json:"error"`
}

type openOrder struct {
	OrderNumber string `json:"orderNumber"`
	Type        string `json:"type"`
	Rate        string `json:"rate"`
	Amount      string `json:"amount"`
	Total       string `json:"total"`
}

type orderRespnose struct {
	OrderNumber string `json:"orderNumber,string"`
}

type transferResponse struct {
	Response string `json:"response"`
}
type cancelOrderResponse struct {
	Success int `json:"success"`
}

func computeHmac256(message, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	io.WriteString(h, message)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func GetMd5HashSign(query string) (string, error) {
	hasher := md5.New()
	hasher.Write([]byte(query))
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func GetParamHmacSHA256Base64Sign(secret, params string) (string, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write([]byte(params))
	if err != nil {
		return "", err
	}
	signByte := mac.Sum(nil)
	return base64.StdEncoding.EncodeToString(signByte), nil
}

func NewHttpRequest(client *http.Client, reqType string, reqUrl string, postData string, requstHeaders map[string]string) ([]byte, error) {
	req, _ := http.NewRequest(reqType, reqUrl, strings.NewReader(postData))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 5.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/31.0.1650.63 Safari/537.36")
	if requstHeaders != nil {
		for k, v := range requstHeaders {
			req.Header.Add(k, v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("HttpStatusCode:%d ,Desc:%s", resp.StatusCode, string(bodyData)))
	}
	return bodyData, nil
}
