# 规则模板语法

内置模板文件位于 [`template/`](../template/) 目录，可直接下载使用或在 Web 控制台上传自定义模板。

## Clash Meta / Stash（YAML）

在 `proxy-groups` 中支持以下扩展字段，生成时自动处理并从输出中删除。

### `filter` — 节点过滤

```yaml
proxy-groups:
  - name: 🇸🇬 新加坡
    type: url-test
    filter: "SG|新加坡|Singapore"    # 正则，仅匹配的节点进入该组
    proxies: ["__NODES__"]
    url: http://cp.cloudflare.com/generate_204
    interval: 300
```

### `exclude-filter` — 排除节点

```yaml
  - name: 🇸🇬 新加坡（精品线路）
    type: url-test
    filter: "SG|新加坡"
    exclude-filter: "IPLC|BGP|中转"  # 在 filter 结果中再排除
    proxies: ["__NODES__"]
```

### `dialer-proxy` — 链式代理（中转落地）

```yaml
  - name: 🇺🇸 美国 via 新加坡
    type: select
    filter: "US|美国"
    dialer-proxy: "SG|新加坡"        # 正则匹配第一个新加坡节点作为中转
    proxies: ["__NODES__"]
```

`url` 和 `benchmark-url` 可混写；生成时按目标客户端自动规范化（Clash Meta 输出 `url`，Stash 输出 `benchmark-url`）。

## Surge（INI）

```ini
[Proxy]
__NODES__

[Proxy Group]
🇸🇬 SG = url-test, __NODES__, policy-regex-filter=SG|新加坡, url=http://cp.cloudflare.com/generate_204, interval=300
🤖 AI = select, __NODES__, policy-regex-filter=US|美国, exclude-filter=IPLC|BGP, dialer-proxy=🇸🇬 SG

[Rule]
RULE-SET,https://ruleset.skk.moe/Clash/non_ip/ai.txt,🤖 AI
FINAL,🇸🇬 SG
```

使用 `policy-regex-filter=` 按正则筛选节点。生成后 `policy-regex-filter=`、`exclude-filter=`、`dialer-proxy=` 均不保留；`dialer-proxy` 翻译为节点行的 `underlying-proxy=` 参数。

> `dialer-proxy` 只作用到该组实际展开的节点，中转目标优先按组名匹配，找不到时再按节点名匹配。
