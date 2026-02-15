# Stats Package - Session Cost Tracking

Session 成本追踪数据层，负责扫描、解析、聚合和缓存 Claude session 的 cost/token 统计数据。

## 数据流

```
~/.claude/projects/<encoded-path>/<session-id>.jsonl
  ↓ ScanSessions()
[]SessionRecord (with cost/tokens)
  ↓ Aggregate()
[]DailyStat (by date/project/model)
  ↓ GenerateSummary()
Summary (complete stats with breakdowns)
  ↓ SaveCache()
~/.codes/stats.json
```

## 核心类型

### SessionRecord
单个 session 的完整成本记录。

```go
type SessionRecord struct {
    SessionID         string
    Project           string        // codes 项目别名
    ProjectPath       string
    Model             string
    StartTime         time.Time
    EndTime           time.Time
    Duration          time.Duration
    InputTokens       int64
    OutputTokens      int64
    CacheCreateTokens int64
    CacheReadTokens   int64
    CostUSD           float64       // 自动计算
    Turns             int
}
```

### DailyStat
按天聚合的统计数据。

```go
type DailyStat struct {
    Date         string             // "2026-02-15"
    Sessions     int
    TotalCost    float64
    InputTokens  int64
    OutputTokens int64
    ByProject    map[string]float64 // project → cost
    ByModel      map[string]float64 // model → cost
}
```

### Summary
完整汇总，包含所有维度的统计。

```go
type Summary struct {
    TotalCost      float64
    TotalSessions  int
    InputTokens    int64
    OutputTokens   int64
    CacheCreate    int64
    CacheRead      int64
    TopProjects    []ProjectCost    // 按成本降序
    TopModels      []ModelCost      // 按成本降序
    DailyBreakdown []DailyStat
}
```

## 使用示例

### 基本统计

```go
package main

import (
    "fmt"
    "codes/internal/stats"
)

func main() {
    // 加载缓存（自动刷新如果超过 5 分钟）
    cache, err := stats.LoadCache()
    if err != nil {
        panic(err)
    }
    cache, err = stats.RefreshIfNeeded(cache)
    if err != nil {
        panic(err)
    }

    // 生成本周汇总
    from, to := stats.ThisWeekRange()
    summary := stats.GenerateSummary(cache.Sessions, from, to)

    // 打印统计
    fmt.Printf("本周总成本: $%.2f\n", summary.TotalCost)
    fmt.Printf("总 sessions: %d\n", summary.TotalSessions)
    fmt.Printf("Input tokens: %dM\n", summary.InputTokens/1_000_000)
    fmt.Printf("Output tokens: %dK\n", summary.OutputTokens/1_000)
    fmt.Printf("Cache create: %dK\n", summary.CacheCreate/1_000)
    fmt.Printf("Cache read: %dK\n", summary.CacheRead/1_000)

    // 按项目排序
    fmt.Println("\n按项目:")
    for _, p := range summary.TopProjects {
        fmt.Printf("  %s: $%.2f\n", p.Project, p.Cost)
    }

    // 按模型排序
    fmt.Println("\n按模型:")
    for _, m := range summary.TopModels {
        fmt.Printf("  %s: $%.2f\n", m.Model, m.Cost)
    }
}
```

### 自定义时间范围

```go
// 最近 7 天
from, to := stats.Last7DaysRange()
summary := stats.GenerateSummary(cache.Sessions, from, to)

// 本月
from, to := stats.ThisMonthRange()
summary := stats.GenerateSummary(cache.Sessions, from, to)

// 全部历史（传零值）
summary := stats.GenerateSummary(cache.Sessions, time.Time{}, time.Time{})

// 自定义范围
from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local)
to := time.Date(2026, 2, 15, 23, 59, 59, 0, time.Local)
summary := stats.GenerateSummary(cache.Sessions, from, to)
```

### 强制刷新

```go
// 强制全量扫描（忽略缓存时间）
cache, err := stats.ForceRefresh(cache)
if err != nil {
    panic(err)
}
```

### 手动聚合

