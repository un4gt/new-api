# Channel Custom Balance Lua Scripts

管理员可以在渠道编辑页的“自定义余额查询 Lua 脚本”中配置自定义余额查询逻辑。配置后，手动或自动更新渠道余额时会优先执行该脚本；未配置脚本时仍走渠道原有的官方/默认余额查询接口。

## 1) 脚本结构

脚本需要定义两个全局对象：

- `request`：余额查询请求配置，由后端负责实际发送 HTTP 请求。
- `extractor(response)`：从后端返回的完整 JSON 响应中提取余额。

后端会自动注入两个变量：

- `base_url`：当前渠道的 Base URL，已去掉末尾 `/`。
- `apikey`：当前渠道的密钥。

最小示例：

```lua
request = {
  url = base_url .. "/user/info",
  method = "GET",
  headers = {
    Authorization = "Bearer " .. apikey
  }
}

function extractor(response)
  return {
    remaining = response.user.normal_balance / 1000000,
    unit = "USD"
  }
end
```

`request` 支持字段：

- `url`：必填，完整 `http` 或 `https` URL。
- `method`：可选，默认 `GET`，支持 `GET`、`POST`、`PUT`、`PATCH`。
- `headers`：可选，Lua table，键和值会转成 HTTP header。
- `body`：可选，字符串请求体。

`extractor` 返回字段：

- `remaining`：必填，数字，剩余额度。
- `unit`：可选，默认 `USD`。当前写入渠道余额时支持 `USD` 和 `CNY`；`CNY` 会按系统汇率换算成 USD。
- `total`：可选，数字，仅用于测试结果展示。
- `used`：可选，数字，仅用于测试结果展示。
- `planName`：可选，字符串。
- `extra`：可选，字符串。

## 2) 常见示例

### 2.1 通用第三方中转：余额在 `user.normal_balance`

适用于响应类似下面的第三方中转：

```json
{
  "user": {
    "normal_balance": 12500000
  }
}
```

脚本：

```lua
request = {
  url = base_url .. "/user/info",
  method = "GET",
  headers = {
    Authorization = "Bearer " .. apikey
  }
}

function extractor(response)
  return {
    remaining = response.user.normal_balance / 1000000,
    unit = "USD"
  }
end
```

### 2.2 OpenRouter Credits

OpenRouter 官方 credits 接口返回 `total_credits` 和 `total_usage`，剩余额度需要相减。

```lua
request = {
  url = "https://openrouter.ai/api/v1/credits",
  method = "GET",
  headers = {
    Authorization = "Bearer " .. apikey
  }
}

function extractor(response)
  local credits = response.data.total_credits or 0
  local usage = response.data.total_usage or 0

  return {
    remaining = credits - usage,
    total = credits,
    used = usage,
    unit = "USD",
    planName = "OpenRouter credits"
  }
end
```

### 2.3 SiliconFlow 余额

SiliconFlow 的 `totalBalance` 通常是字符串，需要用 `tonumber` 转成数字。

```lua
request = {
  url = "https://api.siliconflow.cn/v1/user/info",
  method = "GET",
  headers = {
    Authorization = "Bearer " .. apikey
  }
}

function extractor(response)
  if response.code ~= 20000 then
    error(response.message or "SiliconFlow balance query failed")
  end

  return {
    remaining = tonumber(response.data.totalBalance),
    unit = "USD",
    planName = response.data.name or "SiliconFlow"
  }
end
```

### 2.4 DeepSeek CNY 余额

DeepSeek 返回多个币种时，可以在 Lua 里筛选 `CNY`。返回 `unit = "CNY"` 后，后端会按系统汇率写入 USD 余额。

```lua
request = {
  url = "https://api.deepseek.com/user/balance",
  method = "GET",
  headers = {
    Authorization = "Bearer " .. apikey
  }
}

function extractor(response)
  for _, item in ipairs(response.balance_infos or {}) do
    if item.currency == "CNY" then
      return {
        remaining = tonumber(item.total_balance),
        unit = "CNY",
        planName = "DeepSeek CNY balance"
      }
    end
  end

  error("DeepSeek CNY balance not found")
end
```

### 2.5 Moonshot CNY 余额

Moonshot 官方接口返回人民币余额，同样可以返回 `CNY` 让后端换算。

```lua
request = {
  url = "https://api.moonshot.cn/v1/users/me/balance",
  method = "GET",
  headers = {
    Authorization = "Bearer " .. apikey
  }
}

function extractor(response)
  if response.status ~= true or response.code ~= 0 then
    error(response.scode or "Moonshot balance query failed")
  end

  return {
    remaining = response.data.available_balance,
    total = response.data.available_balance,
    unit = "CNY",
    planName = "Moonshot balance"
  }
end
```

### 2.6 POST 请求体示例

部分第三方中转会要求 POST 请求。`body` 必须是字符串。

```lua
request = {
  url = base_url .. "/api/balance/query",
  method = "POST",
  headers = {
    Authorization = "Bearer " .. apikey,
    ["Content-Type"] = "application/json"
  },
  body = '{"include_details":true}'
}

function extractor(response)
  return {
    remaining = response.data.remaining_usd,
    total = response.data.total_usd,
    used = response.data.used_usd,
    unit = "USD",
    planName = response.data.plan_name or "default"
  }
end
```

## 3) 调试建议

1. 在渠道编辑页先点击“填入模板”，再按上游响应结构修改 `url`、`headers` 和 `extractor`。
2. 保存渠道后点击“测试脚本”；测试接口只返回解析结果，不会写入渠道余额。
3. 测试成功后，再点击渠道列表里的余额更新按钮，此时才会写入 `channel.balance`。
4. 多密钥渠道沿用现有规则，暂不支持余额查询。

## 4) 安全边界

Lua 脚本不能直接访问文件系统、环境变量或网络库。后端只开放基础 Lua 能力，并由 Go 统一执行 HTTP 请求、代理配置、超时控制、响应大小限制和 URL 安全校验。
