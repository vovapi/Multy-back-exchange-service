package core

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/Multy-io/Multy-back-exchange-service/api"
	"github.com/Multy-io/Multy-back-exchange-service/currencies"
	"strconv"
)

type OkexManager struct {
	BasicManager
	okexApi *api.OkexApi
}

type OkexTicker struct {
	Binary int    `json:"binary"`
	Symbol string `json:"channel"`
	Data   struct {
		High       string `json:"high"`
		LimitLow   string `json:"limitLow"`
		Vol        string `json:"vol"`
		Last       string `json:"last"`
		Low        string `json:"low"`
		Buy        string `json:"buy"`
		HoldAmount string `json:"hold_amount"`
		Sell       string `json:"sell"`
		ContractID int64  `json:"contractId"`
		UnitAmount string `json:"unitAmount"`
		LimitHigh  string `json:"limitHigh"`
	} `json:"data"`
}

func (b *OkexTicker) getCurriences() currencies.CurrencyPair {

	if len(b.Symbol) > 0 {
		var symbol = b.Symbol
		var currencyCode = strings.TrimPrefix(strings.TrimSuffix(symbol, "_ticker_this_week"), "ok_sub_futureusd_")
		if len(currencyCode) > 2 {
			return currencies.CurrencyPair{currencies.NewCurrencyWithCode(currencyCode), currencies.Tether}
		}
	}
	return currencies.CurrencyPair{currencies.NotAplicable, currencies.NotAplicable}
}

func (ticker OkexTicker) IsFilled() bool {
	return (len(ticker.Symbol) > 0 && len(ticker.Data.Last) > 0)
}

func (b *OkexManager) StartListen(exchangeConfiguration ExchangeConfiguration, resultChan chan Result) {

	b.tickers = make(map[string]Ticker)
	b.okexApi = &api.OkexApi{}

	var apiCurrenciesConfiguration = api.ApiCurrenciesConfiguration{}
	apiCurrenciesConfiguration.TargetCurrencies = exchangeConfiguration.TargetCurrencies
	apiCurrenciesConfiguration.ReferenceCurrencies = exchangeConfiguration.ReferenceCurrencies

	ch := make(chan api.Reposponse)

	go b.okexApi.StartListen(apiCurrenciesConfiguration, ch)
	go b.startSendingDataBack(exchangeConfiguration, resultChan)

	for {
		select {
		case response := <-ch:

			if *response.Err != nil {
				log.Errorf("error:StartListen:OkexManager %v", response.Err)
				//callback(nil, error)
			} else if response.Message != nil {
				//fmt.Printf("%s \n", message)
				b.addMessage(*response.Message)
			}

		}
	}
}

func (b *OkexManager) startSendingDataBack(exchangeConfiguration ExchangeConfiguration, resultChan chan Result) {

	for range time.Tick(1 * time.Second) {
		func() {
			tickers := []Ticker{}

			b.Lock()
			tempTickers := map[string]Ticker{}
			for k, v := range b.tickers {
				tempTickers[k] = v
			}
			b.Unlock()

			for _, ticker := range tempTickers {
				if ticker.TimpeStamp.After(time.Now().Add(-maxTickerAge * time.Second)) {
					tickers = append(tickers, ticker)
				}
			}

			var tickerCollection = TickerCollection{}
			tickerCollection.TimpeStamp = time.Now()
			tickerCollection.Tickers = tickers
			if len(tickerCollection.Tickers) > 0 {
				resultChan <- Result{exchangeConfiguration.Exchange.String(), &tickerCollection, nil}
			}
		}()
	}
}

func (b *OkexManager) addMessage(message []byte) {

	var okexTickers = []OkexTicker{}
	json.Unmarshal(message, &okexTickers)

	for _, okexTicker := range okexTickers {
		if okexTicker.IsFilled() {
			var ticker = Ticker{}
			ticker.Rate, _ = strconv.ParseFloat(okexTicker.Data.Last, 64)

			ticker.Pair = okexTicker.getCurriences()
			ticker.TimpeStamp = time.Now()
			b.Lock()
			b.tickers[ticker.Pair.Symbol()] = ticker
			b.Unlock()
		}
	}

	//fmt.Println(b.tickers)

}
