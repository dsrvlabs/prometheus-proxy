package converter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/dsrvlabs/prometheus-proxy/jsonselector"
	"github.com/dsrvlabs/prometheus-proxy/types"
)

// ResultConvert represents the result of a conversion.
type ResultConvert struct {
	Selector   string
	MetricName string
	Value      float64
	Err        error
}

// Converter is the interface for retrieving the contents of a URL.
type Converter interface {
	Fetch(config types.RPCFetchConfig) ([]ResultConvert, error)
}

type converter struct {
	selector jsonselector.Selector
}

func (c *converter) Fetch(config types.RPCFetchConfig) ([]ResultConvert, error) {
	var reqBodyReader io.Reader = nil
	if config.Body != "" {
		reqBodyReader = strings.NewReader(config.Body)
	}

	req, err := http.NewRequest(config.Method, config.URL, reqBodyReader)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: time.Second * 2}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	d, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	mapData := make(map[string]interface{})
	err = json.Unmarshal(d, &mapData)
	if err != nil {
		return nil, err
	}

	ret := make([]ResultConvert, len(config.Fields))
	for i, field := range config.Fields {
		value, err := c.selector.Find(mapData, field.Selector)
		convertedValue, err := convertToFloat(value)

		ret[i] = ResultConvert{
			Selector:   field.Selector,
			MetricName: field.MetricName,
			Value:      convertedValue,
			Err:        err,
		}
	}

	return ret, nil
}

func convertToFloat(value interface{}) (float64, error) {
	outValue := 0.0

	switch reflect.TypeOf(value).Kind() {
	case reflect.String:
		strValue := value.(string)
		if strings.HasPrefix(strValue, "0x") {
			outInt, err := strconv.ParseInt(strings.TrimLeft(strValue, "0x"), 16, 64)
			if err != nil {
				return 0.0, err
			}
			return float64(outInt), nil
		}

		outValue, err := strconv.ParseInt(strValue, 10, 64)
		if err == nil {
			return float64(outValue), nil
		}

		outValue, err = strconv.ParseInt(strValue, 16, 64)
		if err != nil {
			return 0.0, err
		}

		return float64(outValue), nil
	case reflect.Int:
		return float64(value.(int)), nil
	case reflect.Bool:
		if value.(bool) {
			outValue = 1.0
		}
	default:
		outValue = 0.0
	}

	return outValue, nil
}

// NewConverter creates a new Converter.
func NewConverter(selector jsonselector.Selector) Converter {
	return &converter{
		selector: selector,
	}
}