```go
// 直接聚合 session records
dailyStats := stats.Aggregate(cache.Sessions, from, to)

// 计算总成本和 session 数
totalCost := stats.TotalCost(dailyStats)
totalSessions := stats.TotalSessions(dailyStats)

// 计算总 token 数
inputTokens, outputTokens := stats.TotalTokens(dailyStats)

// 按项目分解
projects := stats.ProjectBreakdown(dailyStats)
for _, p := range projects {
    fmt.Printf("%s: $%.2f\n", p.Project, p.Cost)
}

// 按模型分解
models := stats.ModelBreakdown(dailyStats)
for _, m := range models {
    fmt.Printf("%s: $%.2f\n", m.Model, m.Cost)
}
```

## 定价表

| 模型 | Input ($/1M tokens) | Output ($/1M tokens) |
|------|---------------------|----------------------|
| claude-opus-4 | $15.00 | $75.00 |
| claude-sonnet-4/4-5 | $3.00 | $15.00 |
| claude-haiku-3-5 | $0.80 | $4.00 |
| claude-haiku-3 | $0.25 | $1.25 |

**Cache 定价**:
- Cache write: 25% of input price
- Cache read: 10% of input price

**前缀匹配**: `claude-sonnet-4-5-20250929` 自动匹配 `claude-sonnet-4-5`

## 缓存机制

- **路径**: `~/.codes/stats.json`
- **刷新间隔**: 5 分钟
- **增量扫描**: 仅扫描修改过的 JSONL 文件（基于 ModTime）
- **原子写入**: temp file + rename 保证数据一致性

## 数据源

扫描 `~/.claude/projects/` 下的所有 JSONL 文件：

```
~/.claude/projects/
  -Users-ourines-Projects-codes/
    a1b2c3d4.jsonl
    e5f6g7h8.jsonl
  -home-user-my-project/
    i9j0k1l2.jsonl
```

每个 JSONL 文件包含 session 的消息记录：

```jsonl
{"type": "user", "timestamp": "2026-02-15T10:00:00Z", ...}
{"type": "assistant", "timestamp": "2026-02-15T10:00:05Z", "message": {"model": "claude-sonnet-4-5-20250929", "usage": {"input_tokens": 1234, "output_tokens": 567, ...}}}
{"type": "user", "timestamp": "2026-02-15T10:01:00Z", ...}
...
```

## 测试

```bash
# 运行所有测试
go test ./internal/stats -v

# 测试覆盖
go test ./internal/stats -cover
```

当前测试覆盖：
- ✅ 定价计算（所有模型 + cache）
- ✅ 项目路径解码
- ✅ 聚合和时间过滤
- ✅ Token 统计
- ✅ 项目/模型分解
- ✅ 完整汇总生成

## 注意事项

1. **项目别名解析**: 需要 `config.ListProjects()` 返回项目映射，否则使用路径 basename
2. **时间范围**: `from`/`to` 为零值时包含所有记录
3. **成本计算**: 自动在解析时计算，无需手动调用 `CalculateCost()`
4. **模型名称**: 支持版本号前缀匹配，如 `claude-sonnet-4-5-20250929` → `claude-sonnet-4-5`
5. **缓存失效**: `RefreshIfNeeded()` 自动判断，也可手动 `ForceRefresh()`

## API 参考

### 扫描和缓存
- `ScanSessions(opts ScanOptions) ([]SessionRecord, error)`
- `LoadCache() (*StatsCache, error)`
- `SaveCache(cache *StatsCache) error`
- `RefreshIfNeeded(cache *StatsCache) (*StatsCache, error)`
- `ForceRefresh(cache *StatsCache) (*StatsCache, error)`

### 聚合和统计
- `Aggregate(records []SessionRecord, from, to time.Time) []DailyStat`
- `GenerateSummary(records []SessionRecord, from, to time.Time) Summary`
- `TotalCost(stats []DailyStat) float64`
- `TotalSessions(stats []DailyStat) int`
- `TotalTokens(stats []DailyStat) (int64, int64)`
- `ProjectBreakdown(stats []DailyStat) []ProjectCost`
- `ModelBreakdown(stats []DailyStat) []ModelCost`

### 时间范围
- `ThisWeekRange() (time.Time, time.Time)`
- `ThisMonthRange() (time.Time, time.Time)`
- `Last7DaysRange() (time.Time, time.Time)`
- `Last30DaysRange() (time.Time, time.Time)`

### 定价
- `CalculateCost(model string, usage Usage) float64`
