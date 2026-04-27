package channelbalance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	lua "github.com/yuin/gopher-lua"
)

const (
	defaultLuaTimeout       = 10 * time.Second
	maxLuaScriptBytes       = 32 * 1024
	maxLuaResponseBodyBytes = 2 * 1024 * 1024
)

type Params struct {
	BaseURL         string
	APIKey          string
	Timeout         time.Duration
	HTTPClient      *http.Client
	AllowPrivateURL bool
}

type RequestConfig struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    string
}

type Result struct {
	Remaining float64  `json:"remaining"`
	Unit      string   `json:"unit"`
	Total     *float64 `json:"total,omitempty"`
	Used      *float64 `json:"used,omitempty"`
	PlanName  string   `json:"planName,omitempty"`
	Extra     string   `json:"extra,omitempty"`
}

func QueryChannelBalance(ctx context.Context, channel *model.Channel, script string) (Result, error) {
	if channel == nil {
		return Result{}, errors.New("channel cannot be nil")
	}
	client, err := service.NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return Result{}, err
	}
	baseURL := channel.GetBaseURL()
	return QueryWithScript(ctx, script, Params{
		BaseURL:         baseURL,
		APIKey:          channel.Key,
		HTTPClient:      client,
		AllowPrivateURL: isLoopbackBaseURL(baseURL),
	})
}

func QueryWithScript(ctx context.Context, script string, params Params) (Result, error) {
	script = strings.TrimSpace(script)
	if script == "" {
		return Result{}, errors.New("custom balance script is empty")
	}
	if len(script) > maxLuaScriptBytes {
		return Result{}, fmt.Errorf("custom balance script is too large: %d bytes, max %d bytes", len(script), maxLuaScriptBytes)
	}
	if params.Timeout <= 0 {
		params.Timeout = defaultLuaTimeout
	}
	if params.HTTPClient == nil {
		params.HTTPClient = http.DefaultClient
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, params.Timeout)
	defer cancel()

	L, err := newLuaState(ctx, params)
	if err != nil {
		return Result{}, err
	}
	defer L.Close()

	if err := L.DoString(script); err != nil {
		return Result{}, fmt.Errorf("execute custom balance script failed: %w", err)
	}

	request, err := readRequestConfig(L)
	if err != nil {
		return Result{}, err
	}
	if err := validateRequestConfig(request, params.AllowPrivateURL); err != nil {
		return Result{}, err
	}

	body, err := sendRequest(ctx, params.HTTPClient, request)
	if err != nil {
		return Result{}, err
	}

	var response any
	if err := common.Unmarshal(body, &response); err != nil {
		return Result{}, fmt.Errorf("parse balance response JSON failed: %w", err)
	}

	extractor := L.GetGlobal("extractor")
	if extractor == lua.LNil {
		return Result{}, errors.New("custom balance script must define extractor(response)")
	}
	extractorFn, ok := extractor.(*lua.LFunction)
	if !ok {
		return Result{}, errors.New("custom balance script extractor must be a function")
	}
	if err := L.CallByParam(lua.P{
		Fn:      extractorFn,
		NRet:    1,
		Protect: true,
	}, goValueToLua(L, response)); err != nil {
		return Result{}, fmt.Errorf("execute balance extractor failed: %w", err)
	}
	ret := L.Get(-1)
	L.Pop(1)

	return readResult(ret)
}

func newLuaState(ctx context.Context, params Params) (*lua.LState, error) {
	L := lua.NewState(lua.Options{
		SkipOpenLibs: true,
	})
	L.SetContext(ctx)

	for _, lib := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		if err := L.CallByParam(lua.P{
			Fn:      L.NewFunction(lib.fn),
			NRet:    0,
			Protect: true,
		}, lua.LString(lib.name)); err != nil {
			L.Close()
			return nil, fmt.Errorf("open lua library %s failed: %w", lib.name, err)
		}
	}

	for _, name := range []string{
		"dofile",
		"loadfile",
		"load",
		"loadstring",
		"require",
		"module",
		"collectgarbage",
		"print",
	} {
		L.SetGlobal(name, lua.LNil)
	}

	L.SetGlobal("base_url", lua.LString(strings.TrimRight(params.BaseURL, "/")))
	L.SetGlobal("apikey", lua.LString(params.APIKey))
	return L, nil
}

func readRequestConfig(L *lua.LState) (RequestConfig, error) {
	requestValue := L.GetGlobal("request")
	if requestValue == lua.LNil {
		return RequestConfig{}, errors.New("custom balance script must define request")
	}
	requestTable, ok := requestValue.(*lua.LTable)
	if !ok {
		return RequestConfig{}, errors.New("custom balance script request must be a table")
	}

	request := RequestConfig{
		Method:  "GET",
		Headers: map[string]string{},
	}
	var err error
	if request.URL, err = requiredLuaString(requestTable, "url"); err != nil {
		return RequestConfig{}, err
	}
	if methodValue := requestTable.RawGetString("method"); methodValue != lua.LNil {
		method, ok := methodValue.(lua.LString)
		if !ok {
			return RequestConfig{}, errors.New("request.method must be a string when provided")
		}
		if strings.TrimSpace(string(method)) != "" {
			request.Method = string(method)
		}
	}
	if request.Body = optionalLuaString(requestTable, "body"); request.Body == "" {
		if bodyValue := requestTable.RawGetString("body"); bodyValue != lua.LNil && bodyValue.Type() != lua.LTString {
			return RequestConfig{}, errors.New("request.body must be a string when provided")
		}
	}

	headersValue := requestTable.RawGetString("headers")
	if headersValue == lua.LNil {
		return request, nil
	}
	headersTable, ok := headersValue.(*lua.LTable)
	if !ok {
		return RequestConfig{}, errors.New("request.headers must be a table")
	}

	var headerErr error
	headersTable.ForEach(func(key lua.LValue, value lua.LValue) {
		if headerErr != nil {
			return
		}
		headerName, ok := key.(lua.LString)
		if !ok || strings.TrimSpace(string(headerName)) == "" {
			headerErr = errors.New("request.headers keys must be non-empty strings")
			return
		}
		headerValue, err := luaScalarToString(value)
		if err != nil {
			headerErr = fmt.Errorf("request.headers[%s]: %w", string(headerName), err)
			return
		}
		request.Headers[string(headerName)] = headerValue
	})
	if headerErr != nil {
		return RequestConfig{}, headerErr
	}
	return request, nil
}

