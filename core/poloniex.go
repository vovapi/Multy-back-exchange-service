package core

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Multy-io/Multy-back-exchange-service/api"
	"github.com/Multy-io/Multy-back-exchange-service/currencies"
)

const (
	TICKER     = "1002" /* Ticker Channel Id */
	SUBSBUFFER = 24     /* Subscriptions Buffer */
)

type PoloniexTicker struct {
	Symbol string `json:"currencyPair"`
	Last   string `json:"last"`
}

func (b *PoloniexTicker) getCurriences() currencies.CurrencyPair {

	if len(b.Symbol) > 0 {
		var symbol = b.Symbol
		var currencyCodes = strings.Split(symbol, "_")
		if len(currencyCodes) > 1 {
			return currencies.CurrencyPair{currencies.NewCurrencyWithCode(currencyCodes[1]), currencies.NewCurrencyWithCode(currencyCodes[0])}
		}
	}
	return currencies.CurrencyPair{currencies.NotAplicable, currencies.NotAplicable}
}

type PoloniexManager struct {
	BasicManager
	poloniexApi    *api.PoloniexApi
	channelsByID   map[string]string
	channelsByName map[string]string
	marketChannels []string
	symbolsToParse map[string]bool
}

func (poloniexTicker PoloniexTicker) IsFilled() bool {
	return (len(poloniexTicker.Symbol) > 0 && len(poloniexTicker.Last) > 0)
}

func (b *PoloniexManager) StartListen(exchangeConfiguration ExchangeConfiguration, resultChan chan Result) {

	b.tickers = make(map[string]Ticker)
	b.poloniexApi = api.NewPoloniexApi()
	b.channelsByID = make(map[string]string)
	b.channelsByName = make(map[string]string)
	b.marketChannels = []string{}
	b.symbolsToParse = b.composeSybolsToParse(exchangeConfiguration)
	b.setchannelids()

	ch := make(chan api.Reposponse)

	go b.poloniexApi.StartListen(ch)
	go b.startSendingDataBack(exchangeConfiguration, resultChan)

	for {
		select {
		case response := <-ch:

			if *response.Err != nil {
				log.Errorf("StartListen:PoloniexManager: %v", &response.Err)
				//callback(TickerCollection{}, err)
			} else if *response.Message != nil {
				var unmarshaledMessage []interface{}

				err := json.Unmarshal(*response.Message, &unmarshaledMessage)
				if err != nil {
					log.Errorf("StartListen:PoloniexManager:json.Unmarshal %v", err.Error())
					//callback(TickerCollection{}, err)
				} else if len(unmarshaledMessage) > 2 {
					var poloniexTicker PoloniexTicker
					args := unmarshaledMessage[2].([]interface{})
					poloniexTicker, err = b.convertArgsToTicker(args)
					//fmt.Println(poloniexTicker.CurrencyPair)

					if err == nil && poloniexTicker.IsFilled() && b.symbolsToParse[poloniexTicker.Symbol] {

						var ticker Ticker
						ticker.Rate, _ = strconv.ParseFloat(poloniexTicker.Last, 64)
						ticker.TimpeStamp = time.Now()
						ticker.Pair = poloniexTicker.getCurriences()
						//fmt.Println(targetCurrency.CurrencyCode(), referenceCurrency.CurrencyCode())
						b.Lock()
						b.tickers[ticker.Pair.Symbol()] = ticker
						b.Unlock()
					}
				}
			} else {
				log.Errorf("Poloniex Error and Message are nil")
			}

		}
	}
}

func (b *PoloniexManager) startSendingDataBack(exchangeConfiguration ExchangeConfiguration, resultChan chan Result) {

	for range time.Tick(1 * time.Second) {
		func() {
			values := []Ticker{}
			b.Lock()
			tickers := map[string]Ticker{}
			for k, v := range b.tickers {
				tickers[k] = v
			}
			b.Unlock()

			for _, value := range tickers {
				if value.TimpeStamp.After(time.Now().Add(-maxTickerAge * time.Second)) {
					values = append(values, value)
				}
			}

			var tickerCollection = TickerCollection{}
			tickerCollection.TimpeStamp = time.Now()
			tickerCollection.Tickers = values
			//fmt.Println(tickerCollection)
			if len(tickerCollection.Tickers) > 0 {
				resultChan <- Result{exchangeConfiguration.Exchange.String(), &tickerCollection, nil}
			}
		}()
	}
}

func (b *PoloniexManager) convertArgsToTicker(args []interface{}) (wsticker PoloniexTicker, err error) {

	if len(b.channelsByID) < 1 {
		b.setchannelids()
	}

	wsticker.Symbol = b.channelsByID[strconv.FormatFloat(args[0].(float64), 'f', 0, 64)]
	wsticker.Last = args[1].(string)
	return
}

func (b *PoloniexManager) setchannelids() (err error) {

	resp, err := b.poloniexApi.PubReturnTickers()
	if err != nil {
		return err
	}

	for k, v := range resp {
		chid := strconv.Itoa(v.ID)
		b.channelsByName[k] = chid
		b.channelsByID[chid] = k
		b.marketChannels = append(b.marketChannels, chid)
	}

	b.channelsByName["TICKER"] = TICKER
	b.channelsByID[TICKER] = "TICKER"
	return
}

func (b *PoloniexManager) composeSybolsToParse(exchangeConfiguration ExchangeConfiguration) map[string]bool {
	var symbolsToParse = map[string]bool{}
	for _, targetCurrency := range exchangeConfiguration.TargetCurrencies {
		for _, referenceCurrency := range exchangeConfiguration.ReferenceCurrencies {

			if referenceCurrency == "USD" {
				referenceCurrency = "USDT"
			}

			symbol := referenceCurrency + "_" + targetCurrency
			symbolsToParse[symbol] = true
		}
	}
	return symbolsToParse

}