func requiredLuaString(table *lua.LTable, field string) (string, error) {
	value := table.RawGetString(field)
	if value == lua.LNil {
		return "", fmt.Errorf("request.%s is required", field)
	}
	text, ok := value.(lua.LString)
	if !ok {
		return "", fmt.Errorf("request.%s must be a string", field)
	}
	result := strings.TrimSpace(string(text))
	if result == "" {
		return "", fmt.Errorf("request.%s cannot be empty", field)
	}
	return result, nil
}

func optionalLuaString(table *lua.LTable, field string) string {
	value := table.RawGetString(field)
	if text, ok := value.(lua.LString); ok {
		return string(text)
	}
	return ""
}

func luaScalarToString(value lua.LValue) (string, error) {
	switch typed := value.(type) {
	case lua.LString:
		return string(typed), nil
	case lua.LNumber:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64), nil
	case lua.LBool:
		if bool(typed) {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("value must be string, number or boolean, got %s", value.Type().String())
	}
}

func validateRequestConfig(request RequestConfig, allowPrivateURL bool) error {
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		return fmt.Errorf("unsupported custom balance request method: %s", request.Method)
	}

	parsedURL, err := url.Parse(request.URL)
	if err != nil {
		return fmt.Errorf("invalid custom balance request url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported custom balance request protocol: %s", parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return errors.New("custom balance request url must include host")
	}
	if allowPrivateURL {
		return nil
	}

	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(
		request.URL,
		fetchSetting.EnableSSRFProtection,
		fetchSetting.AllowPrivateIp,
		fetchSetting.DomainFilterMode,
		fetchSetting.IpFilterMode,
		fetchSetting.DomainList,
		fetchSetting.IpList,
		fetchSetting.AllowedPorts,
		fetchSetting.ApplyIPFilterForDomain,
	); err != nil {
		return fmt.Errorf("custom balance request url blocked: %w", err)
	}
	return nil
}

func isLoopbackBaseURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	hostname := parsedURL.Hostname()
	if strings.EqualFold(hostname, "localhost") {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}

func sendRequest(ctx context.Context, client *http.Client, config RequestConfig) ([]byte, error) {
	method := strings.ToUpper(strings.TrimSpace(config.Method))
	var requestBody io.Reader
	if config.Body != "" {
		requestBody = strings.NewReader(config.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, config.URL, requestBody)
	if err != nil {
		return nil, err
	}
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, maxLuaResponseBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(responseBody) > maxLuaResponseBodyBytes {
		return nil, fmt.Errorf("custom balance response body exceeds %d bytes", maxLuaResponseBodyBytes)
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		preview := string(responseBody)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("custom balance request failed with status %d: %s", res.StatusCode, preview)
	}
	return responseBody, nil
}

func goValueToLua(L *lua.LState, value any) lua.LValue {
	switch typed := value.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(typed)
	case string:
		return lua.LString(typed)
	case float64:
		return lua.LNumber(typed)
	case float32:
		return lua.LNumber(typed)
	case int:
		return lua.LNumber(typed)
	case int64:
		return lua.LNumber(typed)
	case map[string]any:
		table := L.NewTable()
		for key, item := range typed {
			table.RawSetString(key, goValueToLua(L, item))
		}
		return table
	case []any:
		table := L.NewTable()
		for index, item := range typed {
			table.RawSetInt(index+1, goValueToLua(L, item))
		}
		return table
	default:
		return lua.LString(fmt.Sprint(typed))
	}
}

func readResult(value lua.LValue) (Result, error) {
	table, ok := value.(*lua.LTable)
	if !ok {
		return Result{}, errors.New("extractor must return a table")
	}

	remainingValue := table.RawGetString("remaining")
	remaining, ok := remainingValue.(lua.LNumber)
	if !ok {
		return Result{}, errors.New("extractor result.remaining must be a number")
	}

	unit := "USD"
	if unitValue := table.RawGetString("unit"); unitValue != lua.LNil {
		unitText, ok := unitValue.(lua.LString)
		if !ok {
			return Result{}, errors.New("extractor result.unit must be a string")
		}
		if strings.TrimSpace(string(unitText)) != "" {
			unit = strings.TrimSpace(string(unitText))
		}
	}

	result := Result{
		Remaining: float64(remaining),
		Unit:      unit,
		Total:     optionalLuaNumber(table, "total"),
		Used:      optionalLuaNumber(table, "used"),
		PlanName:  optionalResultString(table, "planName"),
		Extra:     optionalResultString(table, "extra"),
	}
	return result, nil
}

func optionalLuaNumber(table *lua.LTable, field string) *float64 {
	value := table.RawGetString(field)
	if number, ok := value.(lua.LNumber); ok {
		result := float64(number)
		return &result
	}
	return nil
}

func optionalResultString(table *lua.LTable, field string) string {
	value := table.RawGetString(field)
	if text, ok := value.(lua.LString); ok {
		return string(text)
	}
	return ""
}
